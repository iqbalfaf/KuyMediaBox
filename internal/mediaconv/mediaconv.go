// Package mediaconv builds ffmpeg arguments for video and audio conversions.
package mediaconv

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/i18n"
)

// VideoOptions come from the Video page.
type VideoOptions struct {
	Format     string `json:"format"`     // mp4 mkv webm mov avi gif
	Codec      string `json:"codec"`      // h264 h265 vp9 av1 copy
	Quality    string `json:"quality"`    // hemat seimbang tinggi
	Manual     bool   `json:"manual"`     // use CRF/Preset below
	CRF        int    `json:"crf"`        // manual only
	Preset     string `json:"preset"`     // fast medium slow (manual only)
	Resolution string `json:"resolution"` // original 2160 1440 1080 720 480 custom
	Custom     int    `json:"custom"`     // short side in px when Resolution == custom

	TargetMB     float64 `json:"targetMB"`     // >0: aim for this file size (bitrate mode, two passes when possible)
	BitrateK     int     `json:"bitrateK"`     // >0: fixed average video bitrate in kbps (one pass)
	HW           string  `json:"hw"`           // "" (processor) | nvenc | qsv | amf
	TrimStart    string  `json:"trimStart"`    // "", seconds or [hh:]mm:ss[.ms]
	TrimEnd      string  `json:"trimEnd"`      //
	FPS          string  `json:"fps"`          // original 60 30 24 15 custom
	FPSCustom    float64 `json:"fpsCustom"`    //
	AudioMode    string  `json:"audioMode"`    // auto copy aac mp3 opus mute
	AudioBitrate int     `json:"audioBitrate"` // kbps when the audio is re-encoded
	Rotate       int     `json:"rotate"`       // clockwise: 0 90 180 270
	FlipH        bool    `json:"flipH"`
	FlipV        bool    `json:"flipV"`
	Subtitles    string  `json:"subtitles"` // none embed burn

	// SubFile is a subtitle file found next to the source (set by the app, not the UI).
	SubFile string `json:"-"`
}

// AudioOptions come from the Audio page and the "Ambil audio saja" mode.
type AudioOptions struct {
	Format       string `json:"format"`     // mp3 m4a flac wav ogg opus, or "original" (copy, tags only)
	Bitrate      int    `json:"bitrate"`    // kbps for lossy formats (constant bitrate)
	VBR          bool   `json:"vbr"`        // variable bitrate by quality level (MP3, OGG, Opus)
	VBRLevel     string `json:"vbrLevel"`   // best | high | medium | small
	Channels     string `json:"channels"`   // source stereo mono
	SampleRate   string `json:"sampleRate"` // source 44100 48000
	KeepMetadata bool   `json:"keepMetadata"`

	TrimStart     string  `json:"trimStart"`
	TrimEnd       string  `json:"trimEnd"`
	FadeIn        float64 `json:"fadeIn"`  // seconds
	FadeOut       float64 `json:"fadeOut"` // seconds
	NormVolume    bool    `json:"normalize"`
	Loudness      float64 `json:"loudness"` // LUFS target: -14, -16, -23
	RemoveSilence bool    `json:"removeSilence"`
	Speed         float64 `json:"speed"` // 0.5 … 2 (1 = normal)
	Pitch         float64 `json:"pitch"` // semitones, -12 … 12
}

// Tags are the song details written by the tag editor (empty fields are cleared).
type Tags struct {
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	AlbumArtist string `json:"albumArtist"`
	Year        string `json:"year"`
	Genre       string `json:"genre"`
	Track       string `json:"track"`
	Cover       string `json:"cover"`       // new cover picture (file path)
	RemoveCover bool   `json:"removeCover"` // drop the existing cover
}

// Plan is one conversion: ffmpeg runs (the last one writes the result) plus helper runs.
type Plan struct {
	Prep     [][]string // quick runs before (e.g. extract subtitles), no progress
	Passes   [][]string // e.g. two-pass encoding; progress is split evenly
	Duration float64    // seconds of output, for progress
	Dir      string     // working directory for ffmpeg ("" = inherit)
}

// VideoFormats lists the codecs each container accepts (first is the default).
var VideoFormats = map[string][]string{
	"mp4":  {"h264", "h265", "av1", "copy"},
	"mkv":  {"h264", "h265", "vp9", "av1", "copy"},
	"webm": {"vp9", "av1", "copy"},
	"mov":  {"h264", "h265", "copy"},
	"avi":  {"h264", "copy"},
	"gif":  {"gif"},
}

// AudioFormats lists supported audio outputs and whether they are lossy.
var AudioFormats = map[string]bool{"mp3": true, "m4a": true, "ogg": true, "opus": true, "flac": false, "wav": false}

// Encoder names per codec, in order of preference.
var codecEncoders = map[string][]string{
	"h264": {"libx264"},
	"h265": {"libx265"},
	"vp9":  {"libvpx-vp9"},
	"av1":  {"libsvtav1", "libaom-av1"},
}

