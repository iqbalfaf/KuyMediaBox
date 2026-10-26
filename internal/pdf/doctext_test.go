package pdf

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocLinesWord(t *testing.T) {
	dir := t.TempDir()
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:pPr><w:tabs><w:tab w:val="left" w:pos="2000"/></w:tabs></w:pPr><w:r><w:t>RAPOR SEMESTER</w:t></w:r></w:p>
<w:tbl>
<w:tr><w:tc><w:p><w:r><w:t>Nama Peserta</w:t></w:r><w:r><w:t xml:space="preserve"> Didik</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>:</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>BUDI  SANTOSO</w:t></w:r></w:p></w:tc></w:tr>
<w:tr><w:tc><w:p><w:r><w:t>NISN</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>:</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>0012345678</w:t></w:r></w:p></w:tc></w:tr>
</w:tbl>
<w:p><w:r><w:t>Kelas</w:t></w:r><w:r><w:tab/><w:t>: X-1</w:t></w:r></w:p>
</w:body></w:document>`
	path := filepath.Join(dir, "rapor.docx")
	if err := writeZip(path, []zipFile{{"word/document.xml", []byte(doc)}}); err != nil {
		t.Fatal(err)
	}
	lines, err := DocLines(context.Background(), Input{Path: path}, dir, Engines{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"RAPOR SEMESTER", "Nama Peserta Didik\t:\tBUDI SANTOSO\t", "NISN\t:\t0012345678\t", "Kelas\t: X-1"}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines = %q\nwant    %q", lines, want)
	}

	odt := `<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0"><office:body><office:text>
<text:p>Rapor<text:s/>Akhir</text:p>
<table:table><table:table-row><table:table-cell><text:p>Nama Siswa</text:p></table:table-cell><table:table-cell><text:p>: <text:span>SITI</text:span> AMINAH</text:p></table:table-cell></table:table-row></table:table>
</office:text></office:body></office:document-content>`
	path = filepath.Join(dir, "rapor.odt")
	if err := writeZip(path, []zipFile{{"content.xml", []byte(odt)}}); err != nil {
		t.Fatal(err)
	}
	lines, err = DocLines(context.Background(), Input{Path: path}, dir, Engines{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "Rapor Akhir\nNama Siswa\t: SITI AMINAH\t"; strings.Join(lines, "\n") != want {
		t.Errorf("odt lines = %q", lines)
	}
}

func TestDocLinesPDF(t *testing.T) {
	dir := t.TempDir()
	blank := makePDF(t, dir, "blank.pdf", 1)
	src := filepath.Join(dir, "rapor.pdf")
	items := map[int][]Item{1: {
		{Kind: "text", Text: "Nama Sekolah", X: 60, Y: 100, Size: 11},
		{Kind: "text", Text: ": SMA NEGERI 1", X: 200, Y: 100, Size: 11},
		{Kind: "text", Text: "Nama Peserta Didik", X: 60, Y: 120, Size: 11},
		{Kind: "text", Text: ": SITI AMINAH", X: 200, Y: 120, Size: 11},
		{Kind: "text", Text: "Kelas", X: 380, Y: 120, Size: 11},
		{Kind: "text", Text: ": XI IPS 2", X: 440, Y: 120, Size: 11},
	}}
	if err := DrawItems(Input{Path: blank}, items, src); err != nil {
		t.Fatal(err)
	}
	lines, err := DocLines(context.Background(), Input{Path: src}, dir, Engines{})
	if err != nil {
		t.Fatal(err)
	}
	// Columns far apart are split by tabs, words of one column by spaces.
	want := []string{"Nama Sekolah\t: SMA NEGERI 1", "Nama Peserta Didik\t: SITI AMINAH\tKelas\t: XI IPS 2"}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines = %q", lines)
	}
	// An image-only PDF says it needs OCR.
	if _, err := DocLines(context.Background(), Input{Path: blank}, dir, Engines{}); err == nil || !strings.Contains(err.Error(), "OCR") {
		t.Errorf("scan without text: %v", err)
	}
	// Old Word formats need Office or LibreOffice.
	if _, err := DocLines(context.Background(), Input{Path: filepath.Join(dir, "x.doc")}, dir, Engines{}); err == nil {
		t.Error(".doc without Office must fail")
	}
}
