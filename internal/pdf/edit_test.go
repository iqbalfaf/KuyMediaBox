package pdf

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// makeTextPDF writes n pages with a few lines of real text.
func makeTextPDF(t *testing.T, dir, name string, n int) string {
	t.Helper()
	blank := makePDF(t, dir, "blank-"+name, n)
	items := map[int][]Item{}
	for p := 1; p <= n; p++ {
		items[p] = []Item{
			{Kind: "text", Text: "Laporan Keuangan", X: 72, Y: 90, Size: 20, Bold: true},
			{Kind: "text", Text: "Nomor rekening 1234-5678 milik Budi " + string(rune('A'+p-1)), X: 72, Y: 130, Size: 12},
			{Kind: "text", Text: "Halaman isi nomor " + string(rune('0'+p)), X: 72, Y: 160, Size: 12},
		}
	}
	out := filepath.Join(dir, name)
	if err := DrawItems(Input{Path: blank}, items, out); err != nil {
		t.Fatal(err)
	}
	return out
}

// streamsContain reports whether any decoded stream or string of the file contains s.
func streamsContain(t *testing.T, path, s string) bool {
	t.Helper()
	ctx, err := readCtx(path, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ctx.XRefTable.Table {
		if e == nil || e.Free || e.Object == nil {
			continue
		}
		if sd, ok := e.Object.(types.StreamDict); ok {
			if err := sd.Decode(); err == nil && strings.Contains(string(sd.Content), s) {
				return true
			}
		}
	}
	return false
}

func TestFindAndRedact(t *testing.T) {
	dir := t.TempDir()
	src := makeTextPDF(t, dir, "t.pdf", 2)
	bg := context.Background()
	if !streamsContain(t, src, "1234-5678") {
		t.Fatal("fixture should contain the account number")
	}
	boxes, err := FindText(bg, src, "REKENING 1234", false)
	if err != nil || len(boxes) != 2 {
		t.Fatalf("find: %v %v", boxes, err)
	}
	b := boxes[0]
	if b.Page != 1 || math.Abs(b.X*595-111) > 8 || math.Abs(b.Y*842-125) > 15 {
		t.Fatalf("box %+v", b)
	}
	out := filepath.Join(dir, "redacted.pdf")
	if err := Redact(bg, Input{Path: src}, RedactOptions{Boxes: boxes[:1]}, out, noProgress); err != nil {
		t.Fatal(err)
	}
	if streamsContain(t, out, "milik Budi A") || !streamsContain(t, out, "milik Budi B") {
		t.Fatal("redacted text still inside the file")
	}
	d, err := Open(bg, out, "")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if txt, _ := d.PageText(0); strings.Contains(txt, "1234") {
		t.Fatalf("page 1 still has text %q", txt)
	}
	if txt, _ := d.PageText(1); !strings.Contains(txt, "1234-5678") {
		t.Fatalf("page 2 must keep its text, got %q", txt)
	}
	saveRender(t, out, 0, "redacted.png")
}

func TestCropAndImages(t *testing.T) {
	dir := t.TempDir()
	src := makeTextPDF(t, dir, "t.pdf", 3)
	bg := context.Background()

	cropped := filepath.Join(dir, "cropped.pdf")
	if err := Crop(Input{Path: src}, CropOptions{Mode: "margins", Top: 10, Bottom: 10, Left: 20, Right: 20}, cropped); err != nil {
		t.Fatal(err)
	}
	info, _ := Inspect(bg, cropped)
	mm := 72 / 25.4
	if math.Abs(info.Width-(595-40*mm)) > 1 || math.Abs(info.Height-(842-20*mm)) > 1 {
		t.Fatalf("cropped size %.1fx%.1f", info.Width, info.Height)
	}
	boxed := filepath.Join(dir, "boxed.pdf")
	if err := Crop(Input{Path: src}, CropOptions{Mode: "box", Box: Rect{X: 0.1, Y: 0.1, W: 0.5, H: 0.25}, Pages: "2"}, boxed); err != nil {
		t.Fatal(err)
	}
	d, _ := Open(bg, boxed, "")
	w1, _, _ := d.Size(0)
	w2, h2, _ := d.Size(1)
	d.Close()
	if math.Abs(w1-595) > 1 || math.Abs(w2-297.5) > 1 || math.Abs(h2-210.5) > 1 {
		t.Fatalf("box crop sizes %.1f / %.1fx%.1f", w1, w2, h2)
	}

	files, err := ToImages(bg, Input{Path: src}, ImageExportOptions{Mode: "pages", Format: "jpg", DPI: 72, Pages: "1,3"}, filepath.Join(dir, "img"), "t", noProgress)
	if err != nil || len(files) != 2 || !strings.HasSuffix(files[1], "t-3.jpg") {
		t.Fatalf("to images: %v %v", files, err)
	}
	if st, err := os.Stat(files[0]); err != nil || st.Size() < 1000 {
		t.Fatalf("image file: %v", err)
	}
	files, err = ToImages(bg, Input{Path: src}, ImageExportOptions{Mode: "extract"}, filepath.Join(dir, "ext"), "t", noProgress)
	if err != nil || len(files) != 3 {
		t.Fatalf("extract: %v %v", files, err)
	}
}

func TestCompress(t *testing.T) {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 3000, 2000))
	for y := 0; y < 2000; y++ {
		for x := 0; x < 3000; x++ {
			img.Pix[(y*3000+x)*4], img.Pix[(y*3000+x)*4+1], img.Pix[(y*3000+x)*4+2], img.Pix[(y*3000+x)*4+3] = uint8(x*y%251), uint8(x/12), uint8(y/8), 255
		}
	}
	src := filepath.Join(dir, "photo.pdf")
	if err := WriteImagePDF(src, []ImagePage{{Img: img, PageW: 842, PageH: 595, Box: [4]float64{0, 0, 842, 595}}}, 95); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "small.pdf")
	if err := Compress(context.Background(), Input{Path: src}, CompressOptions{Level: "extreme"}, out, noProgress); err != nil {
		t.Fatal(err)
	}
	a, _ := os.Stat(src)
	b, _ := os.Stat(out)
	if b.Size() > a.Size()/3 {
		t.Fatalf("compressed %d → %d", a.Size(), b.Size())
	}
	if info, err := Inspect(context.Background(), out); err != nil || info.Pages != 1 {
		t.Fatalf("result unreadable: %v", err)
	}
	saveRender(t, out, 0, "compressed.png")
	// Compressing again gains nothing.
	if err := Compress(context.Background(), Input{Path: out}, CompressOptions{Level: "low"}, filepath.Join(dir, "again.pdf"), noProgress); !errors.Is(err, ErrNoGain) {
		t.Fatalf("second compress: %v", err)
	}
}

