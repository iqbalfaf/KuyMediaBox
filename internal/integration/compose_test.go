//go:build integration

package integration

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/imaging"

	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/mediaconv"
	"kuymediabox/internal/proc"
	"kuymediabox/internal/tools"
)

func TestSlideshowAndCollage(t *testing.T) {
	m := setup(t)
	ff, fp := m.Path(tools.FFmpeg), m.Path(tools.FFprobe)
	ctx := context.Background()
	dir := t.TempDir()
	// Four pictures of different sizes and colours.
	var pics []string
	for i, c := range []color.NRGBA{{220, 40, 40, 255}, {40, 180, 60, 255}, {40, 80, 220, 255}, {230, 200, 30, 255}} {
		p := filepath.Join(dir, fmt.Sprintf("p%d.png", i))
		if err := imaging.Save(imaging.New(400+i*100, 300+i*50, c), p); err != nil {
			t.Fatal(err)
		}
		pics = append(pics, p)
	}
	music := filepath.Join(dir, "m.mp3")
	if _, err := proc.Output(ctx, ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=500:duration=2", "-c:a", "libmp3lame", music); err != nil {
		t.Fatal(err)
	}
	frames := func(t *testing.T, o mediaconv.SlideOptions) []string {
		work := t.TempDir()
		var out []string
		var w, h int
		for i, p := range pics {
			img, err := imageconv.Decode(ctx, p, ff)
			if err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				w, h = imageconv.RatioSize(o.Ratio, img.Bounds(), o.Size)
			}
			f := filepath.Join(work, fmt.Sprintf("frame_%04d.png", i+1))
			if err := imaging.Save(imageconv.FitCanvas(img, w, h, o.Fit, color.Black), f); err != nil {
				t.Fatal(err)
			}
			out = append(out, f)
		}
		return out
	}
	for _, c := range []struct {
		name string
		o    mediaconv.SlideOptions
		dur  float64
		w, h int
	}{
		{"cut.gif", mediaconv.SlideOptions{Format: "gif", Seconds: 0.5, Size: 320}, 2, 320, 240},
		{"fade.gif", mediaconv.SlideOptions{Format: "gif", Seconds: 0.5, Size: 200, Ratio: "1:1", Fit: "blur", Fade: 0.2}, 2.2, 200, 200},
		{"anim.webp", mediaconv.SlideOptions{Format: "webp", Seconds: 0.5, Size: 300, Fit: "cover"}, 0, 300, 226},
		{"show.mp4", mediaconv.SlideOptions{Format: "mp4", Seconds: 1.5, Size: 640, Ratio: "16:9", Fade: 0.5, Music: music}, 6.5, 640, 360},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := filepath.Join(dir, c.name)
			p, err := mediaconv.SlideshowPlan(frames(t, c.o), out, c.o)
			if err != nil {
				t.Fatal(err)
			}
			runPlan(t, ff, p)
			if filepath.Ext(c.name) == ".webp" {
				// ffprobe can't always read animated WebP: check the file exists and is animated.
				data, _ := os.ReadFile(out)
				if len(data) < 100 || string(data[:4]) != "RIFF" || !containsBytes(data, "ANIM") {
					t.Fatalf("not an animated webp (%d bytes)", len(data))
				}
				return
			}
			got := probe(t, fp, out)
			if got.Width != c.w || got.Height != c.h {
				t.Errorf("size %dx%d, want %dx%d", got.Width, got.Height, c.w, c.h)
			}
			if c.dur > 0 && math.Abs(got.Duration-c.dur) > 0.3 {
				t.Errorf("duration %.2f, want %.2f", got.Duration, c.dur)
			}
			if c.o.Music != "" && !got.HasAudio {
				t.Error("music missing")
			}
		})
	}

	img, err := imageconv.Collage(ctx, append(pics, pics[0]), imageconv.CollageOptions{Cols: 3, Width: 900, Gap: 12, Cell: "1:1", Fit: "cover", Background: "#ffffff", Radius: 16}, ff, func(float64) {})
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	cell := (900 - 12*4) / 3
	if b.Dx() != 900 || b.Dy() != 2*cell+3*12 {
		t.Fatalf("collage %v", b)
	}
	// The second row has 2 of 3 cells and is centred: the far left of it is background.
	if c := img.NRGBAAt(20, 12+cell+12+cell/2); c.R != 255 || c.G != 255 || c.B != 255 {
		t.Errorf("last row not centred: %v", c)
	}
	// Rounded corner: the very corner of the first cell is background.
	if c := img.NRGBAAt(12, 12); c != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("corner %v", c)
	}
	if err := imageconv.EncodeFile(ctx, img, filepath.Join(dir, "collage.jpg"), imageconv.Options{Format: "jpg", Quality: 90}); err != nil {
		t.Fatal(err)
	}
	_ = image.Rect
}

func containsBytes(data []byte, s string) bool {
	for i := 0; i+len(s) <= len(data) && i < 4096; i++ {
		if string(data[i:i+len(s)]) == s {
			return true
		}
	}
	return false
}