// Hardware encoder names per vendor and codec.
var hwEncoders = map[string]map[string]string{
	"nvenc": {"h264": "h264_nvenc", "h265": "hevc_nvenc", "av1": "av1_nvenc"},
	"qsv":   {"h264": "h264_qsv", "h265": "hevc_qsv", "av1": "av1_qsv", "vp9": "vp9_qsv"},
	"amf":   {"h264": "h264_amf", "h265": "hevc_amf", "av1": "av1_amf"},
}

// HWEncoder returns the hardware encoder of a vendor for a codec ("" when there is none).
func HWEncoder(hw, codec string) string { return hwEncoders[hw][codec] }

// PickEncoder returns the ffmpeg encoder for a codec, or "" when none is available.
func PickEncoder(codec string, encoders map[string]bool) string {
	for _, e := range codecEncoders[codec] {
		if encoders == nil || encoders[e] {
			return e
		}
	}
	return ""
}

// VideoExt returns the output extension for video options.
func VideoExt(o VideoOptions) string { return o.Format }

// Normalize fixes combinations the UI should not produce but might (old saved settings).
func (o *VideoOptions) Normalize() {
	codecs, ok := VideoFormats[o.Format]
	if !ok {
		o.Format = "mp4"
		codecs = VideoFormats["mp4"]
	}
	valid := false
	for _, c := range codecs {
		if c == o.Codec {
			valid = true
		}
	}
	if !valid {
		o.Codec = codecs[0]
	}
	switch o.Quality {
	case "hemat", "seimbang", "tinggi":
	default:
		o.Quality = "seimbang"
	}
	switch o.Preset {
	case "fast", "medium", "slow":
	default:
		o.Preset = "medium"
	}
	if _, ok := hwEncoders[o.HW]; !ok || o.Codec == "copy" || o.Format == "gif" {
		o.HW = ""
	}
	switch o.FPS {
	case "60", "30", "25", "24", "15", "12", "10":
	case "custom":
		if o.FPSCustom <= 0 || o.FPSCustom > 240 {
			o.FPS = "original"
		}
	default:
		o.FPS = "original"
	}
	switch o.AudioMode {
	case "copy", "aac", "mp3", "opus", "mute":
	default:
		o.AudioMode = "auto"
	}
	// Codecs each container can hold.
	switch {
	case o.Format == "webm" && (o.AudioMode == "aac" || o.AudioMode == "mp3"):
		o.AudioMode = "opus"
	case o.Format == "mov" && o.AudioMode == "opus":
		o.AudioMode = "aac"
	case o.Format == "avi" && (o.AudioMode == "opus" || o.AudioMode == "aac"):
		o.AudioMode = "mp3"
	}
	switch o.AudioBitrate {
	case 64, 96, 128, 160, 192, 256, 320:
	default:
		o.AudioBitrate = 160
	}
	o.Rotate = ((o.Rotate % 360) + 360) % 360 / 90 * 90
	switch o.Subtitles {
	case "embed", "burn":
	default:
		o.Subtitles = "none"
	}
	if o.Format == "gif" || (o.Format == "avi" && o.Subtitles == "embed") {
		o.Subtitles = "none"
	}
	if o.TargetMB < 0 || o.Format == "gif" || o.Codec == "copy" {
		o.TargetMB = 0
	}
	if o.BitrateK < 0 || o.TargetMB > 0 || o.Format == "gif" || o.Codec == "copy" {
		o.BitrateK = 0
	}
	if o.BitrateK > 0 {
		o.BitrateK = max(50, min(o.BitrateK, 200000))
	}
}

// crfFor maps the friendly quality choice to a CRF per codec.
func crfFor(codec, quality string) int {
	table := map[string][3]int{ // hemat, seimbang, tinggi
		"h264": {28, 23, 18},
		"h265": {30, 26, 22},
		"vp9":  {38, 32, 26},
		"av1":  {40, 34, 27},
	}
	t, ok := table[codec]
	if !ok {
		t = table["h264"]
	}
	switch quality {
	case "hemat":
		return t[0]
	case "tinggi":
		return t[2]
	default:
		return t[1]
	}
}

// DescribeQuality returns e.g. "CRF 23 · kecepatan sedang" for the UI.
func DescribeQuality(o VideoOptions) string {
	o.Normalize()
	crf := crfFor(o.Codec, o.Quality)
	if o.Manual && o.CRF > 0 {
		crf = o.CRF
	}
	speed := map[string]string{"fast": "cepat", "medium": "sedang", "slow": "lambat"}[o.Preset]
	return fmt.Sprintf("CRF %d · kecepatan %s", crf, speed)
}

// ScaleFilter returns a scale filter so the short side becomes target (never upscaling).
func ScaleFilter(w, h, target int) string {
	if target <= 0 || w <= 0 || h <= 0 {
		return ""
	}
	short := w
	if h < short {
		short = h
	}
	if short <= target {
		return ""
	}
	if w >= h {
		return fmt.Sprintf("scale=-2:%d", target)
	}
	return fmt.Sprintf("scale=%d:-2", target)
}

