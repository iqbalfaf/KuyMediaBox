package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestHeaderFooterTextAndCSV(t *testing.T) {
	dir := t.TempDir()
	src := makePDF(t, dir, "doc.pdf", 3)
	hf := filepath.Join(dir, "hf.pdf")
	o := HeaderFooterOptions{TopLeft: "Laporan Keuangan", TopRight: "{date}", BottomCenter: "Halaman {n} dari {total}", Line: true, SkipFirst: true}
	if err := AddHeaderFooter(Input{Path: src}, o, hf); err != nil {
		t.Fatal(err)
	}
	txt := filepath.Join(dir, "out.txt")
	if err := PDFToText(context.Background(), Input{Path: hf}, false, txt, noProgress); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(txt)
	s := string(data)
	if !strings.Contains(s, "Halaman 2 dari 3") || !strings.Contains(s, "Laporan Keuangan") || strings.Contains(s, "Halaman 1 dari 3") {
		t.Fatalf("text: %q", s)
	}
	md := filepath.Join(dir, "out.md")
	if err := PDFToText(context.Background(), Input{Path: hf}, true, md, noProgress); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(md); !strings.Contains(string(data), "---") {
		t.Errorf("markdown pages: %q", data)
	}
	csvOut := filepath.Join(dir, "out.csv")
	if err := PDFToCSV(context.Background(), Input{Path: hf}, true, csvOut, noProgress); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(csvOut); !strings.HasPrefix(string(data), "\xEF\xBB\xBF") || !strings.Contains(string(data), "Laporan Keuangan") {
		t.Errorf("csv: %q", data)
	}
	// A PDF without text says so.
	if err := PDFToText(context.Background(), Input{Path: src}, false, txt, noProgress); err == nil {
		t.Error("image-only PDF must be refused")
	}
}

func TestNUpAndBooklet(t *testing.T) {
	dir := t.TempDir()
	src := makePDF(t, dir, "doc.pdf", 8)
	out := filepath.Join(dir, "nup.pdf")
	if err := NUp(Input{Path: src}, NUpOptions{Mode: "nup", N: 4, Paper: "A4", Border: true}, out); err != nil {
		t.Fatal(err)
	}
	if n := pageCount(t, out); n != 2 {
		t.Errorf("4-up of 8 pages: %d sheets", n)
	}
	if err := NUp(Input{Path: src}, NUpOptions{Mode: "booklet", N: 2, Paper: "F4"}, out); err != nil {
		t.Fatal(err)
	}
	if n := pageCount(t, out); n != 4 {
		t.Errorf("booklet of 8 pages: %d sides", n)
	}
}

func TestMetaAndBookmarks(t *testing.T) {
	dir := t.TempDir()
	src := makePDF(t, dir, "doc.pdf", 4)
	out := filepath.Join(dir, "meta.pdf")
	m := Meta{Title: "Laporan Tahunan — 2026", Author: "Budi Ŝantoso", Subject: "Keuangan", Keywords: "laporan, 2026"}
	bms := []Bookmark{{Title: "Bab 1", Page: 1, Kids: []Bookmark{{Title: "1.1 Pendahuluan", Page: 2}}}, {Title: "Bab 2", Page: 3}}
	if err := WriteDetails(Input{Path: src}, m, bms, out); err != nil {
		t.Fatal(err)
	}
	d, err := ReadDetails(Input{Path: out})
	if err != nil {
		t.Fatal(err)
	}
	if d.Meta.Title != m.Title || d.Meta.Author != m.Author || d.Meta.Keywords != m.Keywords || d.Pages != 4 {
		t.Errorf("meta %+v", d.Meta)
	}
	if len(d.Bookmarks) != 2 || d.Bookmarks[0].Kids[0].Title != "1.1 Pendahuluan" || d.Bookmarks[1].Page != 3 {
		t.Errorf("bookmarks %+v", d.Bookmarks)
	}
	// Removing all bookmarks, keeping the properties.
	out2 := filepath.Join(dir, "meta2.pdf")
	if err := WriteDetails(Input{Path: out}, d.Meta, []Bookmark{}, out2); err != nil {
		t.Fatal(err)
	}
	if d2, _ := ReadDetails(Input{Path: out2}); len(d2.Bookmarks) != 0 || d2.Meta.Title != m.Title {
		t.Errorf("after removing: %+v", d2)
	}
}

