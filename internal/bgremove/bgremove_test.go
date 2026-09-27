package bgremove

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/imaging"
)

// TestCutoutWithRuntime needs onnxruntime.dll and the model files:
// KMB_ORT_DLL=…\onnxruntime.dll KMB_BG_MODELS=…\folder go test ./internal/bgremove
func TestCutoutWithRuntime(t *testing.T) {
	dll, dir := os.Getenv("KMB_ORT_DLL"), os.Getenv("KMB_BG_MODELS")
	if dll == "" || dir == "" {
		t.Skip("KMB_ORT_DLL / KMB_BG_MODELS not set")
	}
	// A bright red disc on a flat grey-blue background.
	img := imaging.New(640, 480, color.NRGBA{90, 110, 140, 255})
	for y := 0; y < 480; y++ {
		for x := 0; x < 640; x++ {
			if dx, dy := x-320, y-240; dx*dx+dy*dy < 150*150 {
				img.Set(x, y, color.NRGBA{230, 40, 30, 255})
			}
		}
	}
	r := New(dll, dir)
	for _, m := range Models {
		if _, err := os.Stat(filepath.Join(dir, m.File)); err != nil {
			continue
		}
		out, err := r.Cutout(img, m)
		if err != nil {
			t.Fatalf("%s: %v", m.ID, err)
		}
		centre, corner := out.NRGBAAt(320, 240).A, out.NRGBAAt(10, 10).A
		t.Logf("%s: centre alpha %d, corner alpha %d", m.ID, centre, corner)
		if centre < 200 || corner > 60 {
			t.Errorf("%s: subject/background not separated (centre %d, corner %d)", m.ID, centre, corner)
		}
		if p := os.Getenv("KMB_BG_OUT"); p != "" {
			f, _ := os.Create(filepath.Join(p, m.ID+".png"))
			_ = png.Encode(f, out)
			f.Close()
		}
		_ = image.Rect
	}
}