func targetShortSide(o VideoOptions) int {
	switch o.Resolution {
	case "2160":
		return 2160
	case "1440":
		return 1440
	case "1080":
		return 1080
	case "720":
		return 720
	case "480":
		return 480
	case "custom":
		if o.Custom >= 16 {
			return o.Custom - o.Custom%2
		}
	}
	return 0
}

// rotated returns the size after the rotation option.
func rotated(w, h int, o VideoOptions) (int, int) {
	if o.Rotate == 90 || o.Rotate == 270 {
		return h, w
	}
	return w, h
}

// OutputSize returns the width/height after rotation and scaling (for the UI "Hasil" column).
func OutputSize(w, h int, o VideoOptions) (int, int) {
	if o.Codec == "copy" && o.Format != "gif" {
		return w, h
	}
	w, h = rotated(w, h, o)
	target := targetShortSide(o)
	if o.Format == "gif" && target == 0 {
		target = 480
	}
	if ScaleFilter(w, h, target) == "" {
		return w, h
	}
	// ffmpeg's "-2" rounds the scaled side to the nearest even number.
	if w >= h {
		return evenRound(float64(w) * float64(target) / float64(h)), target
	}
	return target, evenRound(float64(h) * float64(target) / float64(w))
}

// ParseTime reads "83", "83.5", "1:23", "01:02:03,5" as seconds. ok is false for bad input;
// an empty string is (0, true).
func ParseTime(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return 0, true
	}
	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, false
	}
	total := 0.0
	for i, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		if i > 0 && v >= 60 {
			return 0, false
		}
		total = total*60 + v
	}
	return total, true
}

// trimRange resolves trim options: start and length in seconds (length 0 = to the end).
func trimRange(startS, endS string, duration float64) (float64, float64, error) {
	start, ok1 := ParseTime(startS)
	end, ok2 := ParseTime(endS)
	if !ok1 || !ok2 {
		return 0, 0, errors.New(i18n.L("Format waktu potong tidak valid. Contoh: 1:30 atau 00:01:30", "Invalid trim time. Example: 1:30 or 00:01:30"))
	}
	if duration > 0 && start >= duration {
		return 0, 0, errors.New(i18n.L("Waktu mulai melewati durasi file", "The start time is past the end of the file"))
	}
	if end > 0 && end <= start {
		return 0, 0, errors.New(i18n.L("Waktu akhir harus setelah waktu mulai", "The end time must be after the start time"))
	}
	if end > 0 && duration > 0 && end >= duration {
		end = 0
	}
	length := 0.0
	if end > 0 {
		length = end - start
	}
	return start, length, nil
}

func secs(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }

// inputArgs is "-ss start -t length -i in": both are input options, so the cut is taken from
// the source timeline (speed changes and frame-rate filters can't shift it).
func inputArgs(in string, start, length float64) []string {
	var a []string
	if start > 0 {
		a = append(a, "-ss", secs(start))
	}
	if length > 0 {
		a = append(a, "-t", secs(length))
	}
	return append(a, "-i", in)
}

// outDuration is the length of the result.
func outDuration(total, start, length float64) float64 {
	if length > 0 {
		return length
	}
	if total > start {
		return total - start
	}
	return total
}

// transformFilters returns rotate/flip filters.
func transformFilters(o VideoOptions) []string {
	var f []string
	switch o.Rotate {
	case 90:
		f = append(f, "transpose=1")
	case 180:
		f = append(f, "hflip", "vflip")
	case 270:
		f = append(f, "transpose=2")
	}
	if o.FlipH {
		f = append(f, "hflip")
	}
	if o.FlipV {
		f = append(f, "vflip")
	}
	return f
}

func fpsValue(o VideoOptions) string {
	switch o.FPS {
	case "original", "":
		return ""
	case "custom":
		return strconv.FormatFloat(o.FPSCustom, 'f', -1, 64)
	}
	return o.FPS
}

// VideoArgs builds single-pass ffmpeg arguments (without global flags) to convert in → out.
// Two-pass target-size encodes and subtitle burning need VideoPlan.
func VideoArgs(in, out string, info ffmpeg.Info, o VideoOptions, encoders map[string]bool) ([]string, error) {
	o.TargetMB = 0
	if o.Subtitles == "burn" {
		o.Subtitles = "none"
	}
	p, err := VideoPlan(in, out, info, o, encoders, "")
	if err != nil {
		return nil, err
	}
	return p.Passes[len(p.Passes)-1], nil
}

var textSubs = map[string]bool{"subrip": true, "srt": true, "ass": true, "ssa": true, "webvtt": true, "mov_text": true, "text": true}

