package mediaconv

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/imageconv"
)

// CropBox is a manual crop in fractions (0..1) of the rotated frame. W == 0 means no crop.
type CropBox struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Music is a background track added to (or replacing) the sound of a video.
type Music struct {
	File     string  `json:"file"`     // "" = no music
	Mode     string  `json:"mode"`     // mix | replace
	Volume   float64 `json:"volume"`   // music level, 0.05 … 2 (1 = as recorded)
	Original float64 `json:"original"` // level of the video's own sound in mix mode, 0 … 2
	Duck     bool    `json:"duck"`     // lower the music while the video's sound is loud (speech)
	Loop     bool    `json:"loop"`     // repeat the music until the video ends
}

// Frames of the "Bingkai" choice: width:height.
var frameRatios = map[string][2]float64{"9:16": {9, 16}, "1:1": {1, 1}, "4:5": {4, 5}, "16:9": {16, 9}, "4:3": {4, 3}}

// Denoise strengths for FFmpeg's afftdn (noise reduction in dB, noise floor in dB).
var denoiseLevels = map[string][2]int{"light": {10, -50}, "medium": {20, -40}, "strong": {30, -32}}

// DenoiseFilter returns the audio filter of a noise-reduction level ("" for off).
func DenoiseFilter(level string) string {
	l, ok := denoiseLevels[level]
	if !ok {
		return ""
	}
	// A gentle high-pass first removes rumble (wind, air conditioning) the FFT filter keeps.
	return fmt.Sprintf("highpass=f=70,afftdn=nr=%d:nf=%d:tn=1", l[0], l[1])
}

func normalizeDenoise(s string) string {
	if _, ok := denoiseLevels[s]; ok {
		return s
	}
	return "off"
}

// normalizeEdits fixes the editing options (called from Normalize).
func (o *VideoOptions) normalizeEdits() {
	if o.Speed < 0.25 || o.Speed > 4 {
		o.Speed = 1
	}
	o.Speed = math.Round(o.Speed*100) / 100
	c := &o.Crop
	c.X, c.Y = clamp01(c.X), clamp01(c.Y)
	c.W, c.H = math.Min(clamp01(c.W), 1-c.X), math.Min(clamp01(c.H), 1-c.Y)
	if c.W < 0.02 || c.H < 0.02 || (c.W > 0.999 && c.H > 0.999) {
		*c = CropBox{}
	}
	if _, ok := frameRatios[o.Frame]; !ok {
		o.Frame = ""
	}
	switch o.FrameFit {
	case "crop", "pad", "blur":
	default:
		o.FrameFit = "blur"
	}
	o.Denoise = normalizeDenoise(o.Denoise)
	m := &o.Music
	if strings.TrimSpace(m.File) == "" {
		*m = Music{}
	} else {
		if m.Mode != "replace" {
			m.Mode = "mix"
		}
		if m.Volume <= 0 || m.Volume > 2 {
			m.Volume = 1
		}
		m.Original = math.Max(0, math.Min(m.Original, 2))
	}
	o.Watermark.Normalize()
	if o.Format == "gif" {
		o.Stabilize = o.Stabilize && o.Codec != "copy"
		o.Music, o.Denoise = Music{}, "off"
	}
	if o.AudioMode == "mute" {
		o.Music, o.Denoise = Music{}, "off"
	}
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return math.Max(0, math.Min(v, 1))
}

// visualEdits reports options that change the picture (they need re-encoding).
func (o VideoOptions) visualEdits() bool {
	return o.Speed != 1 || o.Reverse || o.Crop.W > 0 || o.Frame != "" || o.Stabilize || o.Watermark.Enabled
}

// soundEdits reports options that change the sound.
func (o VideoOptions) soundEdits() bool {
	return o.Speed != 1 || o.Reverse || o.Denoise != "off" || o.Music.File != ""
}

