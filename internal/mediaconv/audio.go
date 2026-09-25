package mediaconv

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/i18n"
)

// FormatOriginal keeps the source audio as it is (only tags and trimming change).
const FormatOriginal = "original"

// Normalize fixes invalid audio options.
func (o *AudioOptions) Normalize() {
	if _, ok := AudioFormats[o.Format]; !ok && o.Format != FormatOriginal {
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
	switch o.VBRLevel {
	case "best", "high", "medium", "small":
	default:
		o.VBRLevel = "high"
	}
	// AAC (M4A) has no reliable VBR in FFmpeg's own encoder: it stays constant bitrate.
	if o.Format != "mp3" && o.Format != "ogg" && o.Format != "opus" {
		o.VBR = false
	}
	switch o.Loudness {
	case -14, -16, -18, -23:
	default:
		o.Loudness = -16
	}
	if o.Speed < 0.5 || o.Speed > 2 {
		o.Speed = 1
	}
	o.Pitch = math.Max(-12, math.Min(12, math.Round(o.Pitch)))
	o.FadeIn = math.Max(0, math.Min(o.FadeIn, 60))
	o.FadeOut = math.Max(0, math.Min(o.FadeOut, 60))
}

// needsEncode reports whether the options change the sound (so it can't be copied).
func (o AudioOptions) needsEncode() bool {
	return o.NormVolume || o.FadeIn > 0 || o.FadeOut > 0 || o.Speed != 1 || o.Pitch != 0 || o.Channels != "source" || o.SampleRate != "source"
}

// AudioArgs builds ffmpeg arguments to convert the first audio stream of in → out.
func AudioArgs(in, out string, info ffmpeg.Info, o AudioOptions) ([]string, error) {
	p, err := AudioPlan(in, out, info, o, nil, 0, 0)
	if err != nil {
		return nil, err
	}
	return p.Passes[0], nil
}

// tempoChain splits a tempo factor into atempo steps within 0.5 … 2.
func tempoChain(f float64) []string {
	var out []string
	for f > 2 {
		out = append(out, "atempo=2")
		f /= 2
	}
	for f < 0.5 {
		out = append(out, "atempo=0.5")
		f /= 0.5
	}
	if math.Abs(f-1) > 1e-4 {
		out = append(out, "atempo="+strconv.FormatFloat(f, 'f', 6, 64))
	}
	return out
}

// soundFilters returns the audio filter chain for speed, pitch, loudness and fades.
// dur is the length of the result (for the fade out); rate the sample rate of the result.
func soundFilters(o AudioOptions, srcRate, rate int, dur float64) []string {
	var f []string
	if o.Pitch != 0 {
		p := math.Pow(2, o.Pitch/12)
		if srcRate <= 0 {
			srcRate = 44100
		}
		f = append(f, fmt.Sprintf("asetrate=%d", int(math.Round(float64(srcRate)*p))), fmt.Sprintf("aresample=%d", srcRate))
		f = append(f, tempoChain(o.Speed/p)...)
	} else {
		f = append(f, tempoChain(o.Speed)...)
	}
	if o.NormVolume {
		f = append(f, fmt.Sprintf("loudnorm=I=%s:TP=-1.5:LRA=11", strconv.FormatFloat(o.Loudness, 'f', -1, 64)))
		if rate <= 0 {
			rate = 48000
		}
		f = append(f, fmt.Sprintf("aresample=%d", rate)) // loudnorm works at 192 kHz
	}
	if o.FadeIn > 0 {
		f = append(f, "afade=t=in:st=0:d="+secs(o.FadeIn))
	}
	if o.FadeOut > 0 && dur > o.FadeOut {
		f = append(f, "afade=t=out:st="+secs(dur-o.FadeOut)+":d="+secs(o.FadeOut))
	}
	return f
}

func outRate(o AudioOptions, info ffmpeg.Info) int {
	switch {
	case o.Format == "opus":
		return 48000
	case o.SampleRate != "source":
		r, _ := strconv.Atoi(o.SampleRate)
		return r
	case info.SampleRate > 0:
		return info.SampleRate
	}
	return 48000
}

// codecArgs returns the encoder settings of an audio format.
// VBR quality per level: LAME -V (0 best … 9), Vorbis -q (10 best … 0), Opus target kbps.
var vbrLevels = map[string]struct {
	mp3, ogg, opus int
}{
	"best":   {0, 8, 192},
	"high":   {2, 6, 160},
	"medium": {4, 4, 128},
	"small":  {6, 2, 96},
}

// VBRKbps is the usual average bitrate of a VBR level (for the UI and size estimates).
func VBRKbps(format, level string) int {
	avg := map[string]map[string]int{
		"mp3":  {"best": 245, "high": 190, "medium": 165, "small": 130},
		"ogg":  {"best": 256, "high": 192, "medium": 128, "small": 96},
		"opus": {"best": 192, "high": 160, "medium": 128, "small": 96},
	}
	return avg[format][level]
}

func codecArgs(o AudioOptions, bits int) []string {
	br := strconv.Itoa(o.Bitrate) + "k"
	lv := vbrLevels[o.VBRLevel]
	switch o.Format {
	case "mp3":
		a := []string{"-c:a", "libmp3lame", "-b:a", br}
		if o.VBR {
			a = []string{"-c:a", "libmp3lame", "-q:a", strconv.Itoa(lv.mp3)}
		}
		if o.KeepMetadata {
			a = append(a, "-id3v2_version", "3")
		}
		return a
	case "m4a":
		return []string{"-c:a", "aac", "-b:a", br, "-movflags", "+faststart"}
	case "ogg":
		if o.VBR {
			return []string{"-c:a", "libvorbis", "-q:a", strconv.Itoa(lv.ogg)}
		}
		return []string{"-c:a", "libvorbis", "-b:a", br}
	case "opus":
		if o.VBR {
			return []string{"-c:a", "libopus", "-b:a", strconv.Itoa(lv.opus) + "k", "-vbr", "on"}
		}
		// A fixed rate: constrained VBR keeps the size predictable.
		return []string{"-c:a", "libopus", "-b:a", br, "-vbr", "constrained"}
	case "flac":
		if bits > 16 {
			return []string{"-c:a", "flac", "-sample_fmt", "s32"}
		}
		return []string{"-c:a", "flac", "-sample_fmt", "s16"}
	case "wav":
		if bits > 16 {
			return []string{"-c:a", "pcm_s24le"}
		}
		return []string{"-c:a", "pcm_s16le"}
	}
	return []string{"-c:a", "copy"}
}

func tagArgs(t *Tags, format string) []string {
	if t == nil {
		return nil
	}
	kv := [][2]string{{"title", t.Title}, {"artist", t.Artist}, {"album", t.Album}, {"album_artist", t.AlbumArtist},
		{"date", t.Year}, {"genre", t.Genre}, {"track", t.Track}}
	var a []string
	for _, p := range kv {
		a = append(a, "-metadata", p[0]+"="+strings.TrimSpace(p[1]))
	}
	if format == "mp3" {
		a = append(a, "-id3v2_version", "3")
	}
	return a
}

// coverFormats can hold an embedded picture.
var coverFormats = map[string]bool{"mp3": true, "m4a": true, "flac": true}

// AudioPlan converts the first audio stream of in → out. tags (optional) come from the tag
// editor. lead/trail cut silence found by ffmpeg.Silence (0 = none).
func AudioPlan(in, out string, info ffmpeg.Info, o AudioOptions, tags *Tags, lead, trail float64) (Plan, error) {
	o.Normalize()
	if !info.HasAudio {
		return Plan{}, errors.New(i18n.L("file ini tidak punya audio", "this file has no audio"))
	}
	format := o.Format
	if format == FormatOriginal {
		if o.needsEncode() {
			return Plan{}, errors.New(i18n.L("\"Format asli\" hanya menyalin audio. Matikan efek (normalisasi, fade, kecepatan) atau pilih format lain.", "\"Original format\" only copies the audio. Turn off effects (normalize, fade, speed) or pick another format."))
		}
		format = strings.TrimPrefix(strings.ToLower(extOf(out)), ".")
	}
	start, length, err := trimRange(o.TrimStart, o.TrimEnd, info.Duration)
	if err != nil {
		return Plan{}, err
	}
	if lead > 0 || trail > 0 {
		end := 0.0
		if length > 0 {
			end = start + length
		}
		if lead > start {
			start = math.Max(0, lead-0.05)
		}
		if trail > 0 && (end == 0 || trail < end) {
			end = trail + 0.05
		}
		if end > 0 {
			length = math.Max(0.1, end-start)
		}
	}
	srcDur := outDuration(info.Duration, start, length)
	dur := srcDur / o.Speed

	args := inputArgs(in, start, length)
	coverIn := tags != nil && tags.Cover != "" && coverFormats[format]
	if coverIn {
		args = append(args, "-i", tags.Cover)
	}
	args = append(args, "-map", "0:a:0")

	keepCover := o.KeepMetadata && info.CoverIndex >= 0 && (info.CoverCodec == "mjpeg" || info.CoverCodec == "png") && coverFormats[format]
	if tags != nil && tags.RemoveCover {
		keepCover = false
	}
	switch {
	case coverIn:
		args = append(args, "-map", "1:v:0", "-c:v", "mjpeg", "-disposition:v:0", "attached_pic")
	case keepCover:
		args = append(args, "-map", "0:"+strconv.Itoa(info.CoverIndex), "-c:v", "copy", "-disposition:v:0", "attached_pic")
	default:
		args = append(args, "-vn")
	}
	if o.KeepMetadata || tags != nil {
		args = append(args, "-map_metadata", "0")
	} else {
		args = append(args, "-map_metadata", "-1")
	}
	args = append(args, "-sn", "-dn")

	if o.Format == FormatOriginal {
		args = append(args, "-c:a", "copy")
		if format == "mp3" && (o.KeepMetadata || tags != nil) {
			args = append(args, "-id3v2_version", "3")
		}
	} else {
		args = append(args, codecArgs(o, info.BitsPerSample)...)
		rate := outRate(o, info)
		if chain := soundFilters(o, info.SampleRate, rate, dur); len(chain) > 0 {
			args = append(args, "-af", strings.Join(chain, ","))
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
	}
	args = append(args, tagArgs(tags, format)...)
	return Plan{Passes: [][]string{append(args, out)}, Duration: dur}, nil
}

// MergeAudioPlan joins the audio of several files into one, in order.
func MergeAudioPlan(ins []string, infos []ffmpeg.Info, out string, o AudioOptions) (Plan, error) {
	o.Normalize()
	if o.Format == FormatOriginal {
		o.Format = "mp3"
	}
	if len(ins) < 2 {
		return Plan{}, errors.New(i18n.L("Pilih minimal 2 file untuk digabung", "Pick at least 2 files to join"))
	}
	rate := outRate(o, infos[0])
	layout := "stereo"
	if o.Channels == "mono" {
		layout = "mono"
	}
	var args, graph []string
	var cat strings.Builder
	total := 0.0
	bits := 16
	for i, in := range ins {
		if !infos[i].HasAudio {
			return Plan{}, fmt.Errorf(i18n.L("file ke-%d tidak punya audio", "file %d has no audio"), i+1)
		}
		args = append(args, "-i", in)
		total += infos[i].Duration
		bits = max(bits, infos[i].BitsPerSample)
		graph = append(graph, fmt.Sprintf("[%d:a:0]aresample=%d,aformat=sample_fmts=fltp:channel_layouts=%s[a%d]", i, rate, layout, i))
		fmt.Fprintf(&cat, "[a%d]", i)
	}
	dur := total / o.Speed
	last := fmt.Sprintf("%sconcat=n=%d:v=0:a=1", cat.String(), len(ins))
	if chain := soundFilters(o, rate, rate, dur); len(chain) > 0 {
		last += "," + strings.Join(chain, ",")
	}
	graph = append(graph, last+"[out]")
	args = append(args, "-filter_complex", strings.Join(graph, ";"), "-map", "[out]", "-map_metadata", "-1", "-vn", "-sn", "-dn")
	args = append(args, codecArgs(o, bits)...)
	return Plan{Passes: [][]string{append(args, out)}, Duration: dur}, nil
}

func extOf(p string) string {
	if i := strings.LastIndexAny(p, `.\/`); i >= 0 && p[i] == '.' {
		return p[i:]
	}
	return ""
}
