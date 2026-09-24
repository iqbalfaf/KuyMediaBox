package pdf

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOCR(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows OCR only")
	}
	if _, err := exec.LookPath("powershell.exe"); err != nil {
		t.Skip("no PowerShell")
	}
	dir := t.TempDir()
	bg := context.Background()
	langs, err := OCRLanguages(bg, dir)
	if err != nil || len(langs) == 0 {
		t.Skipf("no OCR languages: %v", err)
	}
	// A "scan": the text page as a picture only.
	src := makeTextPDF(t, dir, "t.pdf", 1)
	d, err := Open(bg, src, "")
	if err != nil {
		t.Fatal(err)
	}
	img, _ := d.RenderDPI(0, 150)
	d.Close()
	scan := filepath.Join(dir, "scan.pdf")
	if err := WriteImagePDF(scan, []ImagePage{{Img: img, PageW: 595, PageH: 842, Box: [4]float64{0, 0, 595, 842}}}, 90); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "ocr.pdf")
	n, err := OCR(bg, Input{Path: scan}, OCROptions{SkipText: true}, out, dir, noProgress)
	if err != nil || n != 1 {
		t.Fatalf("ocr: %d %v", n, err)
	}
	boxes, err := FindText(bg, out, "rekening", false)
	if err != nil || len(boxes) != 1 {
		t.Fatalf("searchable text not found: %v %v", boxes, err)
	}
	// Close to where the word is printed (x ≈ 111pt, y ≈ 120pt).
	if b := boxes[0]; b.X*595 < 90 || b.X*595 > 135 || b.Y*842 < 105 || b.Y*842 > 135 {
		t.Fatalf("word box %+v", b)
	}
	if _, err := OCR(bg, Input{Path: out}, OCROptions{SkipText: true}, filepath.Join(dir, "x.pdf"), dir, noProgress); err == nil || !strings.Contains(err.Error(), "teks") {
		t.Fatalf("second OCR should find text already: %v", err)
	}
}
