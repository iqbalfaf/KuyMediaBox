package mediaconv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kuymediabox/internal/ffmpeg"
)

var hd = ffmpeg.Info{HasVideo: true, HasAudio: true, Width: 1920, Height: 1080, Duration: 100, FPS: 29.97, VideoCodec: "h264", AudioCodec: "aac", SampleRate: 44100, CoverIndex: -1}

func TestParseTime(t *testing.T) {
	for in, want := range map[string]float64{"": 0, "90": 90, "1:30": 90, "01:02:03": 3723, "1:30,5": 90.5} {
		if got, ok := ParseTime(in); !ok || got != want {
			t.Errorf("%q: %v %v", in, got, ok)
		}
	}
	for _, bad := range []string{"a", "1:75", "-3", "1:2:3:4"} {
		if _, ok := ParseTime(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestTargetSizeTwoPass(t *testing.T) {
	work := t.TempDir()
	o := VideoOptions{Format: "mp4", Codec: "h264", TargetMB: 16, TrimStart: "0:10", TrimEnd: "0:50"}
	p, err := VideoPlan("in.mp4", "out.mp4", hd, o, map[string]bool{"libx264": true}, work)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Passes) != 2 || p.Dir != work || p.Duration != 40 {
		t.Fatalf("plan %+v", p)
	}
	p1, p2 := joined(p.Passes[0]), joined(p.Passes[1])
	// 16 MB over 40 s ≈ 3072 kbit/s minus 160 kbit/s audio.
	for _, want := range []string{"-ss 10.000 -t 40.000 -i in.mp4", "-pass 1", "-b:v 2912k", "-f null -"} {
		if !strings.Contains(p1, want) {
			t.Errorf("pass 1 missing %q: %s", want, p1)
		}
	}
	if !strings.Contains(p2, "-pass 2") || !strings.Contains(p2, "-c:a aac") || strings.Contains(p1, "-c:a") {
		t.Errorf("pass 2: %s", p2)
	}
	o.TargetMB = 0.1
	if _, err := VideoPlan("in.mp4", "out.mp4", hd, o, map[string]bool{"libx264": true}, work); err == nil {
		t.Fatal("tiny target must be refused")
	}
}

func TestVideoFiltersAudioAndHW(t *testing.T) {
	o := VideoOptions{Format: "mp4", Codec: "h265", HW: "nvenc", Resolution: "720", FPS: "30", Rotate: 90, FlipH: true, AudioMode: "mute"}
	p, err := VideoPlan("in.mp4", "out.mp4", hd, o, map[string]bool{"hevc_nvenc": true}, "")
	if err != nil {
		t.Fatal(err)
	}
	s := joined(p.Passes[0])
	for _, want := range []string{"-c:v hevc_nvenc", "-cq", "-vf transpose=1,hflip,scale=720:-2,fps=30", "-an", "-tag:v hvc1"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q: %s", want, s)
		}
	}
	if strings.Contains(s, "0:a:0") {
		t.Errorf("muted video must not map audio: %s", s)
	}
	if w, h := OutputSize(1920, 1080, o); w != 720 || h != 1280 {
		t.Errorf("rotated size %dx%d", w, h)
	}
	if _, err := VideoPlan("in.mp4", "out.mp4", hd, o, map[string]bool{}, ""); err == nil {
		t.Error("missing GPU encoder must fail")
	}
	o = VideoOptions{Format: "webm", Codec: "vp9", AudioMode: "mp3", AudioBitrate: 96}
	p, _ = VideoPlan("in.mp4", "out.webm", hd, o, map[string]bool{"libvpx-vp9": true}, "")
	if !strings.Contains(joined(p.Passes[0]), "-c:a libopus -b:a 96k") {
		t.Errorf("webm audio must become opus: %s", joined(p.Passes[0]))
	}
}

func TestSubtitles(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "film.mkv")
	sub := filepath.Join(dir, "film.id.srt")
	_ = os.WriteFile(video, nil, 0o644)
	_ = os.WriteFile(sub, []byte("1\n00:00:01,000 --> 00:00:02,000\nHalo dünya\xe9\n"), 0o644)
	if got := FindSubtitle(video); got != sub {
		t.Fatalf("FindSubtitle = %q", got)
	}
	o := VideoOptions{Format: "mp4", Codec: "h264", Subtitles: "embed", SubFile: sub}
	p, err := VideoPlan(video, "out.mp4", hd, o, nil, "")
	if err != nil || !strings.Contains(joined(p.Passes[0]), "-map 1:0") || !strings.Contains(joined(p.Passes[0]), "-c:s mov_text") {
		t.Fatalf("embed: %v %s", err, joined(p.Passes[0]))
	}
	work := t.TempDir()
	o.Subtitles = "burn"
	o.TrimStart = "5"
	p, err = VideoPlan(video, "out.mp4", hd, o, nil, work)
	if err != nil {
		t.Fatal(err)
	}
	s := joined(p.Passes[0])
	if !strings.Contains(s, "setpts=PTS+5.000/TB,subtitles=sub.srt:charenc=CP1252,setpts=PTS-STARTPTS") || p.Dir != work {
		t.Fatalf("burn: %s", s)
	}
	if _, err := os.Stat(filepath.Join(work, "sub.srt")); err != nil {
		t.Fatal("subtitle not copied")
	}
	pgs := hd
	pgs.SubCodec = "hdmv_pgs_subtitle"
	if _, err := VideoPlan(video, "out.mp4", pgs, VideoOptions{Format: "mp4", Subtitles: "burn"}, nil, work); err == nil {
		t.Fatal("picture subtitles can't be burned")
	}
}

