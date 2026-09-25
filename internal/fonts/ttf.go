package fonts

import (
	"encoding/binary"
	"fmt"
	"sort"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type table struct {
	off, length uint32
}

// tableDir reads the table records of a single TrueType font.
func tableDir(data []byte) (map[string]table, error) {
	if len(data) < 12 {
		return nil, errBadFont
	}
	n := int(binary.BigEndian.Uint16(data[4:]))
	if len(data) < 12+16*n {
		return nil, errBadFont
	}
	out := make(map[string]table, n)
	for i := 0; i < n; i++ {
		r := data[12+16*i:]
		t := table{off: binary.BigEndian.Uint32(r[8:]), length: binary.BigEndian.Uint32(r[12:])}
		if uint64(t.off)+uint64(t.length) > uint64(len(data)) {
			return nil, errBadFont
		}
		out[string(r[:4])] = t
	}
	return out, nil
}

// firstOfCollection copies the first font of a TrueType collection (.ttc) into a standalone font.
func firstOfCollection(data []byte) ([]byte, error) {
	if len(data) < 16 || string(data[:4]) != "ttcf" {
		return data, nil
	}
	off := binary.BigEndian.Uint32(data[12:])
	if int(off)+12 > len(data) {
		return nil, errBadFont
	}
	n := int(binary.BigEndian.Uint16(data[off+4:]))
	tables := map[string][]byte{}
	for i := 0; i < n; i++ {
		r := int(off) + 12 + 16*i
		if r+16 > len(data) {
			return nil, errBadFont
		}
		to, tl := binary.BigEndian.Uint32(data[r+8:]), binary.BigEndian.Uint32(data[r+12:])
		if uint64(to)+uint64(tl) > uint64(len(data)) {
			return nil, errBadFont
		}
		tables[string(data[r:r+4])] = data[to : to+tl]
	}
	return writeFont(tables), nil
}

func checksum(b []byte) uint32 {
	var sum uint32
	for i := 0; i < len(b); i += 4 {
		var w [4]byte
		copy(w[:], b[i:min(i+4, len(b))])
		sum += binary.BigEndian.Uint32(w[:])
	}
	return sum
}

// writeFont assembles tables into a TrueType file with correct checksums.
func writeFont(tables map[string][]byte) []byte {
	tags := make([]string, 0, len(tables))
	for t := range tables {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	n := len(tags)
	sr, es := 1, 0
	for sr*2 <= n {
		sr *= 2
		es++
	}
	hdr := make([]byte, 12+16*n)
	binary.BigEndian.PutUint32(hdr[0:], 0x00010000)
	binary.BigEndian.PutUint16(hdr[4:], uint16(n))
	binary.BigEndian.PutUint16(hdr[6:], uint16(sr*16))
	binary.BigEndian.PutUint16(hdr[8:], uint16(es))
	binary.BigEndian.PutUint16(hdr[10:], uint16(n*16-sr*16))
	out := hdr
	headAt := -1
	for i, t := range tags {
		d := tables[t]
		if t == "head" && len(d) >= 12 {
			d = append([]byte(nil), d...)
			binary.BigEndian.PutUint32(d[8:], 0) // checkSumAdjustment, fixed below
			tables[t] = d
			headAt = len(out)
		}
		r := hdr[12+16*i:]
		copy(r, t)
		binary.BigEndian.PutUint32(r[4:], checksum(d))
		binary.BigEndian.PutUint32(r[8:], uint32(len(out)))
		binary.BigEndian.PutUint32(r[12:], uint32(len(d)))
		out = append(out, d...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	if headAt >= 0 {
		binary.BigEndian.PutUint32(out[headAt+8:], 0xB1B0AFBA-checksum(out))
	}
	return out
}

// Subset is a TrueType font cut down to the glyphs of some text, renumbered densely.
type Subset struct {
	Data       []byte          // the TrueType font
	GID        map[rune]uint16 // rune → glyph id in Data (0 = missing)
	Widths     []float64       // advance per new glyph id, in 1/1000 em
	Ascent     float64         // 1/1000 em
	Descent    float64         // negative, 1/1000 em
	CapHeight  float64
	BBox       [4]float64
	UnitsPerEm int
	PostScript string
}

// Width returns the width of text in points at size.
func (s *Subset) Width(text string, size float64) float64 {
	w := 0.0
	for _, r := range text {
		g := s.GID[r]
		if int(g) < len(s.Widths) {
			w += s.Widths[g]
		}
	}
	return w * size / 1000
}

// MakeSubset builds a subset of face with every rune of text.
func MakeSubset(face *Face, text string) (*Subset, error) {
	src := face.Data
	dir, err := tableDir(src)
	if err != nil {
		return nil, err
	}
	get := func(tag string) []byte {
		t, ok := dir[tag]
		if !ok {
			return nil
		}
		return src[t.off : t.off+t.length]
	}
	head, hhea, maxp, loca, glyf, hmtx := get("head"), get("hhea"), get("maxp"), get("loca"), get("glyf"), get("hmtx")
	if len(head) < 54 || len(hhea) < 36 || len(maxp) < 6 || loca == nil || glyf == nil || hmtx == nil {
		return nil, fmt.Errorf("%s: %w", face.Name, errBadFont)
	}
	numGlyphs := int(binary.BigEndian.Uint16(maxp[4:]))
	longLoca := binary.BigEndian.Uint16(head[50:]) == 1
	glyphRange := func(g int) (int, int) {
		if g < 0 || g >= numGlyphs {
			return 0, 0
		}
		var a, b int
		if longLoca {
			if 4*g+8 > len(loca) {
				return 0, 0
			}
			a, b = int(binary.BigEndian.Uint32(loca[4*g:])), int(binary.BigEndian.Uint32(loca[4*g+4:]))
		} else {
			if 2*g+4 > len(loca) {
				return 0, 0
			}
			a, b = 2*int(binary.BigEndian.Uint16(loca[2*g:])), 2*int(binary.BigEndian.Uint16(loca[2*g+2:]))
		}
		if a > b || b > len(glyf) {
			return 0, 0
		}
		return a, b
	}

	// Old glyph ids in first-use order, notdef first.
	var buf sfnt.Buffer
	order := []int{0}
	newID := map[int]uint16{0: 0}
	var add func(g int)
	add = func(g int) {
		if _, ok := newID[g]; ok || g >= numGlyphs {
			return
		}
		newID[g] = uint16(len(order))
		order = append(order, g)
		a, b := glyphRange(g)
		d := glyf[a:b]
		if len(d) < 10 || int16(binary.BigEndian.Uint16(d)) >= 0 {
			return
		}
		for _, c := range components(d) {
			add(int(binary.BigEndian.Uint16(d[c:])))
		}
	}
	gidOf := map[rune]uint16{}
	for _, r := range text {
		if _, ok := gidOf[r]; ok {
			continue
		}
		g, err := face.Font.GlyphIndex(&buf, r)
		if err != nil || g == 0 {
			gidOf[r] = 0
			continue
		}
		add(int(g))
		gidOf[r] = newID[int(g)]
	}

	// glyf + loca (long) with component references renumbered.
	var newGlyf []byte
	newLoca := make([]byte, 4*(len(order)+1))
	for i, g := range order {
		binary.BigEndian.PutUint32(newLoca[4*i:], uint32(len(newGlyf)))
		a, b := glyphRange(g)
		d := append([]byte(nil), glyf[a:b]...)
		if len(d) >= 10 && int16(binary.BigEndian.Uint16(d)) < 0 {
			for _, c := range components(d) {
				old := int(binary.BigEndian.Uint16(d[c:]))
				binary.BigEndian.PutUint16(d[c:], newID[old])
			}
		}
		newGlyf = append(newGlyf, d...)
		for len(newGlyf)%4 != 0 {
			newGlyf = append(newGlyf, 0)
		}
	}
	binary.BigEndian.PutUint32(newLoca[4*len(order):], uint32(len(newGlyf)))

	// hmtx: one full metric per glyph.
	nHM := int(binary.BigEndian.Uint16(hhea[34:]))
	metric := func(g int) (uint16, int16) {
		if nHM == 0 {
			return 0, 0
		}
		if g < nHM && 4*g+4 <= len(hmtx) {
			return binary.BigEndian.Uint16(hmtx[4*g:]), int16(binary.BigEndian.Uint16(hmtx[4*g+2:]))
		}
		adv := binary.BigEndian.Uint16(hmtx[4*(nHM-1):])
		lsbAt := 4*nHM + 2*(g-nHM)
		var lsb int16
		if lsbAt+2 <= len(hmtx) {
			lsb = int16(binary.BigEndian.Uint16(hmtx[lsbAt:]))
		}
		return adv, lsb
	}
	upem := int(binary.BigEndian.Uint16(head[18:]))
	if upem <= 0 {
		upem = 1000
	}
	newHmtx := make([]byte, 4*len(order))
	widths := make([]float64, len(order))
	for i, g := range order {
		adv, lsb := metric(g)
		binary.BigEndian.PutUint16(newHmtx[4*i:], adv)
		binary.BigEndian.PutUint16(newHmtx[4*i+2:], uint16(lsb))
		widths[i] = float64(adv) * 1000 / float64(upem)
	}

	newHead := append([]byte(nil), head...)
	binary.BigEndian.PutUint16(newHead[50:], 1)
	newHhea := append([]byte(nil), hhea...)
	binary.BigEndian.PutUint16(newHhea[34:], uint16(len(order)))
	newMaxp := append([]byte(nil), maxp...)
	binary.BigEndian.PutUint16(newMaxp[4:], uint16(len(order)))
	post := make([]byte, 32)
	binary.BigEndian.PutUint32(post, 0x00030000) // format 3: no glyph names

	tables := map[string][]byte{"head": newHead, "hhea": newHhea, "maxp": newMaxp, "loca": newLoca, "glyf": newGlyf, "hmtx": newHmtx, "post": post}
	for _, tag := range []string{"cvt ", "fpgm", "prep", "OS/2"} {
		if d := get(tag); d != nil {
			tables[tag] = d
		}
	}
	if d := get("name"); d != nil && len(d) < 64*1024 {
		tables["name"] = d
	}

	s := &Subset{Data: writeFont(tables), GID: gidOf, Widths: widths, UnitsPerEm: upem}
	scale := 1000 / float64(upem)
	s.BBox = [4]float64{
		float64(int16(binary.BigEndian.Uint16(head[36:]))) * scale, float64(int16(binary.BigEndian.Uint16(head[38:]))) * scale,
		float64(int16(binary.BigEndian.Uint16(head[40:]))) * scale, float64(int16(binary.BigEndian.Uint16(head[42:]))) * scale,
	}
	s.Ascent = float64(int16(binary.BigEndian.Uint16(hhea[4:]))) * scale
	s.Descent = float64(int16(binary.BigEndian.Uint16(hhea[6:]))) * scale
	s.CapHeight = s.Ascent * 0.7
	if os2 := get("OS/2"); len(os2) >= 90 && binary.BigEndian.Uint16(os2) >= 2 {
		s.CapHeight = float64(int16(binary.BigEndian.Uint16(os2[88:]))) * scale
	}
	s.PostScript = postScriptName(face)
	return s, nil
}

// components returns the byte offsets of the glyph-index fields of a composite glyph.
func components(d []byte) []int {
	var out []int
	p := 10
	for p+4 <= len(d) {
		flags := binary.BigEndian.Uint16(d[p:])
		out = append(out, p+2)
		p += 4
		if flags&0x0001 != 0 {
			p += 4
		} else {
			p += 2
		}
		switch {
		case flags&0x0008 != 0:
			p += 2
		case flags&0x0040 != 0:
			p += 4
		case flags&0x0080 != 0:
			p += 8
		}
		if flags&0x0020 == 0 {
			break
		}
	}
	return out
}

func postScriptName(face *Face) string {
	var buf sfnt.Buffer
	name, err := face.Font.Name(&buf, sfnt.NameIDPostScript)
	if err != nil || name == "" {
		name = "KmbFont"
	}
	clean := make([]rune, 0, len(name))
	for _, r := range name {
		if r > 32 && r < 127 && r != '/' && r != '(' && r != ')' && r != '[' && r != ']' && r != '<' && r != '>' && r != '{' && r != '}' && r != '%' {
			clean = append(clean, r)
		}
	}
	return string(clean)
}

// Advance is the advance width of r in font units per em = 1000.
func (f *Face) Advance(r rune) float64 {
	var buf sfnt.Buffer
	g, err := f.Font.GlyphIndex(&buf, r)
	if err != nil {
		return 500
	}
	upem := f.Font.UnitsPerEm()
	adv, err := f.Font.GlyphAdvance(&buf, g, fixed.I(int(upem)), font.HintingNone)
	if err != nil {
		return 500
	}
	return float64(adv) / 64 * 1000 / float64(upem)
}

// TextWidth is the width of text in points at size.
func (f *Face) TextWidth(text string, size float64) float64 {
	w := 0.0
	for _, r := range text {
		w += f.Advance(r)
	}
	return w * size / 1000
}