func TestPDFAAndRepair(t *testing.T) {
	dir := t.TempDir()
	src := makeTextPDF(t, dir, "t.pdf", 2) // Helvetica is not embedded → pages become images
	out := filepath.Join(dir, "a.pdf")
	res, err := ToPDFA(context.Background(), Input{Path: src}, out, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.RasterPages) != 2 {
		t.Fatalf("raster pages %v", res.RasterPages)
	}
	pc, err := readCtx(out, "")
	if err != nil {
		t.Fatal(err)
	}
	root, _ := pc.Catalog()
	if _, ok := root.Find("OutputIntents"); !ok {
		t.Fatal("missing output intent")
	}
	if !streamsContain(t, out, "pdfaid:part>2") {
		t.Fatal("missing XMP")
	}
	info, _ := pc.DereferenceDict(*pc.Info)
	if mod, ok := info["ModDate"].(types.StringLiteral); !ok || !strings.HasPrefix(string(mod), "D:") {
		t.Fatalf("mod date %v", info["ModDate"])
	}
	if len(srgbProfile())%4 != 0 {
		t.Fatal("icc size must be 4-byte aligned")
	}

	// Truncated file with its xref table cut off.
	good := makePDF(t, dir, "g.pdf", 3)
	data, _ := os.ReadFile(good)
	broken := filepath.Join(dir, "broken.pdf")
	_ = os.WriteFile(broken, data[:len(data)-40], 0o644)
	fixed := filepath.Join(dir, "fixed.pdf")
	if err := Repair(context.Background(), Input{Path: broken}, fixed, dir); err != nil {
		t.Fatal(err)
	}
	if n := pageCount(t, fixed); n != 3 {
		t.Fatalf("repaired pages %d", n)
	}
}

func TestCompare(t *testing.T) {
	dir := t.TempDir()
	blank := makePDF(t, dir, "blank.pdf", 1)
	mk := func(name string, lines ...string) string {
		var items []Item
		for i, l := range lines {
			items = append(items, Item{Kind: "text", Text: l, X: 72, Y: 100 + float64(i)*20, Size: 12})
		}
		out := filepath.Join(dir, name)
		if err := DrawItems(Input{Path: blank}, map[int][]Item{1: items}, out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	a := mk("a.pdf", "Harga apel adalah 5000 rupiah", "Pengiriman hari Senin", "Terima kasih")
	b := mk("b.pdf", "Harga apel adalah 7000 rupiah", "Pengiriman hari Senin", "Terima kasih banyak")
	res, err := Compare(context.Background(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	if res.Removed != 1 || res.Added != 2 || len(res.Changes) != 3 {
		t.Fatalf("result %+v", res)
	}
	same, _ := Compare(context.Background(), a, a)
	if !same.Same {
		t.Fatal("a file equals itself")
	}
	// Large, completely different inputs stay bounded.
	var x, y []token
	for i := 0; i < 30000; i++ {
		x = append(x, token{key: fmt.Sprint("a", i)})
		y = append(y, token{key: fmt.Sprint("b", i)})
	}
	ka, _ := diffOps(x, y)
	if ka[0] {
		t.Fatal("different tokens must not be kept")
	}
}
