package pdf

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// saveRender writes page i of pdfPath as PNG into $KMB_DEBUG_DIR (when set).
func saveRender(t *testing.T, pdfPath string, i int, name string) {
	dir := os.Getenv("KMB_DEBUG_DIR")
	if dir == "" {
		return
	}
	d, err := Open(context.Background(), pdfPath, "")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	img, err := d.RenderDPI(i, 60)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(filepath.Join(dir, name))
	defer f.Close()
	_ = png.Encode(f, img)
}