// VideoPlan builds the ffmpeg runs to convert in → out. work is a scratch folder (pass logs,
// subtitle copies); it may be "" when neither target size nor burned subtitles are used.
func VideoPlan(in, out string, info ffmpeg.Info, o VideoOptions, encoders map[string]bool, work string) (Plan, error) {
	o.Normalize()
	if !info.HasVideo {
		return Plan{}, errors.New(i18n.L("file ini tidak punya video", "this file has no video"))
	}
	start, length, err := trimRange(o.TrimStart, o.TrimEnd, info.Duration)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Duration: outDuration(info.Duration, start, length)}
	args := inputArgs(in, start, length)

	if o.Format == "gif" {
		w, h := rotated(info.Width, info.Height, o)
		target := targetShortSide(o)
		if target == 0 {
			target = 480
		}
		chain := transformFilters(o)
		fps := fpsValue(o)
		if fps == "" {
			fps = "12"
		}
		chain = append(chain, "fps="+fps)
		if scale := ScaleFilter(w, h, target); scale != "" {
			chain = append(chain, scale+":flags=lanczos")
		}
		vf := strings.Join(chain, ",") + ",split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=5"
		args = append(args, "-map", "0:v:0", "-vf", vf, "-loop", "0", "-f", "gif", out)
		plan.Passes = [][]string{args}
		return plan, nil
	}

	// Subtitles: a file next to the video wins over a subtitle track inside it.
	subIn := ""      // extra input for embedding
	burnFile := ""   // file name (relative to work) for the subtitles filter
	subCodecIn := "" // codec of the internal subtitle track
	hasSub := o.SubFile != "" || info.SubCodec != ""
	if o.Subtitles != "none" && hasSub {
		if o.SubFile == "" {
			subCodecIn = info.SubCodec
		}
		textual := o.SubFile != "" || textSubs[subCodecIn]
		switch o.Subtitles {
		case "embed":
			if !textual && o.Format != "mkv" {
				return Plan{}, errors.New(i18n.L("Subtitle bergambar (PGS/DVD) hanya bisa disematkan ke MKV", "Picture subtitles (PGS/DVD) can only be embedded in MKV"))
			}
			if o.SubFile != "" {
				subIn = o.SubFile
			}
		case "burn":
			if !textual {
				return Plan{}, errors.New(i18n.L("Subtitle bergambar (PGS/DVD) tidak bisa dibakar ke video", "Picture subtitles (PGS/DVD) can't be burned into the video"))
			}
			if o.Codec == "copy" {
				return Plan{}, errors.New(i18n.L("Membakar subtitle butuh encode ulang. Pilih codec selain \"salin\".", "Burning subtitles needs re-encoding. Pick a codec other than \"copy\"."))
			}
			if work == "" {
				return Plan{}, errors.New("burn needs a work folder")
			}
			if o.SubFile != "" {
				name, charenc, err := copySubtitle(o.SubFile, work)
				if err != nil {
					return Plan{}, err
				}
				burnFile = name
				if charenc != "" {
					burnFile += ":charenc=" + charenc
				}
			} else {
				burnFile = "sub.ass"
				plan.Prep = append(plan.Prep, []string{"-i", in, "-map", "0:s:0", filepath.Join(work, "sub.ass")})
			}
			plan.Dir = work
		}
	}
	if subIn != "" {
		if start > 0 {
			args = append(args, "-ss", secs(start))
		}
		args = append(args, "-i", subIn)
	}

	mapSubs := func() {
		if o.Subtitles != "embed" || !hasSub {
			args = append(args, "-sn")
			return
		}
		if subIn != "" {
			args = append(args, "-map", "1:0")
		} else {
			args = append(args, "-map", "0:s:0?")
		}
		switch o.Format {
		case "mp4", "mov":
			args = append(args, "-c:s", "mov_text")
		case "webm":
			args = append(args, "-c:s", "webvtt")
		default: // mkv
			low := strings.ToLower(subIn)
			switch {
			case subIn == "":
				args = append(args, "-c:s", "copy")
			case strings.HasSuffix(low, ".ass") || strings.HasSuffix(low, ".ssa"):
				args = append(args, "-c:s", "ass")
			default:
				args = append(args, "-c:s", "srt")
			}
		}
	}

	if o.Codec == "copy" {
		if !CanCopyVideo(info.VideoCodec, o.Format) {
			return Plan{}, fmt.Errorf(i18n.L("Video %s tidak bisa disalin ke %s tanpa encode ulang. Pilih codec lain (misalnya %s).", "%s video can't be copied into %s without re-encoding. Choose another codec (e.g. %s)."),
				codecName(info.VideoCodec), strings.ToUpper(o.Format), strings.ToUpper(VideoFormats[o.Format][0]))
		}
		args = append(args, "-map", "0:v:0")
		withAudio := info.HasAudio && o.AudioMode != "mute"
		if withAudio {
			args = append(args, "-map", "0:a:0?")
		}
		args = append(args, "-c:v", "copy", "-dn", "-map_metadata", "0")
		if withAudio {
			args = append(args, videoAudio(o, info)...)
		} else {
			args = append(args, "-an")
		}
		mapSubs()
		if o.Format == "mp4" || o.Format == "mov" {
			args = append(args, "-movflags", "+faststart")
		}
		plan.Passes = [][]string{append(args, out)}
		return plan, nil
	}

	enc := PickEncoder(o.Codec, encoders)
	if o.HW != "" {
		enc = HWEncoder(o.HW, o.Codec)
		if enc == "" || (encoders != nil && !encoders[enc]) {
			return Plan{}, fmt.Errorf(i18n.L("Akselerasi GPU %s tidak mendukung %s di PC ini", "GPU acceleration %s doesn't support %s on this PC"), strings.ToUpper(o.HW), codecName(o.Codec))
		}
	}
	if enc == "" {
		return Plan{}, fmt.Errorf(i18n.L("encoder %s tidak tersedia di FFmpeg ini", "the %s encoder isn't available in this FFmpeg"), strings.ToUpper(o.Codec))
	}

	// Video filters.
	w, h := rotated(info.Width, info.Height, o)
	chain := transformFilters(o)
	if scale := ScaleFilter(w, h, targetShortSide(o)); scale != "" {
		chain = append(chain, scale)
	}
	if fps := fpsValue(o); fps != "" {
		chain = append(chain, "fps="+fps)
	}
	if burnFile != "" {
		if start > 0 { // subtitles are timed against the original timeline
			chain = append(chain, "setpts=PTS+"+secs(start)+"/TB", "subtitles="+burnFile, "setpts=PTS-STARTPTS")
		} else {
			chain = append(chain, "subtitles="+burnFile)
		}
	}

	withAudio := info.HasAudio && o.AudioMode != "mute"
	kbps := 0
	if o.TargetMB > 0 {
		if plan.Duration <= 0 {
			return Plan{}, errors.New(i18n.L("Durasi video tidak diketahui, ukuran target tidak bisa dihitung", "Unknown video duration, the target size can't be worked out"))
		}
		audioK := 0
		if withAudio {
			audioK = o.AudioBitrate
			if o.AudioMode == "auto" || o.AudioMode == "copy" {
				audioK = 160
			}
		}
		totalK := o.TargetMB * 8 * 1000 * 0.96 / plan.Duration // MB → kbit, 4 % for the container
		kbps = int(totalK) - audioK
		if kbps < 60 {
			need := float64(60+audioK) * plan.Duration / 8 / 1000 / 0.96
			return Plan{}, fmt.Errorf(i18n.L("Ukuran target terlalu kecil untuk video sepanjang ini (minimal ±%.1f MB). Potong videonya atau naikkan ukuran target.", "The target size is too small for a video this long (at least ±%.1f MB). Trim the video or raise the target."), need)
		}
	} else if o.BitrateK > 0 {
		kbps = o.BitrateK
	}

	common := func(pass int) []string {
		a := append([]string{}, args...)
		a = append(a, "-map", "0:v:0")
		if withAudio && pass != 1 {
			a = append(a, "-map", "0:a:0?")
		}
		a = append(a, "-dn", "-map_metadata", "0", "-c:v", enc)
		a = append(a, encoderArgs(enc, o, kbps, pass)...)
		if len(chain) > 0 {
			a = append(a, "-vf", strings.Join(chain, ","))
		}
		return a
	}
	// Two passes only for a target size: a chosen bitrate is a one-pass average.
	twoPass := o.TargetMB > 0 && kbps > 0 && work != "" && (enc == "libx264" || enc == "libx265" || enc == "libvpx-vp9" || enc == "libaom-av1")
	if twoPass {
		p1 := common(1)
		p1 = append(p1, "-an", "-sn", "-f", "null", "-")
		plan.Passes = append(plan.Passes, p1)
		plan.Dir = work
	}
	pass := 0
	if twoPass {
		pass = 2
	}
	final := common(pass)
	if withAudio {
		final = append(final, videoAudio(o, info)...)
	} else {
		final = append(final, "-an")
	}
	args = final
	mapSubs()
	if o.Format == "mp4" || o.Format == "mov" {
		args = append(args, "-movflags", "+faststart")
	}
	plan.Passes = append(plan.Passes, append(args, out))
	return plan, nil
}

