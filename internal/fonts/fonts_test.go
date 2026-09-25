package fonts

import (
	"encoding/binary"
	"runtime"
	"testing"
)

func TestGoFallbackCoversLatin(t *testing.T) {
	g := Go(false)
	if !g.Covers("Hello, dunia!") {
		t.Fatal("Go font should cover Latin")
	}
	if g.TextWidth("Hello", 12) <= 0 {
		t.Fatal("zero width")
	}
}

func TestForAndSubset(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("needs Windows fonts")
	}
	// Segoe UI (Latin, Cyrillic, Greek) is always there; CJK and Indic fonts can be missing
	// on trimmed Windows installs (e.g. CI servers), so those are only checked when present.
	required := map[string]bool{"Привет мир": true, "Καλημέρα": true, "Halo café": true}
	for _, text := range []string{"Привет мир", "Καλημέρα", "日本語のテキスト", "नमस्ते", "Halo café"} {
		f := For(text, false)
		if f == nil || !f.Covers(text) {
			if required[text] {
				t.Fatalf("%q: no covering font (got %v)", text, f)
			}
			t.Logf("%q: no font installed for this script, skipped", text)
			continue
		}
		s, err := MakeSubset(f, text)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		dir, err := tableDir(s.Data)
		if err != nil {
			t.Fatal(err)
		}
		maxp := s.Data[dir["maxp"].off:]
		n := int(binary.BigEndian.Uint16(maxp[4:]))
		if n < 2 || n > 40 {
			t.Fatalf("%q: subset has %d glyphs", text, n)
		}
		for _, r := range text {
			if r != ' ' && s.GID[r] == 0 {
				t.Fatalf("%q: rune %q missing", text, r)
			}
		}
		if s.Width(text, 10) <= 0 {
			t.Fatalf("%q: zero width", text)
		}
		if checksum(s.Data) != 0xB1B0AFBA {
			t.Fatalf("%q: bad font checksum", text)
		}
		if len(s.Data) > 200*1024 {
			t.Fatalf("%q: subset too big: %d bytes", text, len(s.Data))
		}
	}
}