// contentSize is the picture size after rotation and the manual crop.
func contentSize(w, h int, o VideoOptions) (int, int) {
	w, h = rotated(w, h, o)
	if o.Crop.W > 0 && w > 0 && h > 0 {
		w = max(2, evenFloor(float64(w)*o.Crop.W))
		h = max(2, evenFloor(float64(h)*o.Crop.H))
	}
	return w, h
}

// frameArea is the canvas of a frame choice before scaling: the largest centred area of the
// ratio for "crop", otherwise a canvas whose short side equals the picture's short side.
func frameArea(w, h int, o VideoOptions) (int, int) {
	r, ok := frameRatios[o.Frame]
	if !ok || w <= 0 || h <= 0 {
		return w, h
	}
	want := r[0] / r[1]
	if o.FrameFit == "crop" {
		if float64(w)/float64(h) > want {
			return max(2, evenFloor(float64(h)*want)), h - h%2
		}
		return w - w%2, max(2, evenFloor(float64(w)/want))
	}
	short := min(w, h)
	if want >= 1 {
		return evenRound(float64(short) * want), short - short%2
	}
	return short - short%2, evenRound(float64(short) / want)
}

// scaledSize applies the resolution choice (short side, never upscaling) like ScaleFilter.
func scaledSize(w, h, target int) (int, int) {
	if ScaleFilter(w, h, target) == "" {
		return w, h
	}
	if w >= h {
		return evenRound(float64(w) * float64(target) / float64(h)), target
	}
	return target, evenRound(float64(h) * float64(target) / float64(w))
}

func evenFloor(v float64) int { return int(v/2) * 2 }

// graph builds an ffmpeg filtergraph. Linear filters collect in chain; a step that needs
// several streams (overlay, split) closes the chain into a labelled segment.
type graph struct {
	parts   []string
	chain   []string
	label   string // input of the current chain
	seq     int
	complex bool
}

func newGraph(input string) *graph { return &graph{label: input} }

func (g *graph) add(f ...string) {
	for _, s := range f {
		if s != "" {
			g.chain = append(g.chain, s)
		}
	}
}

func (g *graph) next(prefix string) string {
	g.seq++
	return fmt.Sprintf("%s%d", prefix, g.seq)
}

// flush closes the pending chain and returns the label that holds its result.
func (g *graph) flush(prefix string) string {
	if len(g.chain) == 0 {
		return g.label
	}
	out := g.next(prefix)
	g.parts = append(g.parts, fmt.Sprintf("[%s]%s[%s]", g.label, strings.Join(g.chain, ","), out))
	g.chain, g.label = nil, out
	return out
}

// raw adds a segment that reads the current stream (as {in}) and writes {out}.
func (g *graph) raw(prefix, seg string) {
	in := g.flush(prefix)
	out := g.next(prefix)
	seg = strings.ReplaceAll(seg, "{in}", "["+in+"]")
	seg = strings.ReplaceAll(seg, "{out}", "["+out+"]")
	g.parts = append(g.parts, seg)
	g.label = out
	g.complex = true
}

// finish returns the whole graph ending in the label out.
func (g *graph) finish(out string) string {
	chain := strings.Join(g.chain, ",")
	if chain == "" {
		chain = "null"
		if strings.HasPrefix(out, "a") {
			chain = "anull"
		}
	}
	return strings.Join(append(append([]string{}, g.parts...), fmt.Sprintf("[%s]%s[%s]", g.label, chain, out)), ";")
}

// linear returns the pending chain when nothing needed several streams.
func (g *graph) linear() (string, bool) {
	if g.complex || len(g.parts) > 0 {
		return "", false
	}
	return strings.Join(g.chain, ","), true
}

