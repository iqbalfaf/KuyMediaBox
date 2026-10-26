package pdf

import (
	"context"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// Every per-file tool takes a password protected PDF (with its password) and writes a
// result that opens without one.
func TestToolsOnLockedPDF(t *testing.T) {
	bg := context.Background()
	dir := t.TempDir()
	src := makeTextPDF(t, dir, "src.pdf", 3)
	locked := filepath.Join(dir, "locked.pdf")
	if err := Protect(Input{Path: src}, ProtectOptions{Password: "rahasia", AllowPrint: true}, locked, dir); err != nil {
		t.Fatal(err)
	}
	in := Input{Path: locked, Password: "rahasia"}
	opens := func(name, out string, pages int, text string) {
		t.Helper()
		d, err := Open(bg, out, "")
		if err != nil {
			t.Errorf("%s: the result needs a password or is broken: %v", name, err)
			return
		}
		defer d.Close()
		if pages > 0 && d.Pages() != pages {
			t.Errorf("%s: %d pages, want %d", name, d.Pages(), pages)
		}
		if text != "" {
			var all string
			for i := 0; i < d.Pages(); i++ {
				s, _ := d.PageText(i)
				all += s
			}
			if !strings.Contains(all, text) {
				t.Errorf("%s: text %q is missing", name, text)
			}
		}
	}
	run := func(name string, pages int, text string, f func(out string) error) {
		t.Helper()
		out := filepath.Join(dir, name+".pdf")
		if err := f(out); err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		opens(name, out, pages, text)
	}
	run("repair", 3, "Budi", func(out string) error { return Repair(bg, in, out, dir) })
	run("rotate", 3, "Budi", func(out string) error { return Rotate(in, 90, "", out) })
	run("watermark", 3, "Budi", func(out string) error {
		return Watermark(in, WatermarkOptions{Type: "text", Text: "RAHASIA", Size: 40, Opacity: 0.3, Position: "mc", Color: "#ff0000"}, out)
	})
	run("header", 3, "Halaman 2", func(out string) error {
		return AddHeaderFooter(in, HeaderFooterOptions{BottomCenter: "Halaman {n}"}, out)
	})
	run("nup", 2, "Budi", func(out string) error { return NUp(in, NUpOptions{Mode: "nup", N: 2, Paper: "A4"}, out) })
	run("booklet", 0, "", func(out string) error { return NUp(in, NUpOptions{Mode: "booklet", N: 2, Paper: "A4"}, out) })
	run("flatten", 3, "Budi", func(out string) error { return Flatten(bg, in, out) })
	run("numbers", 3, "Budi", func(out string) error { return AddPageNumbers(in, NumberOptions{}, out) })
	run("crop", 3, "Budi", func(out string) error { return Crop(in, CropOptions{Mode: "margins", Top: 5}, out) })
	run("remove", 2, "Budi", func(out string) error { return RemovePages(in, "2", out) })
	run("extract", 2, "Budi", func(out string) error { return ExtractPages(in, "1,3", out) })
	run("organize", 4, "Budi", func(out string) error {
		return Organize(bg, []Input{in}, []PageRef{{Src: 0, Page: 3}, {Src: -1, W: 595, H: 842}, {Src: 0, Page: 1, Rotate: 90}, {Src: 0, Page: 1}}, out, dir, noProgress)
	})
	run("merge", 6, "Budi", func(out string) error { return Merge(bg, []Input{in, in}, out, dir, noProgress) })
	run("meta", 3, "Budi", func(out string) error {
		return WriteDetails(in, Meta{Title: "Judul"}, []Bookmark{{Title: "Bab 1", Page: 2}}, out)
	})
	run("redact", 3, "", func(out string) error {
		return Redact(bg, in, RedactOptions{Boxes: []Rect{{Page: 1, X: 0.1, Y: 0.1, W: 0.3, H: 0.1}}}, out, noProgress)
	})
	signer, _ := caSigner(t)
	run("digisign", 3, "Budi", func(out string) error {
		return SignDigital(in, DigitalSignOptions{Visible: true, Position: "br", Page: "last"}, signer, out)
	})
	for _, mode := range []string{"pages", "extract"} {
		files, err := ToImages(bg, in, ImageExportOptions{Mode: mode, DPI: 72}, filepath.Join(dir, "img-"+mode), "x", noProgress)
		if err != nil || len(files) == 0 {
			t.Errorf("images %s: %v, %v", mode, files, err)
		}
	}
	if d, err := ReadDetails(in); err != nil || d.Pages != 3 {
		t.Errorf("details: %+v, %v", d, err)
	}

	// A protected form can be read and filled.
	lockedForm := filepath.Join(dir, "lockedform.pdf")
	if err := Protect(Input{Path: makeForm(t, dir)}, ProtectOptions{Password: "rahasia", AllowPrint: true}, lockedForm, dir); err != nil {
		t.Fatal(err)
	}
	fin := Input{Path: lockedForm, Password: "rahasia"}
	fields, err := ReadForm(fin)
	if err != nil || len(fields) == 0 {
		t.Fatalf("read the locked form: %v, %v", fields, err)
	}
	for _, flat := range []bool{false, true} {
		out := filepath.Join(dir, fmt.Sprint("filled-", flat, ".pdf"))
		if err := FillForm(bg, fin, fields, flat, out); err != nil {
			t.Errorf("fill the locked form (flatten %v): %v", flat, err)
		} else {
			opens(fmt.Sprint("fill ", flat), out, 1, "")
		}
	}
}

// formHolds reports which objects of a PDF contain s (in strings, streams or field values).
func formHolds(t *testing.T, path, s string) []int {
	t.Helper()
	rc, err := readCtx(path, "")
	if err != nil {
		t.Fatal(err)
	}
	var hits []int
	for nr, e := range rc.XRefTable.Table {
		if e == nil || e.Free || e.Object == nil {
			continue
		}
		text := e.Object.String()
		if sd, ok := e.Object.(types.StreamDict); ok && sd.Decode() == nil {
			text += string(sd.Content)
		}
		if d, ok := e.Object.(types.Dict); ok {
			if sl, ok := d["V"].(types.StringLiteral); ok {
				v, _ := types.StringLiteralToString(sl)
				text += v
			}
		}
		if strings.Contains(text, s) {
			hits = append(hits, nr)
		}
	}
	return hits
}

// A redacted page takes its form fields along: their values must not stay in the file.
// Fields on other pages stay.
func TestRedactRemovesFormValues(t *testing.T) {
	bg := context.Background()
	dir := t.TempDir()
	form := makeForm(t, dir)
	fields, err := ReadForm(Input{Path: form})
	if err != nil {
		t.Fatal(err)
	}
	for i := range fields {
		if fields[i].Kind == "text" {
			fields[i].Value = "NIK3201999"
		}
	}
	filled := filepath.Join(dir, "filled.pdf")
	if err := FillForm(bg, Input{Path: form}, fields, false, filled); err != nil {
		t.Fatal(err)
	}
	if len(formHolds(t, filled, "NIK3201999")) == 0 {
		t.Fatal("fixture should hold the value")
	}
	out := filepath.Join(dir, "red.pdf")
	if err := Redact(bg, Input{Path: filled}, RedactOptions{Boxes: []Rect{{Page: 1, W: 1, H: 1}}}, out, noProgress); err != nil {
		t.Fatal(err)
	}
	if h := formHolds(t, out, "NIK3201999"); len(h) > 0 {
		t.Errorf("the redacted file still holds the value in objects %v", h)
	}

	both := filepath.Join(dir, "both.pdf")
	if err := Merge(bg, []Input{{Path: filled}, {Path: makeTextPDF(t, dir, "t.pdf", 1)}}, both, dir, noProgress); err != nil {
		t.Fatal(err)
	}
	out = filepath.Join(dir, "red2.pdf")
	if err := Redact(bg, Input{Path: both}, RedactOptions{Boxes: []Rect{{Page: 2, W: 0.5, H: 0.5}}}, out, noProgress); err != nil {
		t.Fatal(err)
	}
	after, err := ReadForm(Input{Path: out})
	if err != nil || len(after) != len(fields) {
		t.Errorf("fields on other pages must stay: %d of %d, %v", len(after), len(fields), err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}

func TestParseRanges(t *testing.T) {
	for in, want := range map[string]string{
		"1-3, 4-6": "[[1 3] [4 6]]",
		"1-3 4-6":  "[[1 3] [4 6]]",
		"1 - 3; 9": "[[1 3] [9 9]]",
		"8-":       "[[8 10]]",
		"-2,akhir": "[[1 2] [10 10]]",
		" 2 , 5 ":  "[[2 2] [5 5]]",
		"3–5\n7":   "[[3 5] [7 7]]",
		"":         "[]",
	} {
		got, err := ParseRanges(in, 10)
		if err != nil || fmt.Sprint(got) != want {
			t.Errorf("ParseRanges(%q) = %v, %v; want %s", in, got, err, want)
		}
	}
	if _, err := ParseRanges("1 3x", 10); err == nil {
		t.Error("garbage must fail")
	}
	if _, err := ParseRanges("12", 10); err == nil {
		t.Error("a page outside the document must fail")
	}
}

func TestImagesToPDF(t *testing.T) {
	dir := t.TempDir()
	var files []string
	for i, size := range [][2]int{{400, 300}, {300, 500}, {200, 200}} {
		p := filepath.Join(dir, fmt.Sprintf("foto%d.png", i))
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, solid(size[0], size[1], color.RGBA{200, uint8(60 * i), 40, 255})); err != nil {
			t.Fatal(err)
		}
		f.Close()
		files = append(files, p)
	}
	out := filepath.Join(dir, "foto.pdf")
	if err := ImagesToPDF(context.Background(), files, out, ImagesOptions{PageSize: "a4", Orientation: "auto", Margin: "small"}, "", noProgress); err != nil {
		t.Fatal(err)
	}
	if n := pageCount(t, out); n != 3 {
		t.Fatalf("pages = %d", n)
	}
	// A broken picture names the file and leaves no half-written PDF behind.
	bad := filepath.Join(dir, "rusak.png")
	if err := os.WriteFile(bad, []byte("bukan gambar"), 0o644); err != nil {
		t.Fatal(err)
	}
	out2 := filepath.Join(dir, "rusak.pdf")
	err := ImagesToPDF(context.Background(), append(files, bad), out2, ImagesOptions{}, "", noProgress)
	if err == nil || !strings.Contains(err.Error(), "rusak.png") {
		t.Errorf("broken picture: %v", err)
	}
	if _, err := os.Stat(out2); err == nil {
		t.Error("a failed PDF must be removed")
	}
}
