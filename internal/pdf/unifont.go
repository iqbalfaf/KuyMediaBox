package pdf

import (
	"crypto/sha1"
	"fmt"
	"sort"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"golang.org/x/text/encoding/charmap"

	"kuymediabox/internal/fonts"
)

// Text that Helvetica/WinAnsi can't show (Cyrillic, Greek, CJK, Devanagari, …) is drawn with
// a Windows font that covers it, embedded as a subset (Type0 / CIDFontType2, Identity-H).

// winAnsiOK reports whether every character of s exists in WinAnsi (Windows-1252).
func winAnsiOK(s string) bool {
	enc := charmap.Windows1252.NewEncoder()
	for _, r := range s {
		if r == '\n' || r == '\r' {
			continue
		}
		b, err := enc.Bytes([]byte(string(r)))
		if err != nil || len(b) != 1 {
			return false
		}
	}
	return true
}

// uniFont is one embedded font on a page: its resource name and subset.
type uniFont struct {
	res    string
	face   *fonts.Face
	text   strings.Builder
	sub    *fonts.Subset
	shaped map[string]fonts.Shaped // lines of complex scripts (Arabic, Indic, …)
}

// collectUnicode picks fonts for the texts that need one and builds their subsets.
func (b *overlayBuilder) collectUnicode(items []Item) error {
	for _, it := range items {
		if it.Kind != "text" || strings.TrimSpace(it.Text) == "" || winAnsiOK(it.Text) {
			continue
		}
		face := fonts.For(it.Text, it.Bold)
		uf := b.ufonts[face.Name]
		if uf == nil {
			uf = &uniFont{res: fmt.Sprintf("KmbU%d", len(b.ufonts)), face: face, shaped: map[string]fonts.Shaped{}}
			b.ufonts[face.Name] = uf
		}
		uf.text.WriteString(it.Text)
		for _, line := range strings.Split(strings.ReplaceAll(it.Text, "\r\n", "\n"), "\n") {
			if _, done := uf.shaped[line]; done || !fonts.NeedsShaping(line) {
				continue
			}
			if sh, err := fonts.Shape(face, line); err == nil {
				uf.shaped[line] = sh
			}
		}
	}
	for _, uf := range b.ufonts {
		var extra []uint16
		for _, sh := range uf.shaped {
			for _, g := range sh.Glyphs {
				extra = append(extra, g.GID)
			}
		}
		sub, err := fonts.MakeSubsetWith(uf.face, uf.text.String(), extra)
		if err != nil {
			return err
		}
		uf.sub = sub
	}
	return nil
}

// uniFor returns the embedded font for a text item, or nil when Helvetica is used.
func (b *overlayBuilder) uniFor(it Item) *uniFont {
	if winAnsiOK(it.Text) {
		return nil
	}
	return b.ufonts[fonts.For(it.Text, it.Bold).Name]
}

// hexGlyphs encodes text as 2-byte glyph ids of the subset.
func (uf *uniFont) hexGlyphs(s string) string {
	var sb strings.Builder
	sb.WriteByte('<')
	for _, r := range s {
		fmt.Fprintf(&sb, "%04X", uf.sub.GID[r])
	}
	sb.WriteByte('>')
	return sb.String()
}

