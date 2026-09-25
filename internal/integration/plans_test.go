//go:build integration

package integration

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/mediaconv"
	"kuymediabox/internal/proc"
	"kuymediabox/internal/tools"
)

func runPlan(t *testing.T, ff string, p mediaconv.Plan) {
	t.Helper()
	ctx := context.Background()
	for _, a := range p.Prep {
		if err := ffmpeg.RunIn(ctx, ff, p.Dir, a, 0, nil); err != nil {
			t.Fatalf("prep %v: %v", a, err)
		}
	}
	for _, a := range p.Passes {
		if err := ffmpeg.RunIn(ctx, ff, p.Dir, a, p.Duration, nil); err != nil {
			t.Fatalf("ffmpeg %s: %v", strings.Join(a, " "), err)
		}
	}
}

func probe(t *testing.T, fp, path string) ffmpeg.Info {
	t.Helper()
	info, err := ffmpeg.Probe(context.Background(), fp, path)
	if err != nil {
		t.Fatalf("probe %s: %v", path, err)
	}
	return info
}

// TestPlansWithFFmpeg runs the new conversion features with the real ffmpeg.
func TestPlansWithFFmpeg(t *testing.T) {
	m := setup(t)
	ff, fp := m.Path(tools.FFmpeg), m.Path(tools.FFprobe)
	ctx := context.Background()
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.mp4")
	quiet := filepath.Join(dir, "quiet.mp4")
	if _, err := proc.Output(ctx, ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=30:duration=12",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=12", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", clip); err != nil {
		t.Fatal(err)
	}
	if _, err := proc.Output(ctx, ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=640x360:rate=25:duration=4", "-c:v", "libx264", "-pix_fmt", "yuv420p", quiet); err != nil {
		t.Fatal(err)
	}
	srt := filepath.Join(dir, "clip.srt")
	_ = os.WriteFile(srt, []byte("1\n00:00:01,000 --> 00:00:09,000\nHalo dunia — subtitle\n"), 0o644)
	info := probe(t, fp, clip)
	enc, err := ffmpeg.Encoders(ctx, ff)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("target size two-pass", func(t *testing.T) {
		work := t.TempDir()
		out := filepath.Join(dir, "small.mp4")
		p, err := mediaconv.VideoPlan(clip, out, info, mediaconv.VideoOptions{Format: "mp4", Codec: "h264", TargetMB: 1, Resolution: "480"}, enc, work)
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		st, _ := os.Stat(out)
		if st.Size() > 1_050_000 || st.Size() < 300_000 {
			t.Fatalf("size %d, want about 1 MB", st.Size())
		}
		if got := probe(t, fp, out); got.Height != 480 {
			t.Fatalf("height %d", got.Height)
		}
	})

	t.Run("trim fps rotate mute", func(t *testing.T) {
		out := filepath.Join(dir, "trim.mp4")
		p, err := mediaconv.VideoPlan(clip, out, info, mediaconv.VideoOptions{Format: "mp4", Codec: "h264", TrimStart: "2", TrimEnd: "7", FPS: "15", Rotate: 90, AudioMode: "mute"}, enc, "")
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		got := probe(t, fp, out)
		if math.Abs(got.Duration-5) > 0.3 || got.HasAudio || got.Width != 720 || got.Height != 1280 || math.Round(got.FPS) != 15 {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("subtitles embed and burn", func(t *testing.T) {
		out := filepath.Join(dir, "subs.mp4")
		p, err := mediaconv.VideoPlan(clip, out, info, mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Subtitles: "embed", SubFile: srt}, enc, "")
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		if got := probe(t, fp, out); got.SubCodec != "mov_text" {
			t.Fatalf("embedded subtitle codec %q", got.SubCodec)
		}
		work := t.TempDir()
		burned := filepath.Join(dir, "burn.mp4")
		p, err = mediaconv.VideoPlan(clip, burned, info, mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Subtitles: "burn", SubFile: srt, TrimStart: "1"}, enc, work)
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		if got := probe(t, fp, burned); got.SubCodec != "" || math.Abs(got.Duration-11) > 0.3 {
			t.Fatalf("burned: %+v", got)
		}
	})

	t.Run("frames", func(t *testing.T) {
		fdir := t.TempDir()
		p, err := mediaconv.FramesPlan(clip, filepath.Join(fdir, "f_%04d.jpg"), info, mediaconv.VideoOptions{Resolution: "480"}, 3, "jpg")
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		files, _ := filepath.Glob(filepath.Join(fdir, "*.jpg"))
		if len(files) < 3 || len(files) > 5 {
			t.Fatalf("%d frames", len(files))
		}
	})

	t.Run("merge videos", func(t *testing.T) {
		out := filepath.Join(dir, "joined.mp4")
		p, err := mediaconv.MergeVideoPlan([]string{clip, quiet}, []ffmpeg.Info{info, probe(t, fp, quiet)}, out, mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Resolution: "720"}, enc)
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		got := probe(t, fp, out)
		if math.Abs(got.Duration-16) > 0.5 || got.Width != 1280 || !got.HasAudio {
			t.Fatalf("joined: %+v", got)
		}
	})

	t.Run("gpu if present", func(t *testing.T) {
		hw := ffmpeg.HWEncoders(ctx, ff, enc)
		t.Logf("working GPU encoders: %v", hw)
		for name := range hw {
			vendor := map[string]string{"nvenc": "nvenc", "qsv": "qsv", "amf": "amf"}[name[strings.LastIndex(name, "_")+1:]]
			codec := map[string]string{"h264": "h264", "hevc": "h265", "av1": "av1", "vp9": "vp9"}[name[:strings.Index(name, "_")]]
			format := "mp4"
			if codec == "vp9" {
				format = "mkv"
			}
			out := filepath.Join(dir, "gpu-"+name+"."+format)
			all := map[string]bool{name: true}
			p, err := mediaconv.VideoPlan(clip, out, info, mediaconv.VideoOptions{Format: format, Codec: codec, HW: vendor}, all, "")
			if err != nil {
				t.Fatal(err)
			}
			runPlan(t, ff, p)
			p, err = mediaconv.VideoPlan(clip, out, info, mediaconv.VideoOptions{Format: format, Codec: codec, HW: vendor, TargetMB: 1}, all, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			runPlan(t, ff, p)
		}
	})

	t.Run("audio effects tags silence merge", func(t *testing.T) {
		song := filepath.Join(dir, "song.wav")
		// 1 s silence, 6 s tone, 2 s silence.
		if _, err := proc.Output(ctx, ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i",
			"anullsrc=r=44100:cl=stereo:d=1[s1];sine=frequency=330:duration=6:sample_rate=44100,aformat=channel_layouts=stereo[t];anullsrc=r=44100:cl=stereo:d=2[s2];[s1][t][s2]concat=n=3:v=0:a=1",
			song); err != nil {
			t.Fatal(err)
		}
		sinfo := probe(t, fp, song)
		lead, trail, err := ffmpeg.Silence(ctx, ff, song, -50, sinfo.Duration)
		if err != nil || math.Abs(lead-1) > 0.1 || math.Abs(trail-7) > 0.1 {
			t.Fatalf("silence %v %v %v", lead, trail, err)
		}
		cover := filepath.Join(dir, "cover.png")
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		for i := range img.Pix {
			img.Pix[i] = 200
		}
		img.Set(1, 1, color.Black)
		f, _ := os.Create(cover)
		_ = png.Encode(f, img)
		f.Close()
		out := filepath.Join(dir, "song.mp3")
		o := mediaconv.AudioOptions{Format: "mp3", Bitrate: 192, NormVolume: true, Loudness: -16, FadeIn: 1, FadeOut: 1, Speed: 1.25, Pitch: -2, RemoveSilence: true}
		p, err := mediaconv.AudioPlan(song, out, sinfo, o, &mediaconv.Tags{Title: "Judul Lagu", Artist: "Artis", Year: "2026", Cover: cover}, lead, trail)
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		got := probe(t, fp, out)
		if math.Abs(got.Duration-6/1.25) > 0.3 || got.Tags["title"] != "Judul Lagu" || got.CoverIndex < 0 || got.SampleRate != 44100 {
			t.Fatalf("song: dur=%v tags=%v cover=%d rate=%d", got.Duration, got.Tags, got.CoverIndex, got.SampleRate)
		}
		copyOut := filepath.Join(dir, "song-copy.mp3")
		p, err = mediaconv.AudioPlan(out, copyOut, got, mediaconv.AudioOptions{Format: mediaconv.FormatOriginal, TrimEnd: "2"}, &mediaconv.Tags{Title: "Baru", RemoveCover: true}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		if g := probe(t, fp, copyOut); g.Tags["title"] != "Baru" || g.CoverIndex >= 0 || g.Duration > 2.2 {
			t.Fatalf("copy: %+v", g)
		}
		joined := filepath.Join(dir, "joined.m4a")
		p, err = mediaconv.MergeAudioPlan([]string{song, clip}, []ffmpeg.Info{sinfo, info}, joined, mediaconv.AudioOptions{Format: "m4a", Bitrate: 160})
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		if g := probe(t, fp, joined); math.Abs(g.Duration-21) > 0.5 {
			t.Fatalf("joined audio %v", g.Duration)
		}
	})
}

func TestYellowItemsWithFFmpeg(t *testing.T) {
	m := setup(t)
	ff, fp := m.Path(tools.FFmpeg), m.Path(tools.FFprobe)
	ctx := context.Background()
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.mp4")
	if _, err := proc.Output(ctx, ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=30:duration=10",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=10", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", clip); err != nil {
		t.Fatal(err)
	}
	info := probe(t, fp, clip)
	enc, _ := ffmpeg.Encoders(ctx, ff)
	out := filepath.Join(dir, "br.mp4")
	p, err := mediaconv.VideoPlan(clip, out, info, mediaconv.VideoOptions{Format: "mp4", Codec: "h264", BitrateK: 800, AudioMode: "aac", AudioBitrate: 96}, enc, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runPlan(t, ff, p)
	st, _ := os.Stat(out)
	kbps := float64(st.Size()*8) / 10 / 1000
	if kbps < 600 || kbps > 1200 {
		t.Fatalf("total bitrate %.0f kbps, want about 900", kbps)
	}
	for _, format := range []string{"mp3", "ogg", "opus"} {
		o := filepath.Join(dir, "vbr."+format)
		p, err := mediaconv.AudioPlan(clip, o, info, mediaconv.AudioOptions{Format: format, VBR: true, VBRLevel: "small"}, nil, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		if g := probe(t, fp, o); !g.HasAudio || math.Abs(g.Duration-10) > 0.5 {
			t.Fatalf("%s VBR: %+v", format, g)
		}
	}
	o := filepath.Join(dir, "64.mp3")
	p, err = mediaconv.AudioPlan(clip, o, info, mediaconv.AudioOptions{Format: "mp3", Bitrate: 64}, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	runPlan(t, ff, p)
	if st, _ := os.Stat(o); st.Size() > 100_000 {
		t.Fatalf("64 kbps file too big: %d", st.Size())
	}
}
