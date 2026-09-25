package pdf

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kuymediabox/internal/fonts"
)

func TestUnicodeText(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("needs Windows fonts")
	}
	dir := t.TempDir()
	blank := makePDF(t, dir, "blank.pdf", 1)
	out := filepath.Join(dir, "uni.pdf")
	items := map[int][]Item{1: {
		{Kind: "text", Text: "Привет, мир", X: 72, Y: 100, Size: 18},
		{Kind: "text", Text: "Halo biasa", X: 72, Y: 180, Size: 12},
		{Kind: "text", Text: "Страница 1 из 3", X: 300, Y: 300, Size: 12, Align: "center", Angle: 30},
	}}
	want := []string{"Привет", "Halo biasa", "Страница"}
	// Japanese only where a CJK font is installed (not on every Windows server).
	if fonts.For("日本語テキスト", true).Covers("日本語テキスト") {
		items[1] = append(items[1], Item{Kind: "text", Text: "日本語テキスト", X: 72, Y: 140, Size: 18, Bold: true})
		want = append(want, "日本語")
	}
	if err := DrawItems(Input{Path: blank}, items, out); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(context.Background(), out, "")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	text, err := doc.PageText(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Errorf("page text %q misses %q", text, w)
		}
	}
	img, err := doc.Render(0, 600, 800)
	if err != nil {
		t.Fatal(err)
	}
	dark := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Min.Y+b.Dy()/4; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r+g+bl < 3*0x6000 {
				dark++
			}
		}
	}
	if dark < 200 {
		t.Fatalf("glyphs not drawn (%d dark pixels)", dark)
	}
	if w := TextWidth("Привет", 10, false); w <= 0 {
		t.Fatal("zero width")
	}
}