// fontDict writes the Type0 font objects and returns the reference of the font dict.
func (uf *uniFont) fontDict(ctx *model.Context) (*types.IndirectRef, error) {
	sub := uf.sub
	sum := sha1.Sum(sub.Data)
	tag := make([]byte, 6)
	for i := range tag {
		tag[i] = 'A' + sum[i]%26
	}
	base := string(tag) + "+" + sub.PostScript

	ff, err := ctx.XRefTable.NewStreamDictForBuf(sub.Data)
	if err != nil {
		return nil, err
	}
	ff.InsertInt("Length1", len(sub.Data))
	if err := ff.Encode(); err != nil {
		return nil, err
	}
	ffRef, err := ctx.XRefTable.IndRefForNewObject(*ff)
	if err != nil {
		return nil, err
	}

	fd := types.NewDict()
	fd.InsertName("Type", "FontDescriptor")
	fd.InsertName("FontName", base)
	fd.InsertInt("Flags", 4)
	fd.Insert("FontBBox", types.NewNumberArray(sub.BBox[0], sub.BBox[1], sub.BBox[2], sub.BBox[3]))
	fd.InsertInt("ItalicAngle", 0)
	fd.Insert("Ascent", types.Float(sub.Ascent))
	fd.Insert("Descent", types.Float(sub.Descent))
	fd.Insert("CapHeight", types.Float(sub.CapHeight))
	fd.InsertInt("StemV", 80)
	fd.Insert("FontFile2", *ffRef)
	fdRef, err := ctx.XRefTable.IndRefForNewObject(fd)
	if err != nil {
		return nil, err
	}

	widths := types.Array{}
	for _, w := range sub.Widths {
		widths = append(widths, types.Integer(int(w+0.5)))
	}
	cid := types.NewDict()
	cid.InsertName("Type", "Font")
	cid.InsertName("Subtype", "CIDFontType2")
	cid.InsertName("BaseFont", base)
	sys := types.NewDict()
	sys.Insert("Registry", types.StringLiteral("Adobe"))
	sys.Insert("Ordering", types.StringLiteral("Identity"))
	sys.InsertInt("Supplement", 0)
	cid.Insert("CIDSystemInfo", sys)
	cid.Insert("FontDescriptor", *fdRef)
	cid.Insert("W", types.Array{types.Integer(0), widths})
	cid.InsertName("CIDToGIDMap", "Identity")
	cidRef, err := ctx.XRefTable.IndRefForNewObject(cid)
	if err != nil {
		return nil, err
	}

	cmap, err := ctx.XRefTable.NewStreamDictForBuf([]byte(toUnicode(sub, uf.shapedText())))
	if err != nil {
		return nil, err
	}
	if err := cmap.Encode(); err != nil {
		return nil, err
	}
	cmapRef, err := ctx.XRefTable.IndRefForNewObject(*cmap)
	if err != nil {
		return nil, err
	}

	f := types.NewDict()
	f.InsertName("Type", "Font")
	f.InsertName("Subtype", "Type0")
	f.InsertName("BaseFont", base)
	f.InsertName("Encoding", "Identity-H")
	f.Insert("DescendantFonts", types.Array{*cidRef})
	f.Insert("ToUnicode", *cmapRef)
	return ctx.XRefTable.IndRefForNewObject(f)
}

// shapedText maps the subset glyphs of shaped lines to the characters they stand for.
func (uf *uniFont) shapedText() map[uint16]string {
	out := map[uint16]string{}
	for _, sh := range uf.shaped {
		for _, g := range sh.Glyphs {
			if nw, ok := uf.sub.Remap[g.GID]; ok && g.Text != "" && out[nw] == "" {
				out[nw] = g.Text
			}
		}
	}
	return out
}

// shapedLine draws a shaped line glyph by glyph (each has its own offset) starting at x, y.
// ActualText gives readers the text in logical order for copying and searching (the glyphs
// of right-to-left and Indic text are stored in visual order).
func (uf *uniFont) shapedLine(sh fonts.Shaped, line string, x, y, size float64) string {
	var sb strings.Builder
	sb.WriteString("/Span <</ActualText <FEFF")
	for _, r := range line {
		sb.WriteString(utf16Hex(r))
	}
	sb.WriteString(">>> BDC\n")
	for _, g := range sh.Glyphs {
		fmt.Fprintf(&sb, "1 0 0 1 %s %s Tm <%04X> Tj\n", num(x+g.X*size/1000), num(y+g.Y*size/1000), uf.sub.Remap[g.GID])
	}
	sb.WriteString("EMC\n")
	return sb.String()
}

// toUnicode maps glyph ids back to text so the result can be searched and copied.
func toUnicode(sub *fonts.Subset, shaped map[uint16]string) string {
	type pair struct {
		gid uint16
		s   string
	}
	var pairs []pair
	seen := map[uint16]bool{}
	for r, g := range sub.GID {
		if g != 0 && !seen[g] {
			seen[g] = true
			pairs = append(pairs, pair{g, string(r)})
		}
	}
	for g, s := range shaped {
		if g != 0 && !seen[g] {
			seen[g] = true
			pairs = append(pairs, pair{g, s})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].gid < pairs[j].gid })
	var sb strings.Builder
	sb.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n")
	sb.WriteString("/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n")
	sb.WriteString("/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n")
	sb.WriteString("1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	for i := 0; i < len(pairs); i += 100 {
		chunk := pairs[i:min(i+100, len(pairs))]
		fmt.Fprintf(&sb, "%d beginbfchar\n", len(chunk))
		for _, p := range chunk {
			var u strings.Builder
			for _, r := range p.s {
				u.WriteString(utf16Hex(r))
			}
			fmt.Fprintf(&sb, "<%04X> <%s>\n", p.gid, u.String())
		}
		sb.WriteString("endbfchar\n")
	}
	sb.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	return sb.String()
}

func utf16Hex(r rune) string {
	if r < 0x10000 {
		return fmt.Sprintf("%04X", r)
	}
	r -= 0x10000
	return fmt.Sprintf("%04X%04X", 0xD800+(r>>10), 0xDC00+(r&0x3FF))
}