func TestFramesAndMerge(t *testing.T) {
	p, err := FramesPlan("in.mp4", `out\f_%04d.jpg`, hd, VideoOptions{Resolution: "720"}, 5, "jpg")
	if err != nil || !strings.Contains(joined(p.Passes[0]), "fps=1/5") || !strings.Contains(joined(p.Passes[0]), "-q:v 2") {
		t.Fatalf("frames: %v %s", err, joined(p.Passes[0]))
	}
	noAudio := hd
	noAudio.HasAudio = false
	noAudio.Width, noAudio.Height = 1280, 720
	p, err = MergeVideoPlan([]string{"a.mp4", "b.mp4"}, []ffmpeg.Info{hd, noAudio}, "out.mp4", VideoOptions{Format: "mp4", Codec: "h264", Resolution: "720"}, map[string]bool{"libx264": true})
	if err != nil {
		t.Fatal(err)
	}
	s := joined(p.Passes[0])
	for _, want := range []string{"scale=1280:720:force_original_aspect_ratio=decrease", "anullsrc", "concat=n=2:v=1:a=1[vout][aout]", "fps=30"} {
		if !strings.Contains(s, want) {
			t.Errorf("merge missing %q: %s", want, s)
		}
	}
	if p.Duration != 200 {
		t.Errorf("duration %v", p.Duration)
	}
}

func TestAudioEffects(t *testing.T) {
	info := ffmpeg.Info{HasAudio: true, Duration: 60, SampleRate: 44100, CoverIndex: -1}
	o := AudioOptions{Format: "mp3", Bitrate: 192, TrimStart: "10", TrimEnd: "40", FadeIn: 2, FadeOut: 3, NormVolume: true, Loudness: -14, Speed: 1.5, Pitch: 2}
	p, err := AudioPlan("a.wav", "a.mp3", info, o, &Tags{Title: "Judul", Artist: "Artis", Cover: "c.jpg"}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	s := joined(p.Passes[0])
	for _, want := range []string{"-ss 10.000", "-t 30.000", "asetrate=49501", "atempo=", "loudnorm=I=-14", "aresample=44100", "afade=t=in:st=0:d=2.000", "afade=t=out:st=17.000:d=3.000",
		"-map 1:v:0", "attached_pic", "-metadata title=Judul", "-metadata album="} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q: %s", want, s)
		}
	}
	if p.Duration != 20 {
		t.Errorf("duration %v, want 20 (30 s at 1.5×)", p.Duration)
	}
	// Silence cut.
	p, _ = AudioPlan("a.wav", "a.flac", info, AudioOptions{Format: "flac"}, nil, 1.5, 55)
	if s := joined(p.Passes[0]); !strings.Contains(s, "-ss 1.450") || !strings.Contains(s, "-t 53.600") {
		t.Errorf("silence trim: %s", s)
	}
	// Original format copies.
	p, err = AudioPlan("a.m4a", "b.m4a", info, AudioOptions{Format: FormatOriginal}, &Tags{Genre: "Pop"}, 0, 0)
	if err != nil || !strings.Contains(joined(p.Passes[0]), "-c:a copy") {
		t.Fatalf("original: %v %s", err, joined(p.Passes[0]))
	}
	if _, err := AudioPlan("a.m4a", "b.m4a", info, AudioOptions{Format: FormatOriginal, NormVolume: true}, nil, 0, 0); err == nil {
		t.Error("original + effects must fail")
	}
	p, err = MergeAudioPlan([]string{"a.mp3", "b.wav"}, []ffmpeg.Info{info, info}, "o.mp3", AudioOptions{Format: "mp3"})
	if err != nil || !strings.Contains(joined(p.Passes[0]), "concat=n=2:v=0:a=1") {
		t.Fatalf("merge audio: %v %s", err, joined(p.Passes[0]))
	}
	if got := tempoChain(4); len(got) != 2 || got[0] != "atempo=2" {
		t.Errorf("tempo chain %v", got)
	}
}

func TestBitrateAndVBR(t *testing.T) {
	o := VideoOptions{Format: "mp4", Codec: "h264", BitrateK: 2500}
	p, err := VideoPlan("in.mp4", "out.mp4", hd, o, map[string]bool{"libx264": true}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := joined(p.Passes[0])
	if len(p.Passes) != 1 || !strings.Contains(s, "-b:v 2500k -maxrate 3750k -bufsize 5000k") || strings.Contains(s, "-crf") || strings.Contains(s, "-pass") {
		t.Fatalf("bitrate mode: %d passes, %s", len(p.Passes), s)
	}
	// A target size wins over a bitrate.
	o.TargetMB = 16
	o.Normalize()
	if o.BitrateK != 0 {
		t.Fatal("target size must clear the bitrate")
	}
	info := ffmpeg.Info{HasAudio: true, Duration: 60, SampleRate: 44100, CoverIndex: -1}
	for format, want := range map[string]string{"mp3": "-q:a 2", "ogg": "-q:a 6", "opus": "-b:a 160k -vbr on"} {
		args, err := AudioArgs("a.wav", "a."+format, info, AudioOptions{Format: format, VBR: true, VBRLevel: "high"})
		if err != nil || !strings.Contains(joined(args), want) || (format != "opus" && strings.Contains(joined(args), "-b:a")) {
			t.Errorf("%s VBR: %v %s", format, err, joined(args))
		}
	}
	args, _ := AudioArgs("a.wav", "a.m4a", info, AudioOptions{Format: "m4a", VBR: true, Bitrate: 64})
	if !strings.Contains(joined(args), "-b:a 64k") {
		t.Errorf("m4a stays constant bitrate: %s", joined(args))
	}
	if VBRKbps("mp3", "best") != 245 {
		t.Error("VBR estimate")
	}
}
