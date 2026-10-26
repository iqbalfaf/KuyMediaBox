package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"kuymediabox/internal/rename"
)

// writeDocx makes a minimal Word file whose only paragraph is text.
func writeDocx(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("word/document.xml")
	_, _ = w.Write([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:body></w:document>`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestRenamePreview(t *testing.T) {
	a := testApp(t)
	dir := t.TempDir()
	doc, img := filepath.Join(dir, "rapor_001.docx"), filepath.Join(dir, "IMG_01.JPG")
	writeDocx(t, doc, "Nama Peserta Didik : BUDI SANTOSO")
	if err := os.WriteFile(img, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	items := []RenameItem{{ID: "d", Path: doc}, {ID: "i", Path: img}}

	plans := a.RenamePreview(items, RenameRequest{Options: rename.Options{Mode: rename.ModeContent, Case: "title"}})
	if plans[0].Name != "Budi Santoso" || plans[0].Ext != ".docx" || plans[0].Error != "" {
		t.Errorf("document: %+v", plans[0])
	}
	if plans[1].Name != "" || plans[1].Error == "" {
		t.Errorf("a picture has no text to read: %+v", plans[1])
	}

	plans = a.RenamePreview(items, RenameRequest{Options: rename.Options{Mode: rename.ModePattern, Pattern: "Berkas {n}", Start: 1, Digits: 2}, ExtLower: true})
	if plans[0].Name != "Berkas 01" || plans[1].Name != "Berkas 02" || plans[1].Ext != ".jpg" {
		t.Errorf("pattern: %+v", plans)
	}

	// The edited document is read again (the text cache follows the file's time and size).
	writeDocx(t, doc, "Nama Peserta Didik : SITI AMINAH WIJAYA")
	plans = a.RenamePreview(items[:1], RenameRequest{Options: rename.Options{Mode: rename.ModeContent}})
	if plans[0].Name != "SITI AMINAH WIJAYA" {
		t.Errorf("after edit: %+v", plans[0])
	}

	res, err := a.RenameApply([]rename.Job{{ID: "d", Path: doc, Name: plans[0].Name}}, "")
	if err != nil || res[0].To != filepath.Join(dir, "SITI AMINAH WIJAYA.docx") {
		t.Fatalf("apply: %+v, %v", res, err)
	}
	if back := a.RenameUndo(res); back[0].To != doc {
		t.Fatalf("undo: %+v", back)
	}
}
