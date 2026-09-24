package pdf

import (
	"bytes"
	"context"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// textAt draws "Halo" at display point (x, y) and returns where the engine finds its first char.
func textAt(t *testing.T, src string, x, y float64) (u, v, dw, dh float64) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "o.pdf")
	if err := DrawItems(Input{Path: src}, map[int][]Item{1: {{Kind: "text", Text: "Halo", X: x, Y: y, Size: 20}}}, out); err != nil {
		t.Fatal(err)
	}
	d, err := Open(context.Background(), out, "")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	dw, dh, _ = d.Size(0)
	chars, err := d.Chars(0)
	if err != nil || len(chars) == 0 {
		t.Fatalf("no text found: %v", err)
	}
	var s strings.Builder
	for _, c := range chars {
		s.WriteString(c.Text)
	}
	if !strings.Contains(s.String(), "Halo") {
		t.Fatalf("text = %q", s.String())
	}
	return chars[0].X0, chars[0].Y1, dw, dh
}

func TestOverlayPositions(t *testing.T) {
	dir := t.TempDir()
	src := makePDF(t, dir, "a.pdf", 1)
	for _, deg := range []int{0, 90, 180, 270} {
		in := src
		if deg != 0 {
			in = filepath.Join(dir, "r.pdf")
			if err := Rotate(Input{Path: src}, deg, "", in); err != nil {
				t.Fatal(err)
			}
		}
		u, v, dw, dh := textAt(t, in, 100, 150)
		// First glyph starts at x = 100 and sits on the baseline y = 150 (bottom of box).
		if math.Abs(u*dw-100) > 3 || math.Abs(v*dh-150) > 6 {
			t.Errorf("rotate %d: glyph at %.1f,%.1f (page %.0fx%.0f)", deg, u*dw, v*dh, dw, dh)
		}
	}
}

func TestStampsRender(t *testing.T) {
	dir := t.TempDir()
	src := makePDF(t, dir, "a.pdf", 3)
	var logo bytes.Buffer
	_ = png.Encode(&logo, solid(40, 20, color.RGBA{255, 0, 0, 128}))
	logoPath := filepath.Join(dir, "logo.png")
	_ = os.WriteFile(logoPath, logo.Bytes(), 0o644)

	wm := filepath.Join(dir, "wm.pdf")
	if err := Watermark(Input{Path: src}, WatermarkOptions{Type: "text", Text: "RAHASIA", Size: 40, Color: "#cc0000", Opacity: 0.3, Angle: 45, Position: "tile"}, wm); err != nil {
		t.Fatal(err)
	}
	wmi := filepath.Join(dir, "wmi.pdf")
	if err := Watermark(Input{Path: wm}, WatermarkOptions{Type: "image", Image: logoPath, Scale: 30, Position: "br", Under: true}, wmi); err != nil {
		t.Fatal(err)
	}
	num := filepath.Join(dir, "num.pdf")
	if err := AddPageNumbers(Input{Path: wmi}, NumberOptions{Position: "br", Format: "Halaman {n} dari {total}", Pages: "2-"}, num); err != nil {
		t.Fatal(err)
	}
	d, err := Open(context.Background(), num, "")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	txt, _ := d.PageText(2)
	if !strings.Contains(txt, "Halaman 2 dari 2") || !strings.Contains(txt, "RAHASIA") {
		t.Fatalf("page 3 text = %q", txt)
	}
	if txt, _ := d.PageText(0); strings.Contains(txt, "Halaman") {
		t.Fatalf("page 1 must not be numbered: %q", txt)
	}
	saveRender(t, num, 2, "stamps.png")
	if _, err := d.RenderDPI(0, 72); err != nil {
		t.Fatal(err)
	}
}
