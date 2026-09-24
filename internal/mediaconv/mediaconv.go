// Package mediaconv builds ffmpeg arguments for video and audio conversions.
package mediaconv

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"kuymediabox/internal/ffmpeg"
)

// VideoOptions come from the Video page.
type VideoOptions struct {
	Format     string `json:"format"`     // mp4 mkv webm mov avi gif
	Codec      string `json:"codec"`      // h264 h265 vp9 av1 copy
	Quality    string `json:"quality"`    // hemat seimbang tinggi
	Manual     bool   `json:"manual"`     // use CRF/Preset below
	CRF        int    `json:"crf"`        // manual only
	Preset     string `json:"preset"`     // fast medium slow (manual only)
	Resolution string `json:"resolution"` // original 1080 720 480 custom
	Custom     int    `json:"custom"`     // short side in px when Resolution == custom
}

// AudioOptions come from the Audio page and the "Ambil audio saja" mode.
type AudioOptions struct {
	Format       string `json:"format"`     // mp3 m4a flac wav ogg opus
	Bitrate      int    `json:"bitrate"`    // kbps for lossy formats
	Channels     string `json:"channels"`   // source stereo mono
	SampleRate   string `json:"sampleRate"` // source 44100 48000
	KeepMetadata bool   `json:"keepMetadata"`
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

// OutputSize returns the width/height after scaling (for the UI "Hasil" column).
func OutputSize(w, h int, o VideoOptions) (int, int) {
	target := targetShortSide(o)
	if o.Format == "gif" && target == 0 {
		target = 480
	}
	if o.Codec == "copy" || ScaleFilter(w, h, target) == "" {
		return w, h
	}
	// ffmpeg's "-2" rounds the scaled side to the nearest even number.
	if w >= h {
		return evenRound(float64(w) * float64(target) / float64(h)), target
	}
	return target, evenRound(float64(h) * float64(target) / float64(w))
}

// VideoArgs builds the ffmpeg arguments (without global flags) to convert in → out.
func VideoArgs(in, out string, info ffmpeg.Info, o VideoOptions, encoders map[string]bool) ([]string, error) {
	o.Normalize()
	if !info.HasVideo {
		return nil, fmt.Errorf("file ini tidak punya video")
	}
	args := []string{"-i", in}

	if o.Format == "gif" {
		target := targetShortSide(o)
		if target == 0 {
			target = 480
		}
		scale := ScaleFilter(info.Width, info.Height, target)
		chain := "fps=12"
		if scale != "" {
			chain += "," + scale + ":flags=lanczos"
		}
		chain += ",split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=5"
		args = append(args, "-map", "0:v:0", "-vf", chain, "-loop", "0", "-f", "gif", out)
		return args, nil
	}

	if o.Codec == "copy" {
		if !CanCopyVideo(info.VideoCodec, o.Format) {
			return nil, fmt.Errorf("Video %s tidak bisa disalin ke %s tanpa encode ulang. Pilih codec lain (misalnya %s).",
				codecName(info.VideoCodec), strings.ToUpper(o.Format), strings.ToUpper(VideoFormats[o.Format][0]))
		}
		args = append(args, "-map", "0:v:0")
		if info.HasAudio {
			args = append(args, "-map", "0:a:0?")
		}
		args = append(args, "-c:v", "copy", "-sn", "-dn", "-map_metadata", "0")
		if info.HasAudio {
			if canCopyAudio(info.AudioCodec, o.Format) {
				args = append(args, "-c:a", "copy")
			} else {
				args = append(args, containerAudio(o.Format)...)
			}
		}
		if o.Format == "mp4" || o.Format == "mov" {
			args = append(args, "-movflags", "+faststart")
		}
		return append(args, out), nil
	}

	enc := PickEncoder(o.Codec, encoders)
	if enc == "" {
		return nil, fmt.Errorf("encoder %s tidak tersedia di FFmpeg ini", strings.ToUpper(o.Codec))
	}
	crf := crfFor(o.Codec, o.Quality)
	if o.Manual && o.CRF > 0 && o.CRF <= 63 {
		crf = o.CRF
	}
	args = append(args, "-map", "0:v:0")
	if info.HasAudio {
		args = append(args, "-map", "0:a:0?")
	}
	args = append(args, "-sn", "-dn", "-map_metadata", "0", "-c:v", enc)
	crfS := strconv.Itoa(crf)
	switch enc {
	case "libx264":
		args = append(args, "-crf", crfS, "-preset", o.Preset, "-pix_fmt", "yuv420p")
	case "libx265":
		args = append(args, "-crf", crfS, "-preset", o.Preset, "-pix_fmt", "yuv420p", "-x265-params", "log-level=error")
		if o.Format == "mp4" || o.Format == "mov" {
			args = append(args, "-tag:v", "hvc1")
		}
	case "libvpx-vp9":
		cpu := map[string]string{"fast": "5", "medium": "3", "slow": "1"}[o.Preset]
		args = append(args, "-crf", crfS, "-b:v", "0", "-row-mt", "1", "-deadline", "good", "-cpu-used", cpu, "-pix_fmt", "yuv420p")
	case "libsvtav1":
		p := map[string]string{"fast": "10", "medium": "8", "slow": "6"}[o.Preset]
		args = append(args, "-crf", crfS, "-preset", p, "-pix_fmt", "yuv420p")
	case "libaom-av1":
		cpu := map[string]string{"fast": "8", "medium": "6", "slow": "4"}[o.Preset]
		args = append(args, "-crf", crfS, "-b:v", "0", "-cpu-used", cpu, "-row-mt", "1", "-pix_fmt", "yuv420p")
	}
	if scale := ScaleFilter(info.Width, info.Height, targetShortSide(o)); scale != "" {
		args = append(args, "-vf", scale)
	}
	if info.HasAudio {
		args = append(args, containerAudio(o.Format)...)
	}
	if o.Format == "mp4" || o.Format == "mov" {
		args = append(args, "-movflags", "+faststart")
	}
	return append(args, out), nil
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
	names := map[string]string{"h264": "H.264", "hevc": "H.265", "vp9": "VP9", "vp8": "VP8", "av1": "AV1", "mpeg4": "MPEG-4", "prores": "ProRes"}
	if n, ok := names[c]; ok {
		return n
	}
	if c == "" {
		return "sumber"
	}
	return strings.ToUpper(c)
}

// Normalize fixes invalid audio options.
func (o *AudioOptions) Normalize() {
	if _, ok := AudioFormats[o.Format]; !ok {
		o.Format = "mp3"
	}
	switch o.Bitrate {
	case 64, 96, 128, 160, 192, 256, 320:
	default:
		o.Bitrate = 192
	}
	if o.Format == "opus" && o.Bitrate > 256 {
		o.Bitrate = 256
	}
	switch o.Channels {
	case "source", "stereo", "mono":
	default:
		o.Channels = "source"
	}
	switch o.SampleRate {
	case "source", "44100", "48000":
	default:
		o.SampleRate = "source"
	}
}

// AudioArgs builds ffmpeg arguments to convert the first audio stream of in → out.
func AudioArgs(in, out string, info ffmpeg.Info, o AudioOptions) ([]string, error) {
	o.Normalize()
	if !info.HasAudio {
		return nil, fmt.Errorf("file ini tidak punya audio")
	}
	args := []string{"-i", in, "-map", "0:a:0"}

	cover := o.KeepMetadata && info.CoverIndex >= 0 &&
		(info.CoverCodec == "mjpeg" || info.CoverCodec == "png") &&
		(o.Format == "mp3" || o.Format == "m4a" || o.Format == "flac")
	if cover {
		args = append(args, "-map", "0:"+strconv.Itoa(info.CoverIndex), "-c:v", "copy", "-disposition:v:0", "attached_pic")
	} else {
		args = append(args, "-vn")
	}
	if o.KeepMetadata {
		args = append(args, "-map_metadata", "0")
	} else {
		args = append(args, "-map_metadata", "-1")
	}
	args = append(args, "-sn", "-dn")

	br := strconv.Itoa(o.Bitrate) + "k"
	switch o.Format {
	case "mp3":
		args = append(args, "-c:a", "libmp3lame", "-b:a", br)
		if o.KeepMetadata {
			args = append(args, "-id3v2_version", "3")
		}
	case "m4a":
		args = append(args, "-c:a", "aac", "-b:a", br, "-movflags", "+faststart")
	case "ogg":
		args = append(args, "-c:a", "libvorbis", "-b:a", br)
	case "opus":
		args = append(args, "-c:a", "libopus", "-b:a", br)
	case "flac":
		args = append(args, "-c:a", "flac")
		if info.BitsPerSample > 16 {
			args = append(args, "-sample_fmt", "s32")
		} else {
			args = append(args, "-sample_fmt", "s16")
		}
	case "wav":
		if info.BitsPerSample > 16 {
			args = append(args, "-c:a", "pcm_s24le")
		} else {
			args = append(args, "-c:a", "pcm_s16le")
		}
	}
	switch o.Channels {
	case "stereo":
		args = append(args, "-ac", "2")
	case "mono":
		args = append(args, "-ac", "1")
	}
	if o.SampleRate != "source" && !(o.Format == "opus" && o.SampleRate != "48000") {
		args = append(args, "-ar", o.SampleRate)
	}
	return append(args, out), nil
}

func evenRound(v float64) int { return int(math.Round(v/2)) * 2 }
