//go:build integration

package integration

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/mediaconv"
	"kuymediabox/internal/proc"
	"kuymediabox/internal/tools"
)

// TestVideoEffectsWithFFmpeg runs speed, reverse, crop, frames, stabilisation, watermark,
// music and noise reduction with the real ffmpeg.
func TestVideoEffectsWithFFmpeg(t *testing.T) {
	m := setup(t)
	ff, fp := m.Path(tools.FFmpeg), m.Path(tools.FFprobe)
	ctx := context.Background()
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.mp4")
	music := filepath.Join(dir, "music.mp3")
	if _, err := proc.Output(ctx, ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=30:duration=12",
		"-f", "lavfi", "-i", "anoisesrc=d=12:c=pink:a=0.05", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", clip); err != nil {
		t.Fatal(err)
	}
	if _, err := proc.Output(ctx, ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=330:duration=3", "-c:a", "libmp3lame", music); err != nil {
		t.Fatal(err)
	}
	info := probe(t, fp, clip)
	enc, err := ffmpeg.Encoders(ctx, ff)
	if err != nil {
		t.Fatal(err)
	}
	run := func(t *testing.T, name string, o mediaconv.VideoOptions) ffmpeg.Info {
		t.Helper()
		work := t.TempDir()
		o.Normalize()
		if o.Watermark.Enabled {
			w, h := mediaconv.OutputSize(info.Width, info.Height, o)
			o.WatermarkFile = filepath.Join(work, "wm.png")
			if err := mediaconv.WatermarkLayer(o.Watermark, w, h, o.WatermarkFile, ff); err != nil {
				t.Fatal(err)
			}
		}
		out := filepath.Join(dir, name)
		p, err := mediaconv.VideoPlan(clip, out, info, o, enc, work)
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		got := probe(t, fp, out)
		if w, h := mediaconv.OutputSize(info.Width, info.Height, o); o.Codec != "copy" && (got.Width != w || got.Height != h) {
			t.Errorf("size %dx%d, OutputSize says %dx%d", got.Width, got.Height, w, h)
		}
		return got
	}
	near := func(t *testing.T, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 0.35 {
			t.Errorf("duration %.2f, want %.2f", got, want)
		}
	}

	t.Run("speed 2x keeps fps and sound", func(t *testing.T) {
		got := run(t, "fast.mp4", mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Resolution: "480", Speed: 2})
		near(t, got.Duration, 6)
		if !got.HasAudio || math.Round(got.FPS) != 30 {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("slow motion 0.5x", func(t *testing.T) {
		got := run(t, "slow.mp4", mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Resolution: "480", Speed: 0.5, TrimEnd: "3"})
		near(t, got.Duration, 6)
	})
	t.Run("reverse short clip", func(t *testing.T) {
		got := run(t, "rev.mp4", mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Resolution: "480", Reverse: true, TrimStart: "1", TrimEnd: "4"})
		near(t, got.Duration, 3)
	})
	t.Run("reverse refuses long clips", func(t *testing.T) {
		long := info
		long.Duration = 3600
		if _, err := mediaconv.VideoPlan(clip, "x.mp4", long, mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Reverse: true}, enc, t.TempDir()); err == nil {
			t.Fatal("an hour-long reverse must be refused")
		}
	})
	t.Run("vertical 9:16 with blurred background", func(t *testing.T) {
		got := run(t, "vertical.mp4", mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Frame: "9:16", FrameFit: "blur", TrimEnd: "2"})
		if got.Width != 720 || got.Height != 1280 {
			t.Errorf("9:16 = %dx%d", got.Width, got.Height)
		}
	})
	t.Run("square with bars at 480", func(t *testing.T) {
		got := run(t, "square.mp4", mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Frame: "1:1", FrameFit: "pad", Resolution: "480", TrimEnd: "2"})
		if got.Width != 480 || got.Height != 480 {
			t.Errorf("1:1 = %dx%d", got.Width, got.Height)
		}
	})
	t.Run("4:5 crop and manual crop", func(t *testing.T) {
		got := run(t, "portrait.mp4", mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Frame: "4:5", FrameFit: "crop", TrimEnd: "2"})
		if got.Width != 576 || got.Height != 720 {
			t.Errorf("4:5 = %dx%d", got.Width, got.Height)
		}
		got = run(t, "cropped.mp4", mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Crop: mediaconv.CropBox{X: 0.25, Y: 0.1, W: 0.5, H: 0.5}, TrimEnd: "2"})
		if got.Width != 640 || got.Height != 360 {
			t.Errorf("crop = %dx%d", got.Width, got.Height)
		}
	})
	t.Run("stabilise", func(t *testing.T) {
		got := run(t, "stable.mp4", mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Resolution: "480", Stabilize: true, TrimEnd: "3"})
		near(t, got.Duration, 3)
	})
	t.Run("watermark text tiled and logo", func(t *testing.T) {
		wm := imageconv.Watermark{Enabled: true, Type: "text", Text: "© KuyMediaBox", Size: 6, Opacity: 0.6, Color: "#ffffff", Position: "tile", Angle: 30}
		got := run(t, "wm.mp4", mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Resolution: "720", Watermark: wm, Frame: "9:16", TrimEnd: "2"})
		near(t, got.Duration, 2)
		logo := filepath.Join(dir, "logo.png")
		if err := mediaconv.WatermarkLayer(imageconv.Watermark{Enabled: true, Type: "text", Text: "LOGO", Color: "#ff0000"}, 200, 80, logo, ff); err != nil {
			t.Fatal(err)
		}
		run(t, "wm-logo.webm", mediaconv.VideoOptions{Format: "webm", Codec: "vp9", Resolution: "480", Watermark: imageconv.Watermark{Enabled: true, Type: "image", Image: logo, Size: 20, Position: "br"}, TrimEnd: "1"})
	})
	t.Run("music mix, ducking, loop", func(t *testing.T) {
		got := run(t, "music.mp4", mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Resolution: "480",
			Music: mediaconv.Music{File: music, Mode: "mix", Volume: 0.8, Original: 1, Duck: true, Loop: true}})
		near(t, got.Duration, 12)
		if !got.HasAudio {
			t.Error("no sound")
		}
	})
	t.Run("music replaces sound, video copied", func(t *testing.T) {
		got := run(t, "replace.mp4", mediaconv.VideoOptions{Format: "mp4", Codec: "copy", Music: mediaconv.Music{File: music, Mode: "replace", Volume: 1}})
		near(t, got.Duration, 12)
		if !got.HasAudio || got.Width != 1280 {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("denoise and speed on the sound", func(t *testing.T) {
		got := run(t, "clean.mkv", mediaconv.VideoOptions{Format: "mkv", Codec: "h264", Resolution: "480", Denoise: "strong", Speed: 1.5, TrimEnd: "6"})
		near(t, got.Duration, 4)
	})
	t.Run("gif with frame, speed and watermark", func(t *testing.T) {
		wm := imageconv.Watermark{Enabled: true, Type: "text", Text: "GIF", Size: 10, Opacity: 1, Color: "#ffff00", Position: "tl"}
		got := run(t, "fx.gif", mediaconv.VideoOptions{Format: "gif", Resolution: "480", Frame: "1:1", FrameFit: "blur", Speed: 2, Watermark: wm, TrimEnd: "4"})
		if got.Width != 480 || got.Height != 480 {
			t.Errorf("gif %dx%d", got.Width, got.Height)
		}
	})
	t.Run("merge with watermark and denoise", func(t *testing.T) {
		o := mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Resolution: "480", Denoise: "light",
			Watermark: imageconv.Watermark{Enabled: true, Type: "text", Text: "JOIN", Color: "#ffffff"}}
		o.Normalize()
		work := t.TempDir()
		w, h := mediaconv.OutputSize(info.Width, info.Height, o)
		o.WatermarkFile = filepath.Join(work, "wm.png")
		if err := mediaconv.WatermarkLayer(o.Watermark, w, h, o.WatermarkFile, ff); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, "joined.mp4")
		p, err := mediaconv.MergeVideoPlan([]string{clip, clip}, []ffmpeg.Info{info, info}, out, o, enc)
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		near(t, probe(t, fp, out).Duration, 24)
	})
	t.Run("audio page denoise", func(t *testing.T) {
		out := filepath.Join(dir, "clean.mp3")
		p, err := mediaconv.AudioPlan(clip, out, info, mediaconv.AudioOptions{Format: "mp3", Denoise: "medium", NormVolume: true}, nil, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		if st, err := os.Stat(out); err != nil || st.Size() < 1000 {
			t.Fatalf("no result: %v", err)
		}
	})
}
