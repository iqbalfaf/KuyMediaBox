package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testdata/chrome-table.pdf is a page printed by Chrome: a heading, a paragraph and a
// three-column table. Its glyph boxes are tight, so punctuation only touches the bottom of
// the line; it must still stay in its word ("1." and "1.250,50", not "1 ." or a line of its own).
func TestChromeTextAndTable(t *testing.T) {
	src := filepath.Join("testdata", "chrome-table.pdf")
	dir := t.TempDir()
	read := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	md := filepath.Join(dir, "out.md")
	if err := PDFToText(context.Background(), Input{Path: src}, true, md, noProgress); err != nil {
		t.Fatal(err)
	}
	got := read(md)
	t.Log(got)
	for _, want := range []string{"# Laporan Nilai", "halaman 1. Nilai", "| Nama | Kota | Nilai |", "| Andi | Jakarta | 1.250,50 |", "| Budi | Surabaya | 1.100 |", "Selesai."} {
		if !strings.Contains(got, want) {
			t.Errorf("markdown is missing %q", want)
		}
	}

	csvPath := filepath.Join(dir, "out.csv")
	if err := PDFToCSV(context.Background(), Input{Path: src}, true, csvPath, noProgress); err != nil {
		t.Fatal(err)
	}
	got = strings.TrimPrefix(read(csvPath), string(rune(0xFEFF)))
	t.Log(got)
	want := "Nama;Kota;Nilai\nAndi;Jakarta;1.250,50\nSari;Bandung;980\nBudi;Surabaya;1.100\n"
	if strings.ReplaceAll(got, "\r\n", "\n") != want {
		t.Errorf("csv = %q, want %q", got, want)
	}
}