// encoderArgs returns quality/speed settings; kbps > 0 switches to bitrate mode and pass
// (1 or 2) to two-pass encoding.
func encoderArgs(enc string, o VideoOptions, kbps, pass int) []string {
	crf := crfFor(o.Codec, o.Quality)
	if o.Manual && o.CRF > 0 && o.CRF <= 63 {
		crf = o.CRF
	}
	br := strconv.Itoa(kbps) + "k"
	rate := func() []string {
		return []string{"-b:v", br, "-maxrate", strconv.Itoa(kbps*3/2) + "k", "-bufsize", strconv.Itoa(kbps*2) + "k"}
	}
	var a []string
	switch enc {
	case "libx264":
		a = []string{"-preset", o.Preset, "-pix_fmt", "yuv420p"}
		if kbps > 0 {
			a = append(a, "-b:v", br)
			if pass > 0 {
				a = append(a, "-pass", strconv.Itoa(pass), "-passlogfile", "kmbpass")
			} else {
				a = append(a, rate()[2:]...)
			}
		} else {
			a = append(a, "-crf", strconv.Itoa(crf))
		}
	case "libx265":
		params := "log-level=error"
		a = []string{"-preset", o.Preset, "-pix_fmt", "yuv420p"}
		if kbps > 0 {
			a = append(a, "-b:v", br)
			if pass > 0 {
				params += ":pass=" + strconv.Itoa(pass) + ":stats=kmbpass.log"
			} else {
				a = append(a, rate()[2:]...)
			}
		} else {
			a = append(a, "-crf", strconv.Itoa(crf))
		}
		a = append(a, "-x265-params", params)
		if o.Format == "mp4" || o.Format == "mov" {
			a = append(a, "-tag:v", "hvc1")
		}
	case "libvpx-vp9":
		cpu := map[string]string{"fast": "5", "medium": "3", "slow": "1"}[o.Preset]
		if pass == 1 {
			cpu = "4"
		}
		a = []string{"-row-mt", "1", "-deadline", "good", "-cpu-used", cpu, "-pix_fmt", "yuv420p"}
		if kbps > 0 {
			a = append(a, "-b:v", br)
			if pass > 0 {
				a = append(a, "-pass", strconv.Itoa(pass), "-passlogfile", "kmbpass")
			}
		} else {
			a = append(a, "-crf", strconv.Itoa(crf), "-b:v", "0")
		}
	case "libsvtav1":
		p := map[string]string{"fast": "10", "medium": "8", "slow": "6"}[o.Preset]
		a = []string{"-preset", p, "-pix_fmt", "yuv420p"}
		if kbps > 0 {
			a = append(a, "-b:v", br)
		} else {
			a = append(a, "-crf", strconv.Itoa(crf))
		}
	case "libaom-av1":
		cpu := map[string]string{"fast": "8", "medium": "6", "slow": "4"}[o.Preset]
		a = []string{"-cpu-used", cpu, "-row-mt", "1", "-pix_fmt", "yuv420p"}
		if kbps > 0 {
			a = append(a, "-b:v", br)
			if pass > 0 {
				a = append(a, "-pass", strconv.Itoa(pass), "-passlogfile", "kmbpass")
			}
		} else {
			a = append(a, "-crf", strconv.Itoa(crf), "-b:v", "0")
		}
	default:
		// Hardware encoders use a constant-quality scale close to x264's CRF.
		q := crfFor("h264", o.Quality)
		if o.Manual && o.CRF > 0 {
			q = min(o.CRF, 51)
		}
		qs := strconv.Itoa(q)
		switch {
		case strings.HasSuffix(enc, "_nvenc"):
			a = []string{"-preset", map[string]string{"fast": "p3", "medium": "p5", "slow": "p7"}[o.Preset], "-pix_fmt", "yuv420p"}
			if kbps > 0 {
				a = append(a, "-rc", "vbr")
				a = append(a, rate()...)
			} else {
				a = append(a, "-rc", "vbr", "-cq", qs, "-b:v", "0")
			}
		case strings.HasSuffix(enc, "_qsv"):
			a = []string{"-preset", map[string]string{"fast": "veryfast", "medium": "medium", "slow": "veryslow"}[o.Preset], "-pix_fmt", "nv12"}
			if kbps > 0 {
				a = append(a, rate()...)
			} else {
				a = append(a, "-global_quality", qs)
			}
		case strings.HasSuffix(enc, "_amf"):
			a = []string{"-quality", map[string]string{"fast": "speed", "medium": "balanced", "slow": "quality"}[o.Preset], "-pix_fmt", "nv12"}
			if kbps > 0 {
				a = append(a, "-rc", "vbr_peak")
				a = append(a, rate()...)
			} else {
				a = append(a, "-rc", "cqp", "-qp_i", qs, "-qp_p", qs)
				if !strings.HasPrefix(enc, "av1") {
					a = append(a, "-qp_b", qs)
				}
			}
		}
		if strings.HasPrefix(enc, "hevc_") && (o.Format == "mp4" || o.Format == "mov") {
			a = append(a, "-tag:v", "hvc1")
		}
	}
	return a
}

