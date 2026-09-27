package fonts

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func TestShapeArabicAndDevanagari(t *testing.T) {
	seg := filepath.Join(fontsDir(), "segoeui.ttf")
	if _, err := os.Stat(seg); err != nil {
		t.Skip("Segoe UI not installed")
	}
	ar := "سلام" // s-l-a-m: joined forms, drawn right to left
	f := For(ar, false)
	sh, err := Shape(f, ar)
	if err != nil {
		t.Fatal(err)
	}
	var buf []uint16
	for _, g := range sh.Glyphs {
		buf = append(buf, g.GID)
	}
	t.Logf("%s: %d glyphs %v width %.0f", f.Name, len(sh.Glyphs), buf, sh.Width)
	// Joined letters use contextual glyphs, not the isolated ones the character map gives.
	iso := map[uint16]bool{}
	for _, r := range ar {
		g, _ := f.Font.GlyphIndex(nil, r)
		iso[uint16(g)] = true
	}
	joined := 0
	for _, g := range sh.Glyphs {
		if !iso[g.GID] {
			joined++
		}
	}
	if joined == 0 {
		t.Error("Arabic was not shaped (only isolated forms)")
	}
	// Visual order: the first drawn glyph (leftmost) is the last letter, meem.
	if last := sh.Glyphs[len(sh.Glyphs)-1]; last.Text != "س" {
		t.Errorf("rightmost glyph is %q, want the first letter", last.Text)
	}
	hi := "क्षत्रिय"
	fh := For(hi, false)
	shh, err := Shape(fh, hi)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %d runes → %d glyphs", fh.Name, len([]rune(hi)), len(shh.Glyphs))
	// Windows Server images (CI) have no Devanagari font (Nirmala UI): nothing to shape with.
	if !fh.Covers(hi) {
		t.Logf("no installed font covers Devanagari (fell back to %s); conjunct check skipped", fh.Name)
	} else if len(shh.Glyphs) >= len([]rune(hi)) {
		t.Error("Devanagari conjuncts were not formed")
	}
	sub, err := MakeSubsetWith(f, ar, buf)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range buf {
		if _, ok := sub.Remap[g]; !ok {
			t.Errorf("glyph %d missing from subset", g)
		}
	}
	if !NeedsShaping(ar) || !NeedsShaping(hi) || NeedsShaping("Привет мир") {
		t.Error("NeedsShaping")
	}
}

// The subset must be a valid font whose glyphs keep their outlines (the table directory was
// once written only partly, so PDF readers dropped most characters).
func TestSubsetIsValidFont(t *testing.T) {
	for _, text := range []string{"Привет мир 1", "Γειά σου", "नमस्ते", "سلام"} {
		f := For(text, false)
		sub, err := MakeSubset(f, text)
		if err != nil {
			t.Fatal(err)
		}
		sf, err := sfnt.Parse(sub.Data)
		if err != nil {
			t.Fatalf("%s: subset doesn't parse: %v", text, err)
		}
		var b sfnt.Buffer
		for _, r := range text {
			if r == ' ' {
				continue
			}
			segs, err := sf.LoadGlyph(&b, sfnt.GlyphIndex(sub.GID[r]), fixed.I(1000), nil)
			if err != nil || len(segs) == 0 {
				t.Errorf("%q in %s: no outline (%v)", r, f.Name, err)
			}
		}
	}
}
