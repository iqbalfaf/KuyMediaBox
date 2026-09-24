package imageconv

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writeTestPNG(t *testing.T, path string, w, h int, transparent bool) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := uint8(255)
			if transparent && x < w/2 {
				a = 0
			}
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 120, A: a})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestTargetSize(t *testing.T) {
	cases := []struct {
		w, h int
		o    Options
		want Size
	}{
		{4032, 3024, Options{ResizeMode: "longest", Longest: 1920}, Size{1920, 1440}},
		{3024, 4032, Options{ResizeMode: "longest", Longest: 1920}, Size{1440, 1920}},
		{512, 512, Options{ResizeMode: "longest", Longest: 1920}, Size{512, 512}},
		{1000, 500, Options{ResizeMode: "percent", Percent: 50}, Size{500, 250}},
		{1000, 500, Options{ResizeMode: "box", Width: 400, Height: 400}, Size{400, 200}},
		{2000, 1000, Options{Format: "ico"}, Size{256, 128}},
	}
	for _, c := range cases {
		if got := TargetSize(c.w, c.h, c.o); got != c.want {
			t.Errorf("%dx%d %+v: got %v want %v", c.w, c.h, c.o, got, c.want)
		}
	}
}

func TestConvertAllFormats(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.png")
	writeTestPNG(t, src, 300, 200, true)
	for _, format := range Formats {
		t.Run(format, func(t *testing.T) {
			out := filepath.Join(dir, "out."+format)
			o := Options{Format: format, Quality: 80, ResizeMode: "longest", Longest: 150, Background: "#ffffff", AutoRotate: true}
			if err := Convert(context.Background(), src, out, o, "", func(float64) {}); err != nil {
				t.Fatal(err)
			}
			st, err := os.Stat(out)
			if err != nil || st.Size() == 0 {
				t.Fatalf("no output: %v", err)
			}
			if format == "pdf" {
				data, _ := os.ReadFile(out)
				if !bytes.HasPrefix(data, []byte("%PDF-1.4")) || !bytes.Contains(data, []byte("%%EOF")) {
					t.Fatal("invalid pdf")
				}
				return
			}
			// Everything except pdf must decode back to the resized dimensions.
			var got image.Image
			if format == "ico" {
				data, _ := os.ReadFile(out)
				got, err = decodeICO(data)
			} else {
				got, err = decode(context.Background(), out, false, "")
			}
			if err != nil {
				t.Fatalf("round trip decode: %v", err)
			}
			if b := got.Bounds(); b.Dx() != 150 || b.Dy() != 100 {
				t.Fatalf("size %dx%d", b.Dx(), b.Dy())
			}
			if format == "jpg" {
				r, g, bb, _ := got.At(5, 50).RGBA()
				if r>>8 < 240 || g>>8 < 240 || bb>>8 < 240 {
					t.Fatalf("transparent area should become white, got %d %d %d", r>>8, g>>8, bb>>8)
				}
			}
		})
	}
}

func TestBadInput(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "rusak.jpg")
	os.WriteFile(bad, []byte("not an image"), 0o644)
	err := Convert(context.Background(), bad, filepath.Join(dir, "o.png"), Options{Format: "png"}, "", func(float64) {})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "o.png")); !os.IsNotExist(statErr) {
		t.Fatal("no output file should be left behind")
	}
}
