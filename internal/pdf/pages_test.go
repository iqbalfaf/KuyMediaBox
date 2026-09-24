package pdf

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"testing"
)

func solid(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	return img
}

// makePDF writes a PDF with n A4 pages, each showing a coloured block.
func makePDF(t *testing.T, dir, name string, n int) string {
	t.Helper()
	var pages []ImagePage
	for i := 0; i < n; i++ {
		pages = append(pages, ImagePage{Img: solid(100, 50, color.RGBA{uint8(40 * i), 100, 200, 255}), PageW: 595, PageH: 842, Box: [4]float64{50, 600, 200, 100}})
	}
	p := filepath.Join(dir, name)
	if err := WriteImagePDF(p, pages, 80); err != nil {
		t.Fatal(err)
	}
	return p
}

func pageCount(t *testing.T, path string) int {
	t.Helper()
	ctx, err := readCtx(path, "")
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return ctx.PageCount
}

func TestPageOperations(t *testing.T) {
	dir := t.TempDir()
	a := makePDF(t, dir, "a.pdf", 3)
	b := makePDF(t, dir, "b.pdf", 2)
	bg := context.Background()

	merged := filepath.Join(dir, "merged.pdf")
	if err := Merge(bg, []Input{{Path: a}, {Path: b}}, merged, dir, noProgress); err != nil {
		t.Fatal(err)
	}
	if n := pageCount(t, merged); n != 5 {
		t.Fatalf("merged pages = %d", n)
	}

	files, err := Split(bg, Input{Path: merged}, SplitOptions{Mode: "ranges", Ranges: "1-2, 3-5"}, filepath.Join(dir, "split"), "m", noProgress)
	if err != nil || len(files) != 2 || pageCount(t, files[1]) != 3 {
		t.Fatalf("split: %v %v", files, err)
	}
	files, err = Split(bg, Input{Path: merged}, SplitOptions{Mode: "every", Every: 2}, filepath.Join(dir, "every"), "m", noProgress)
	if err != nil || len(files) != 3 {
		t.Fatalf("split every: %v %v", files, err)
	}

	removed := filepath.Join(dir, "removed.pdf")
	if err := RemovePages(Input{Path: merged}, "2, 4-5", removed); err != nil || pageCount(t, removed) != 2 {
		t.Fatalf("remove: %v", err)
	}
	if err := RemovePages(Input{Path: merged}, "1-5", removed); err == nil {
		t.Fatal("removing every page must fail")
	}

	extracted := filepath.Join(dir, "extracted.pdf")
	if err := ExtractPages(Input{Path: merged}, "5,1", extracted); err != nil || pageCount(t, extracted) != 2 {
		t.Fatalf("extract: %v", err)
	}

	rotated := filepath.Join(dir, "rotated.pdf")
	if err := Rotate(Input{Path: a}, 90, "", rotated); err != nil {
		t.Fatal(err)
	}
	d, err := Open(bg, rotated, "")
	if err != nil {
		t.Fatal(err)
	}
	w, h, _ := d.Size(0)
	g, _ := d.Geometry(0)
	d.Close()
	if w < h || g.Rotate != 90 {
		t.Fatalf("rotated size %vx%v rot %d", w, h, g.Rotate)
	}

	org := filepath.Join(dir, "org.pdf")
	err = Organize(bg, []Input{{Path: a}, {Path: b}}, []PageRef{{Src: 1, Page: 2}, {Src: -1, W: 300, H: 300}, {Src: 0, Page: 1, Rotate: 180}, {Src: 0, Page: 1}}, org, dir, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	if n := pageCount(t, org); n != 4 {
		t.Fatalf("organized pages = %d", n)
	}

	if _, err := ParsePages("3-1, 7", 5); err == nil {
		t.Fatal("page 7 of 5 must fail")
	}
	if p, _ := ParsePages("4-, 1", 5); len(p) != 3 {
		t.Fatalf("open range: %v", p)
	}
}
