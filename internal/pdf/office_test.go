package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOOXMLAndOffice(t *testing.T) {
	dir := os.Getenv("KMB_DEBUG_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	bg := context.Background()
	src := makeTextPDF(t, t.TempDir(), "t.pdf", 2)
	docx := filepath.Join(dir, "out.docx")
	if err := PDFToDocx(bg, Input{Path: src}, docx, noProgress); err != nil {
		t.Fatal(err)
	}
	pptx := filepath.Join(dir, "out.pptx")
	if err := PDFToPptx(bg, Input{Path: src}, pptx, noProgress); err != nil {
		t.Fatal(err)
	}
	xlsx := filepath.Join(dir, "out.xlsx")
	if err := PDFToXlsx(bg, Input{Path: src}, xlsx, noProgress); err != nil {
		t.Fatal(err)
	}
	eng := DetectEngines("")
	if !eng.Word {
		t.Skip("Microsoft Word not installed")
	}
	tmp := t.TempDir()
	pdf := filepath.Join(dir, "roundtrip.pdf")
	used, err := OfficeToPDF(bg, docx, pdf, tmp, eng)
	if err != nil {
		t.Fatalf("docx → pdf: %v", err)
	}
	d, err := Open(bg, pdf, "")
	if err != nil {
		t.Fatal(err)
	}
	txt, _ := d.PageText(0)
	d.Close()
	if used != "Microsoft Office" || !strings.Contains(txt, "Nomor rekening") {
		t.Fatalf("round trip (%s): %q", used, txt)
	}
	if eng.Excel {
		if _, err := OfficeToPDF(bg, xlsx, filepath.Join(dir, "xlsx.pdf"), tmp, eng); err != nil {
			t.Fatalf("xlsx → pdf: %v", err)
		}
	}
	if eng.PowerPoint {
		if _, err := OfficeToPDF(bg, pptx, filepath.Join(dir, "pptx.pdf"), tmp, eng); err != nil {
			t.Fatalf("pptx → pdf: %v", err)
		}
	}
	w := filepath.Join(dir, "word.docx")
	if used, err := PDFToWord(bg, Input{Path: src}, w, tmp, eng, noProgress); err != nil || used != "Microsoft Word" {
		t.Fatalf("pdf → word via %s: %v", used, err)
	}
}