// pictureSteps adds rotation, stabilisation, crop, frame and scaling to g for a source of
// w×h (as displayed). target is the short side to scale to (0 = keep). It returns the size of
// the picture afterwards. lanczos picks the sharper scaler (GIF).
func pictureSteps(g *graph, w, h int, o VideoOptions, target int, lanczos bool) (int, int) {
	g.add(transformFilters(o)...)
	if o.Stabilize {
		g.add("vidstabtransform=input=kmbstab.trf:smoothing=20:optzoom=1:zoomspeed=0.25:interpol=bicubic", "unsharp=5:5:0.8:3:3:0.4")
	}
	rw, rh := rotated(w, h, o)
	cw, ch := contentSize(w, h, o)
	if o.Crop.W > 0 {
		x := min(evenFloor(float64(rw)*o.Crop.X), rw-cw)
		y := min(evenFloor(float64(rh)*o.Crop.Y), rh-ch)
		g.add(fmt.Sprintf("crop=%d:%d:%d:%d", cw, ch, max(0, x), max(0, y)))
	}
	flags := ""
	if lanczos {
		flags = ":flags=lanczos"
	}
	if o.Frame == "" {
		if scale := ScaleFilter(cw, ch, target); scale != "" {
			g.add(scale + flags)
		}
		return scaledSize(cw, ch, target)
	}
	aw, ah := frameArea(cw, ch, o)
	W, H := scaledSize(aw, ah, target)
	switch o.FrameFit {
	case "crop":
		g.add(fmt.Sprintf("crop=%d:%d:(iw-%d)/2:(ih-%d)/2", aw, ah, aw, ah))
		if W != aw || H != ah {
			g.add(fmt.Sprintf("scale=%d:%d%s", W, H, flags))
		}
		g.add("setsar=1")
	case "pad":
		g.add(fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2%s", W, H, flags),
			fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black", W, H), "setsar=1")
	default: // blur: the picture itself, enlarged and blurred, fills the empty space
		bw, bh := max(2, evenRound(float64(W)/4)), max(2, evenRound(float64(H)/4))
		g.raw("v", fmt.Sprintf("{in}split=2[bgs][fgs];"+
			"[bgs]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,boxblur=12:2,scale=%d:%d,setsar=1[bgb];"+
			"[fgs]scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2%s,setsar=1[fgb];"+
			"[bgb][fgb]overlay=(W-w)/2:(H-h)/2{out}", bw, bh, bw, bh, W, H, W, H, flags))
	}
	return W, H
}

// timeSteps adds reverse and speed to a picture chain. srcFPS keeps the frame rate when the
// speed changes (otherwise 2× would double it).
func timeSteps(g *graph, o VideoOptions, srcFPS float64) {
	if o.Reverse {
		g.add("reverse")
	}
	if o.Speed != 1 {
		g.add("setpts=PTS/" + strconv.FormatFloat(o.Speed, 'f', -1, 64))
		if fpsValue(o) == "" && o.Format != "gif" && srcFPS > 0 && srcFPS <= 240 {
			g.add("fps=" + strconv.FormatFloat(math.Round(srcFPS*1000)/1000, 'f', -1, 64))
		}
	}
}

// overlayWatermark puts the watermark picture (input index idx, already the size of the
// picture) over the current stream.
func overlayWatermark(g *graph, idx int) {
	g.raw("v", fmt.Sprintf("{in}[%d:v:0]overlay=0:0:format=auto{out}", idx))
}

// soundSteps builds the audio graph: the video's own sound (reversed, sped up, cleaned) and
// the background music. musicIdx < 0 means no music; dur is the length of the result.
func soundSteps(o VideoOptions, info ffmpeg.Info, musicIdx int, dur float64) *graph {
	own := info.HasAudio && !(o.Music.File != "" && o.Music.Mode == "replace")
	var g *graph
	if own {
		g = newGraph("0:a:0")
		if o.Reverse {
			g.add("areverse")
		}
		g.add(tempoChain(o.Speed)...)
		if f := DenoiseFilter(o.Denoise); f != "" {
			g.add(f)
		}
	}
	if musicIdx < 0 {
		return g
	}
	m := o.Music
	music := []string{"aresample=48000", "aformat=sample_fmts=fltp:channel_layouts=stereo", "volume=" + strconv.FormatFloat(m.Volume, 'f', 2, 64)}
	if dur > 0 {
		music = append(music, "atrim=0:"+secs(dur), "apad=whole_dur="+secs(dur))
		if dur > 6 {
			music = append(music, "afade=t=out:st="+secs(dur-2)+":d=2")
		}
	}
	if g == nil {
		g = newGraph(fmt.Sprintf("%d:a:0", musicIdx))
		g.add(music...)
		return g
	}
	g.add("aresample=48000", "aformat=sample_fmts=fltp:channel_layouts=stereo", "volume="+strconv.FormatFloat(math.Max(m.Original, 0.01), 'f', 2, 64))
	if m.Duck {
		g.raw("a", fmt.Sprintf("{in}asplit=2[own][side];[%d:a:0]%s[mus];[mus][side]sidechaincompress=threshold=0.03:ratio=8:attack=40:release=700:makeup=1[duck];[own][duck]amix=inputs=2:duration=first:dropout_transition=0:normalize=0{out}",
			musicIdx, strings.Join(music, ",")))
	} else {
		g.raw("a", fmt.Sprintf("[%d:a:0]%s[mus];{in}[mus]amix=inputs=2:duration=first:dropout_transition=0:normalize=0{out}",
			musicIdx, strings.Join(music, ",")))
	}
	return g
}

// checkEdits refuses combinations that can't work.
func (o VideoOptions) checkEdits(info ffmpeg.Info, outW, outH int, dur float64, work string) error {
	if o.Codec == "copy" && o.Format != "gif" && o.visualEdits() {
		return errors.New(i18n.L("Efek video (kecepatan, putar balik, crop, bingkai, stabilisasi, watermark) butuh encode ulang. Pilih codec selain \"salin\".", "Video effects (speed, reverse, crop, frame, stabilisation, watermark) need re-encoding. Pick a codec other than \"copy\"."))
	}
	if (o.Stabilize || o.Watermark.Enabled) && work == "" {
		return errors.New("effects need a work folder")
	}
	if o.Watermark.Enabled && o.WatermarkFile == "" {
		return errors.New("watermark picture missing")
	}
	if o.Subtitles == "embed" && (o.Speed != 1 || o.Reverse) {
		return errors.New(i18n.L("Subtitle yang disematkan tidak cocok lagi setelah kecepatan diubah atau video diputar balik. Pilih \"Bakar\" atau matikan subtitle.", "Embedded subtitles no longer match after changing the speed or reversing. Choose \"Burn in\" or turn subtitles off."))
	}
	if o.Reverse {
		// "reverse" keeps every frame in memory: refuse what would need more than ±2 GB.
		fps := info.FPS
		if f, err := strconv.ParseFloat(fpsValue(o), 64); err == nil && f > 0 {
			fps = f
		}
		if fps <= 0 || fps > 240 {
			fps = 30
		}
		src := dur * o.Speed
		need := float64(outW*outH) * 1.5 * fps * src
		if need > 2e9 || src <= 0 {
			limit := 2e9 / (float64(max(outW*outH, 1)) * 1.5 * fps)
			return fmt.Errorf(i18n.L("Putar balik menyimpan semua frame di memori, jadi hanya untuk klip pendek: maksimal ±%.0f detik pada resolusi ini. Potong videonya atau pilih resolusi lebih kecil.", "Reverse keeps every frame in memory, so it's for short clips only: at most ±%.0f seconds at this resolution. Trim the video or pick a smaller resolution."), math.Floor(limit))
		}
	}
	return nil
}

// WatermarkLayer draws the watermark on a transparent picture of the video size.
func WatermarkLayer(w imageconv.Watermark, width, height int, out, ffmpegPath string) error {
	return imageconv.WatermarkLayer(w, width, height, out, ffmpegPath)
}