const formJSON = `{"paper":"A4P","origin":"UpperLeft","fonts":{"f":{"name":"Helvetica","size":12}},
"pages":{"1":{"content":{
 "textfield":[{"id":"nama","value":"","pos":[140,100],"width":200,"font":{"name":"$f"},"label":{"value":"Nama","width":60,"gap":5,"pos":"left","font":{"name":"$f"}}}],
 "checkbox":[{"id":"setuju","value":false,"pos":[140,150],"width":20,"font":{"name":"$f"},"label":{"value":"Setuju","width":60,"gap":5,"pos":"left","font":{"name":"$f"}}}],
 "combobox":[{"id":"kota","value":"Bandung","options":["Jakarta","Bandung","Surabaya"],"pos":[140,200],"width":120,"font":{"name":"$f"},"label":{"value":"Kota","width":60,"gap":5,"pos":"left","font":{"name":"$f"}}}]
}}}}`

// makeForm writes a one-page PDF form with a text field, a checkbox and a combo box.
func makeForm(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "form.pdf")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := api.Create(nil, strings.NewReader(formJSON), f, conf("")); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFormFillAndFlatten(t *testing.T) {
	src := makeForm(t, t.TempDir())
	fields, err := ReadForm(Input{Path: src})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, f := range fields {
		kinds[f.Kind]++
	}
	t.Logf("%d fields %v", len(fields), kinds)
	if kinds["text"] == 0 {
		t.Fatal("no text fields")
	}
	var textID string
	for i := range fields {
		f := &fields[i]
		switch f.Kind {
		case "text":
			if textID == "" && !f.Locked {
				textID = f.ID
				f.Value = "Siti Nurhaliza"
			}
		case "check":
			f.Checked = true
		case "combo":
			f.Value = "Surabaya"
		}
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "filled.pdf")
	if err := FillForm(context.Background(), Input{Path: src}, fields, false, out); err != nil {
		t.Fatal(err)
	}
	again, err := ReadForm(Input{Path: out})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range again {
		switch {
		case f.ID == textID && f.Value != "Siti Nurhaliza":
			t.Errorf("text field %q", f.Value)
		case f.Kind == "check" && !f.Checked:
			t.Error("checkbox not ticked")
		case f.Kind == "combo" && f.Value != "Surabaya":
			t.Errorf("combo %q", f.Value)
		}
	}
	flat := filepath.Join(dir, "flat.pdf")
	if err := FillForm(context.Background(), Input{Path: src}, fields, true, flat); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadForm(Input{Path: flat}); err == nil {
		if f2, _ := ReadForm(Input{Path: flat}); len(f2) > 0 {
			t.Errorf("flattened PDF still has %d fields", len(f2))
		}
	}
	txt := filepath.Join(dir, "flat.txt")
	if err := PDFToText(context.Background(), Input{Path: flat}, false, txt, noProgress); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(txt); !strings.Contains(string(data), "Siti Nurhaliza") || !strings.Contains(string(data), "Surabaya") {
		t.Errorf("flattened page doesn't show the filled values: %q", data)
	}
	if _, err := ReadForm(Input{Path: makePDF(t, dir, "plain.pdf", 1)}); err == nil {
		t.Error("a PDF without a form must be refused")
	}
}

// Non-Latin text must actually be drawn, not just be searchable: Cyrillic, shaped Arabic
// (right to left, joined) and Devanagari (conjuncts).
func TestNonLatinTextRenders(t *testing.T) {
	for _, text := range []string{"Привет мир", "السلام عليكم", "नमस्ते क्षत्रिय"} {
		dir := t.TempDir()
		src := makePDF(t, dir, "doc.pdf", 1)
		out := filepath.Join(dir, "hf.pdf")
		if err := AddHeaderFooter(Input{Path: src}, HeaderFooterOptions{TopCenter: text, Size: 24, Margin: 40, Color: "#000000"}, out); err != nil {
			t.Fatal(err)
		}
		d, err := Open(context.Background(), out, "")
		if err != nil {
			t.Fatal(err)
		}
		img, err := d.Render(0, 595, 842)
		d.Close()
		if err != nil {
			t.Fatal(err)
		}
		dark := 0
		for y := 0; y < 90; y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r+g+b < 3*0x6000 {
					dark++
				}
			}
		}
		if dark < 300 {
			t.Errorf("%q: only %d dark pixels — the text wasn't drawn", text, dark)
		}
	}
}