// videoAudio returns the audio codec arguments of a video result.
func videoAudio(o VideoOptions, info ffmpeg.Info) []string {
	br := strconv.Itoa(o.AudioBitrate) + "k"
	switch o.AudioMode {
	case "copy", "auto":
		// "auto" copies only when the video is copied too (re-encodes are meant to shrink).
		if (o.AudioMode == "copy" || o.Codec == "copy") && canCopyAudio(info.AudioCodec, o.Format) {
			return []string{"-c:a", "copy"}
		}
	case "aac":
		return []string{"-c:a", "aac", "-b:a", br}
	case "mp3":
		return []string{"-c:a", "libmp3lame", "-b:a", br}
	case "opus":
		return []string{"-c:a", "libopus", "-b:a", br}
	}
	return containerAudio(o.Format)
}

func containerAudio(format string) []string {
	switch format {
	case "webm":
		return []string{"-c:a", "libopus", "-b:a", "128k"}
	case "avi":
		return []string{"-c:a", "libmp3lame", "-b:a", "192k"}
	default:
		return []string{"-c:a", "aac", "-b:a", "160k"}
	}
}

// FramesPlan saves a picture every `every` seconds into pattern (e.g. …\name_%04d.jpg).
func FramesPlan(in, pattern string, info ffmpeg.Info, o VideoOptions, every float64, format string) (Plan, error) {
	o.Normalize()
	if !info.HasVideo {
		return Plan{}, errors.New(i18n.L("file ini tidak punya video", "this file has no video"))
	}
	if every <= 0 {
		every = 1
	}
	start, length, err := trimRange(o.TrimStart, o.TrimEnd, info.Duration)
	if err != nil {
		return Plan{}, err
	}
	w, h := rotated(info.Width, info.Height, o)
	chain := transformFilters(o)
	if scale := ScaleFilter(w, h, targetShortSide(o)); scale != "" {
		chain = append(chain, scale)
	}
	chain = append(chain, "fps=1/"+strconv.FormatFloat(every, 'f', -1, 64))
	args := inputArgs(in, start, length)
	args = append(args, "-map", "0:v:0", "-vf", strings.Join(chain, ","), "-an", "-sn")
	if format == "png" {
		args = append(args, "-c:v", "png")
	} else {
		args = append(args, "-c:v", "mjpeg", "-q:v", "2", "-pix_fmt", "yuvj420p")
	}
	args = append(args, "-f", "image2", pattern)
	return Plan{Passes: [][]string{args}, Duration: outDuration(info.Duration, start, length)}, nil
}

