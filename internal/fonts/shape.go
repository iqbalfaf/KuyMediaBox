package fonts

import (
	"bytes"
	"sync"

	"github.com/go-text/typesetting/di"
	gtfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/unicode/bidi"
)

// Complex scripts (Arabic joining forms, Indic conjuncts and vowel signs, Thai marks, right-
// to-left order) need shaping: characters can't simply be drawn one after another.

// NeedsShaping reports whether text contains a script that must be shaped.
func NeedsShaping(s string) bool {
	for _, r := range s {
		switch {
		case r >= 0x0590 && r <= 0x08FF, // Hebrew, Arabic, Syriac, Thaana, NKo, Arabic supplements
			r >= 0x0900 && r <= 0x0DFF, // Devanagari … Sinhala
			r >= 0x0E00 && r <= 0x0EFF, // Thai, Lao
			r >= 0x0F00 && r <= 0x0FFF, // Tibetan
			r >= 0x1000 && r <= 0x109F, // Myanmar
			r >= 0x1780 && r <= 0x17FF, // Khmer
			r >= 0xA8E0 && r <= 0xA8FF, // Devanagari extended
			r >= 0xFB1D && r <= 0xFDFF, // Hebrew & Arabic presentation forms A
			r >= 0xFE70 && r <= 0xFEFF: // Arabic presentation forms B
			return true
		}
	}
	return false
}

// ShapedGlyph is one glyph to draw: its id in the original font and where its origin goes,
// in 1/1000 em from the start of the line (Y up).
type ShapedGlyph struct {
	GID  uint16
	X, Y float64
	Text string // the characters this glyph stands for ("" for the other glyphs of a cluster)
}

// Shaped is a shaped line of text.
type Shaped struct {
	Glyphs []ShapedGlyph
	Width  float64 // 1/1000 em
}

var (
	shapeMu sync.Mutex
	gtFaces = map[string]*gtfont.Face{}
	shaper  shaping.HarfbuzzShaper
)

func gtFace(f *Face) (*gtfont.Face, error) {
	if gf, ok := gtFaces[f.Name]; ok {
		return gf, nil
	}
	gf, err := gtfont.ParseTTF(bytes.NewReader(f.Data))
	if err != nil {
		return nil, err
	}
	gtFaces[f.Name] = gf
	return gf, nil
}

// size used for shaping: 1000 units per em, so results are already in 1/1000 em.
const shapeSize = 1000

// Shape lays out one line: bidi runs in visual order, each shaped with its own script and
// direction.
func Shape(f *Face, line string) (Shaped, error) {
	shapeMu.Lock()
	defer shapeMu.Unlock()
	gf, err := gtFace(f)
	if err != nil {
		return Shaped{}, err
	}
	var p bidi.Paragraph
	if _, err := p.SetString(line); err != nil {
		return Shaped{}, err
	}
	ord, err := p.Order()
	if err != nil {
		return Shaped{}, err
	}
	var out Shaped
	pen := 0.0
	for i := 0; i < ord.NumRuns(); i++ {
		run := ord.Run(i)
		text := []rune(run.String())
		if len(text) == 0 {
			continue
		}
		dir := di.DirectionLTR
		if run.Direction() == bidi.RightToLeft {
			dir = di.DirectionRTL
		}
		script := language.Common
		for _, r := range text {
			if s := language.LookupScript(r); s != language.Common && s != language.Inherited && s != language.Unknown {
				script = s
				break
			}
		}
		res := shaper.Shape(shaping.Input{
			Text: text, RunStart: 0, RunEnd: len(text), Direction: dir, Face: gf,
			Size: fixed.I(shapeSize), Script: script, Language: language.DefaultLanguage(),
		})
		seen := map[int]bool{}
		for _, g := range res.Glyphs {
			sg := ShapedGlyph{GID: uint16(g.GlyphID), X: pen + f26(g.XOffset), Y: f26(g.YOffset)}
			if !seen[g.ClusterIndex] {
				seen[g.ClusterIndex] = true
				end := min(g.ClusterIndex+max(g.RuneCount, 1), len(text))
				sg.Text = string(text[g.ClusterIndex:end])
			}
			out.Glyphs = append(out.Glyphs, sg)
			pen += f26(g.Advance)
		}
	}
	out.Width = pen
	return out, nil
}

func f26(v fixed.Int26_6) float64 { return float64(v) / 64 }