// MergeVideoPlan joins several videos into one. Every clip is scaled and padded to the size
// of the first one (after rotation and the resolution choice) and to one frame rate.
func MergeVideoPlan(ins []string, infos []ffmpeg.Info, out string, o VideoOptions, encoders map[string]bool) (Plan, error) {
	o.Normalize()
	if len(ins) < 2 {
		return Plan{}, errors.New(i18n.L("Pilih minimal 2 video untuk digabung", "Pick at least 2 videos to join"))
	}
	if o.Format == "gif" || o.Codec == "copy" {
		return Plan{}, errors.New(i18n.L("Gabung video butuh encode ulang: pilih format selain GIF dan codec selain \"salin\"", "Joining videos needs re-encoding: pick a format other than GIF and a codec other than \"copy\""))
	}
	enc := PickEncoder(o.Codec, encoders)
	if o.HW != "" {
		if e := HWEncoder(o.HW, o.Codec); e != "" && (encoders == nil || encoders[e]) {
			enc = e
		}
	}
	if enc == "" {
		return Plan{}, fmt.Errorf(i18n.L("encoder %s tidak tersedia di FFmpeg ini", "the %s encoder isn't available in this FFmpeg"), strings.ToUpper(o.Codec))
	}
	W, H := OutputSize(infos[0].Width, infos[0].Height, o)
	W, H = W-W%2, H-H%2
	if W <= 0 || H <= 0 {
		return Plan{}, errors.New(i18n.L("ukuran video pertama tidak diketahui", "the size of the first video is unknown"))
	}
	fps := fpsValue(o)
	if fps == "" {
		f := math.Round(infos[0].FPS)
		if f < 1 || f > 120 {
			f = 30
		}
		fps = strconv.Itoa(int(f))
	}
	withAudio := o.AudioMode != "mute"
	var args []string
	total := 0.0
	for _, in := range ins {
		args = append(args, "-i", in)
	}
	var graph []string
	var concatIn strings.Builder
	for i, info := range infos {
		if !info.HasVideo {
			return Plan{}, fmt.Errorf(i18n.L("file ke-%d tidak punya video", "file %d has no video"), i+1)
		}
		total += info.Duration
		chain := append(transformFilters(o),
			fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", W, H),
			fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2", W, H), "setsar=1", "fps="+fps, "format=yuv420p")
		graph = append(graph, fmt.Sprintf("[%d:v:0]%s[v%d]", i, strings.Join(chain, ","), i))
		fmt.Fprintf(&concatIn, "[v%d]", i)
		if withAudio {
			if info.HasAudio {
				graph = append(graph, fmt.Sprintf("[%d:a:0]aresample=48000,aformat=sample_fmts=fltp:channel_layouts=stereo[a%d]", i, i))
			} else {
				graph = append(graph, fmt.Sprintf("anullsrc=r=48000:cl=stereo,atrim=duration=%s[a%d]", secs(math.Max(0.1, info.Duration)), i))
			}
			fmt.Fprintf(&concatIn, "[a%d]", i)
		}
	}
	a := 0
	if withAudio {
		a = 1
	}
	graph = append(graph, fmt.Sprintf("%sconcat=n=%d:v=1:a=%d[vout]%s", concatIn.String(), len(ins), a, map[bool]string{true: "[aout]", false: ""}[withAudio]))
	args = append(args, "-filter_complex", strings.Join(graph, ";"), "-map", "[vout]")
	if withAudio {
		args = append(args, "-map", "[aout]")
	}
	kbps := 0
	if o.TargetMB > 0 && total > 0 {
		kbps = int(o.TargetMB*8*1000*0.96/total) - 160
		if kbps < 60 {
			return Plan{}, errors.New(i18n.L("Ukuran target terlalu kecil untuk video sepanjang ini", "The target size is too small for videos this long"))
		}
	} else if o.BitrateK > 0 {
		kbps = o.BitrateK
	}
	args = append(args, "-c:v", enc)
	args = append(args, encoderArgs(enc, o, kbps, 0)...)
	if withAudio {
		ao := o
		if ao.AudioMode == "copy" || ao.AudioMode == "auto" {
			ao.AudioMode = "auto"
		}
		args = append(args, videoAudio(ao, ffmpeg.Info{})...)
	}
	args = append(args, "-map_metadata", "-1", "-sn", "-dn")
	if o.Format == "mp4" || o.Format == "mov" {
		args = append(args, "-movflags", "+faststart")
	}
	return Plan{Passes: [][]string{append(args, out)}, Duration: total}, nil
}

var copyVideo = map[string]map[string]bool{
	"mp4":  {"h264": true, "hevc": true, "av1": true, "vp9": true, "mpeg4": true},
	"mov":  {"h264": true, "hevc": true, "mpeg4": true, "prores": true, "mjpeg": true},
	"webm": {"vp8": true, "vp9": true, "av1": true},
	"avi":  {"h264": true, "mpeg4": true, "mjpeg": true, "msmpeg4v3": true},
}

var copyAudio = map[string]map[string]bool{
	"mp4":  {"aac": true, "mp3": true, "ac3": true, "eac3": true, "opus": true, "alac": true},
	"mov":  {"aac": true, "mp3": true, "ac3": true, "alac": true, "pcm_s16le": true, "pcm_s24le": true},
	"webm": {"opus": true, "vorbis": true},
	"avi":  {"mp3": true, "ac3": true, "pcm_s16le": true},
}

// CanCopyVideo reports whether a source video codec can be stream-copied into a container.
func CanCopyVideo(codec, format string) bool {
	if format == "mkv" {
		return codec != ""
	}
	return copyVideo[format][codec]
}

func canCopyAudio(codec, format string) bool {
	if format == "mkv" {
		return codec != ""
	}
	return copyAudio[format][codec]
}

func codecName(c string) string {
	names := map[string]string{"h264": "H.264", "hevc": "H.265", "h265": "H.265", "vp9": "VP9", "vp8": "VP8", "av1": "AV1", "mpeg4": "MPEG-4", "prores": "ProRes"}
	if n, ok := names[c]; ok {
		return n
	}
	if c == "" {
		return i18n.L("sumber", "source")
	}
	return strings.ToUpper(c)
}

func evenRound(v float64) int { return int(math.Round(v/2)) * 2 }

// copySubtitle copies a subtitle file into work under a plain name (no characters the
// filter syntax would need escaped). charenc is set for files that are not UTF-8.
func copySubtitle(src, work string) (name, charenc string, err error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", "", fmt.Errorf(i18n.L("file subtitle tidak bisa dibaca: %w", "the subtitle file can't be read: %w"), err)
	}
	name = "sub" + strings.ToLower(filepath.Ext(src))
	if err := os.WriteFile(filepath.Join(work, name), data, 0o644); err != nil {
		return "", "", err
	}
	body := data
	if len(body) >= 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
		body = body[3:]
	}
	if !utf8.Valid(body) && !(len(body) >= 2 && (body[0] == 0xFF || body[0] == 0xFE)) {
		charenc = "CP1252"
	}
	return name, charenc, nil
}

// FindSubtitle returns a subtitle file next to a video: "movie.srt", "movie.id.srt", "movie.en.ass", …
func FindSubtitle(video string) string {
	dir := filepath.Dir(video)
	base := strings.TrimSuffix(filepath.Base(video), filepath.Ext(video))
	for _, ext := range []string{".srt", ".ass", ".ssa", ".vtt"} {
		p := filepath.Join(dir, base+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	prefix := strings.ToLower(base) + "."
	for _, e := range entries {
		n := strings.ToLower(e.Name())
		if e.IsDir() || !strings.HasPrefix(n, prefix) {
			continue
		}
		switch filepath.Ext(n) {
		case ".srt", ".ass", ".ssa", ".vtt":
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}
