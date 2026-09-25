package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/font"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"golang.org/x/text/encoding/charmap"

	"kuymediabox/internal/fonts"
)

// Item is something drawn on top of a page. Coordinates are points in display space: origin
// at the top-left of the page as it is shown (rotation applied), y growing downwards.
type Item struct {
	Kind    string       `json:"kind"` // text | rect | ellipse | line | ink | image
	X       float64      `json:"x"`    // text: baseline start (or anchor, see Align); shapes: left
	Y       float64      `json:"y"`    // text: baseline; shapes: top
	W       float64      `json:"w"`
	H       float64      `json:"h"`
	X2      float64      `json:"x2"` // line end
	Y2      float64      `json:"y2"`
	Points  [][2]float64 `json:"points"` // ink
	Text    string       `json:"text"`
	Size    float64      `json:"size"` // font size in points
	Bold    bool         `json:"bold"`
	Align   string       `json:"align"`  // left | center | right (relative to X)
	Color   string       `json:"color"`  // stroke / text colour #rrggbb
	Fill    string       `json:"fill"`   // fill colour, "" for none
	Stroke  float64      `json:"stroke"` // line width in points
	Opacity float64      `json:"opacity"`
	Angle   float64      `json:"angle"` // counter-clockwise rotation in degrees around (X, Y)
	Image   []byte       `json:"-"`     // PNG or JPEG data
	Hidden  bool         `json:"-"`     // invisible text (OCR layer)
	FitW    float64      `json:"-"`     // stretch hidden text to this width
}

const (
	fontRegular = "KmbHelv"
	fontBold    = "KmbHelvB"
)

// page is the part of a page dict the overlay needs.
type pageRef struct {
	dict types.Dict
	geom Geom
}

func pageGeom(ctx *model.Context, pageNr int) (types.Dict, Geom, error) {
	d, _, inh, err := ctx.PageDict(pageNr, false)
	if err != nil {
		return nil, Geom{}, err
	}
	if d == nil {
		return nil, Geom{}, fmt.Errorf("page %d missing", pageNr)
	}
	box := inh.MediaBox
	if inh.CropBox != nil {
		box = inh.CropBox
	}
	if box == nil {
		box = types.NewRectangle(0, 0, 595.28, 841.89)
	}
	rot := ((inh.Rotate % 360) + 360) % 360
	return d, Geom{X0: box.LL.X, Y0: box.LL.Y, X1: box.UR.X, Y1: box.UR.Y, Rotate: rot}, nil
}

// displayMatrix maps display space (origin bottom-left, y up) to user space.
func displayMatrix(g Geom) string {
	switch g.Rotate {
	case 90:
		return fmt.Sprintf("0 1 -1 0 %s %s cm", num(g.X1), num(g.Y0))
	case 180:
		return fmt.Sprintf("-1 0 0 -1 %s %s cm", num(g.X1), num(g.Y1))
	case 270:
		return fmt.Sprintf("0 -1 1 0 %s %s cm", num(g.X0), num(g.Y1))
	}
	return fmt.Sprintf("1 0 0 1 %s %s cm", num(g.X0), num(g.Y0))
}

func num(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "0"
	}
	s := strconv.FormatFloat(f, 'f', 3, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

func rgb(hex string) (r, g, b float64, ok bool) {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return float64(v>>16&255) / 255, float64(v>>8&255) / 255, float64(v&255) / 255, true
}

// winAnsi encodes text for the standard Helvetica font; unsupported characters become '?'.
func winAnsi(s string) []byte {
	enc := charmap.Windows1252.NewEncoder()
	var out []byte
	for _, r := range s {
		b, err := enc.Bytes([]byte(string(r)))
		if err != nil || len(b) != 1 {
			out = append(out, '?')
			continue
		}
		out = append(out, b[0])
	}
	return out
}

func pdfString(b []byte) string {
	var sb strings.Builder
	sb.WriteByte('(')
	for _, c := range b {
		switch c {
		case '(', ')', '\\':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		case '\r':
			sb.WriteString(`\r`)
		case '\n':
			sb.WriteString(`\n`)
		default:
			sb.WriteByte(c)
		}
	}
	sb.WriteByte(')')
	return sb.String()
}

// TextWidth is the width of text in points at size: Helvetica (bold) for WinAnsi text,
// otherwise the Windows font that will be embedded for it.
func TextWidth(text string, size float64, bold bool) float64 {
	if !winAnsiOK(text) {
		return fonts.For(text, bold).TextWidth(text, size)
	}
	name := "Helvetica"
	if bold {
		name = "Helvetica-Bold"
	}
	// Measure what will actually be drawn (unsupported characters become '?').
	dec, _ := charmap.Windows1252.NewDecoder().Bytes(winAnsi(text))
	w, err := font.TextWidthFloat(string(dec), name, size)
	if err != nil {
		return float64(len([]rune(text))) * size * 0.55
	}
	return w
}

// overlayBuilder collects content operators and the resources they need.
type overlayBuilder struct {
	ctx    *model.Context
	buf    bytes.Buffer
	dh     float64 // display height
	gs     map[string]string
	fonts  map[string]bool
	ufonts map[string]*uniFont // embedded fonts for non-WinAnsi text, by font file
	xobj   map[string]*types.IndirectRef
	seq    int
}

func (b *overlayBuilder) opacity(a float64) string {
	if a <= 0 || a >= 1 {
		return ""
	}
	key := num(a)
	if name, ok := b.gs[key]; ok {
		return name
	}
	name := fmt.Sprintf("KmbGS%d", len(b.gs))
	b.gs[key] = name
	return name
}

func (b *overlayBuilder) setColor(hex string, fill bool) bool {
	r, g, bl, ok := rgb(hex)
	if !ok {
		return false
	}
	op := "RG"
	if fill {
		op = "rg"
	}
	fmt.Fprintf(&b.buf, "%s %s %s %s\n", num(r), num(g), num(bl), op)
	return true
}

func (b *overlayBuilder) y(v float64) float64 { return b.dh - v }

func (b *overlayBuilder) add(it Item) error {
	b.buf.WriteString("q\n")
	defer b.buf.WriteString("Q\n")
	if gs := b.opacity(it.Opacity); gs != "" {
		fmt.Fprintf(&b.buf, "/%s gs\n", gs)
	}
	if it.Stroke > 0 {
		fmt.Fprintf(&b.buf, "%s w 1 J 1 j\n", num(it.Stroke))
	}
	switch it.Kind {
	case "text":
		return b.text(it)
	case "rect":
		b.paint(it, fmt.Sprintf("%s %s %s %s re\n", num(it.X), num(b.y(it.Y+it.H)), num(it.W), num(it.H)))
	case "ellipse":
		b.paint(it, ellipsePath(it.X+it.W/2, b.y(it.Y+it.H/2), it.W/2, it.H/2))
	case "line":
		if b.setColor(it.Color, false) {
			fmt.Fprintf(&b.buf, "%s %s m %s %s l S\n", num(it.X), num(b.y(it.Y)), num(it.X2), num(b.y(it.Y2)))
		}
	case "ink":
		if len(it.Points) == 0 || !b.setColor(it.Color, false) {
			return nil
		}
		for i, p := range it.Points {
			op := "l"
			if i == 0 {
				op = "m"
			}
			fmt.Fprintf(&b.buf, "%s %s %s\n", num(p[0]), num(b.y(p[1])), op)
		}
		if len(it.Points) == 1 {
			fmt.Fprintf(&b.buf, "%s %s l\n", num(it.Points[0][0]+0.01), num(b.y(it.Points[0][1])))
		}
		b.buf.WriteString("S\n")
	case "image":
		return b.image(it)
	default:
		return fmt.Errorf("unknown item %q", it.Kind)
	}
	return nil
}

func (b *overlayBuilder) paint(it Item, path string) {
	stroke := it.Color != "" && it.Stroke > 0
	fill := it.Fill != ""
	if fill {
		b.setColor(it.Fill, true)
	}
	if stroke {
		b.setColor(it.Color, false)
	}
	if !fill && !stroke {
		return
	}
	b.buf.WriteString(path)
	switch {
	case fill && stroke:
		b.buf.WriteString("B\n")
	case fill:
		b.buf.WriteString("f\n")
	default:
		b.buf.WriteString("S\n")
	}
}

func ellipsePath(cx, cy, rx, ry float64) string {
	const k = 0.5522847498
	ox, oy := rx*k, ry*k
	var s strings.Builder
	fmt.Fprintf(&s, "%s %s m\n", num(cx+rx), num(cy))
	fmt.Fprintf(&s, "%s %s %s %s %s %s c\n", num(cx+rx), num(cy+oy), num(cx+ox), num(cy+ry), num(cx), num(cy+ry))
	fmt.Fprintf(&s, "%s %s %s %s %s %s c\n", num(cx-ox), num(cy+ry), num(cx-rx), num(cy+oy), num(cx-rx), num(cy))
	fmt.Fprintf(&s, "%s %s %s %s %s %s c\n", num(cx-rx), num(cy-oy), num(cx-ox), num(cy-ry), num(cx), num(cy-ry))
	fmt.Fprintf(&s, "%s %s %s %s %s %s c\nh\n", num(cx+ox), num(cy-ry), num(cx+rx), num(cy-oy), num(cx+rx), num(cy))
	return s.String()
}

func (b *overlayBuilder) text(it Item) error {
	if strings.TrimSpace(it.Text) == "" {
		return nil
	}
	size := it.Size
	if size <= 0 {
		size = 12
	}
	name := fontRegular
	if it.Bold {
		name = fontBold
	}
	uf := b.uniFor(it)
	if uf != nil {
		name = uf.res
	} else {
		b.fonts[name] = true
	}
	color := it.Color
	if color == "" {
		color = "#000000"
	}
	// Position everything relative to the anchor so rotation turns around it.
	fmt.Fprintf(&b.buf, "1 0 0 1 %s %s cm\n", num(it.X), num(b.y(it.Y)))
	if it.Angle != 0 {
		a := it.Angle * math.Pi / 180
		c, s := math.Cos(a), math.Sin(a)
		fmt.Fprintf(&b.buf, "%s %s %s %s 0 0 cm\n", num(c), num(s), num(-s), num(c))
	}
	b.setColor(color, true)
	lines := strings.Split(strings.ReplaceAll(it.Text, "\r\n", "\n"), "\n")
	b.buf.WriteString("BT\n")
	fmt.Fprintf(&b.buf, "/%s %s Tf\n", name, num(size))
	if it.Hidden {
		b.buf.WriteString("3 Tr\n")
	}
	for i, line := range lines {
		var w float64
		if uf != nil {
			w = uf.sub.Width(line, size)
		} else {
			w = TextWidth(line, size, it.Bold)
		}
		x := 0.0
		switch it.Align {
		case "center":
			x = -w / 2
		case "right":
			x = -w
		}
		scale := 100.0
		if it.FitW > 0 && w > 0 {
			scale = it.FitW / w * 100
		}
		fmt.Fprintf(&b.buf, "%s Tz\n", num(scale))
		fmt.Fprintf(&b.buf, "1 0 0 1 %s %s Tm\n", num(x), num(-float64(i)*size*1.2))
		if uf != nil {
			fmt.Fprintf(&b.buf, "%s Tj\n", uf.hexGlyphs(line))
		} else {
			fmt.Fprintf(&b.buf, "%s Tj\n", pdfString(winAnsi(line)))
		}
	}
	b.buf.WriteString("ET\n")
	return nil
}

func (b *overlayBuilder) image(it Item) error {
	if len(it.Image) == 0 || it.W <= 0 || it.H <= 0 {
		return nil
	}
	ref, _, _, err := model.CreateImageResource(b.ctx.XRefTable, bytes.NewReader(it.Image))
	if err != nil {
		return fmt.Errorf("image: %w", err)
	}
	b.seq++
	name := fmt.Sprintf("KmbIm%d", b.seq)
	b.xobj[name] = ref
	// Anchor at the image centre so rotation keeps it in place.
	cx, cy := it.X+it.W/2, b.y(it.Y+it.H/2)
	fmt.Fprintf(&b.buf, "1 0 0 1 %s %s cm\n", num(cx), num(cy))
	if it.Angle != 0 {
		a := it.Angle * math.Pi / 180
		c, s := math.Cos(a), math.Sin(a)
		fmt.Fprintf(&b.buf, "%s %s %s %s 0 0 cm\n", num(c), num(s), num(-s), num(c))
	}
	fmt.Fprintf(&b.buf, "%s 0 0 %s %s %s cm /%s Do\n", num(it.W), num(it.H), num(-it.W/2), num(-it.H/2), name)
	return nil
}

// subDict returns (and attaches) a resource sub-dictionary such as /Font.
func subDict(ctx *model.Context, res types.Dict, key string) (types.Dict, error) {
	o, ok := res.Find(key)
	if !ok || o == nil {
		d := types.NewDict()
		res.Insert(key, d)
		return d, nil
	}
	d, err := ctx.DereferenceDict(o)
	if err != nil {
		return nil, err
	}
	if d == nil {
		d = types.NewDict()
		res.Update(key, d)
	}
	return d, nil
}

// pageResources returns the page's own resource dict, copying inherited resources if needed.
func pageResources(ctx *model.Context, page types.Dict, pageNr int) (types.Dict, error) {
	if o, ok := page.Find("Resources"); ok && o != nil {
		d, err := ctx.DereferenceDict(o)
		if err != nil {
			return nil, err
		}
		if d != nil {
			return d, nil
		}
	}
	_, _, inh, err := ctx.PageDict(pageNr, false)
	if err != nil {
		return nil, err
	}
	res := types.NewDict()
	for k, v := range inh.Resources {
		res.Insert(k, v)
	}
	page.Update("Resources", res)
	return res, nil
}

func newStream(ctx *model.Context, content []byte) (*types.IndirectRef, error) {
	sd, err := ctx.XRefTable.NewStreamDictForBuf(content)
	if err != nil {
		return nil, err
	}
	if err := sd.Encode(); err != nil {
		return nil, err
	}
	return ctx.XRefTable.IndRefForNewObject(*sd)
}

// drawOnPage adds items on top of page pageNr (1-based).
func drawOnPage(ctx *model.Context, pageNr int, items []Item, under bool) error {
	if len(items) == 0 {
		return nil
	}
	page, g, err := pageGeom(ctx, pageNr)
	if err != nil {
		return err
	}
	_, dh := g.DisplaySize()
	b := &overlayBuilder{ctx: ctx, dh: dh, gs: map[string]string{}, fonts: map[string]bool{}, ufonts: map[string]*uniFont{}, xobj: map[string]*types.IndirectRef{}}
	if err := b.collectUnicode(items); err != nil {
		return err
	}
	b.buf.WriteString("q\n" + displayMatrix(g) + "\n")
	for _, it := range items {
		if err := b.add(it); err != nil {
			return err
		}
	}
	b.buf.WriteString("Q\n")

	res, err := pageResources(ctx, page, pageNr)
	if err != nil {
		return err
	}
	if len(b.fonts) > 0 || len(b.ufonts) > 0 {
		fd, err := subDict(ctx, res, "Font")
		if err != nil {
			return err
		}
		for _, uf := range b.ufonts {
			ref, err := uf.fontDict(ctx)
			if err != nil {
				return err
			}
			fd.Update(uf.res, *ref)
		}
		for name := range b.fonts {
			base := "Helvetica"
			if name == fontBold {
				base = "Helvetica-Bold"
			}
			f := types.NewDict()
			f.InsertName("Type", "Font")
			f.InsertName("Subtype", "Type1")
			f.InsertName("BaseFont", base)
			f.InsertName("Encoding", "WinAnsiEncoding")
			ref, err := ctx.XRefTable.IndRefForNewObject(f)
			if err != nil {
				return err
			}
			fd.Update(name, *ref)
		}
	}
	if len(b.gs) > 0 {
		gd, err := subDict(ctx, res, "ExtGState")
		if err != nil {
			return err
		}
		for alpha, name := range b.gs {
			a, _ := strconv.ParseFloat(alpha, 64)
			d := types.NewDict()
			d.InsertName("Type", "ExtGState")
			d.Insert("ca", types.Float(a))
			d.Insert("CA", types.Float(a))
			gd.Update(name, d)
		}
	}
	if len(b.xobj) > 0 {
		xd, err := subDict(ctx, res, "XObject")
		if err != nil {
			return err
		}
		for name, ref := range b.xobj {
			xd.Update(name, *ref)
		}
	}

	// Wrap the old content in q … Q so its graphics state can't leak into the overlay.
	var contents types.Array
	if o, ok := page.Find("Contents"); ok && o != nil {
		switch v := o.(type) {
		case types.IndirectRef:
			obj, err := ctx.Dereference(v)
			if err != nil {
				return err
			}
			if arr, isArr := obj.(types.Array); isArr {
				contents = append(contents, arr...)
			} else {
				contents = append(contents, v)
			}
		case types.Array:
			contents = append(contents, v...)
		default:
			return errors.New("unexpected page contents")
		}
	}
	var first, last []byte
	if under {
		// Draw the overlay first, then the original content on top of it.
		first = append(append([]byte{}, b.buf.Bytes()...), "\nq\n"...)
		last = []byte("\nQ\n")
	} else {
		first = []byte("q\n")
		last = append([]byte("\nQ\n"), b.buf.Bytes()...)
	}
	pre, err := newStream(ctx, first)
	if err != nil {
		return err
	}
	post, err := newStream(ctx, last)
	if err != nil {
		return err
	}
	all := types.Array{*pre}
	all = append(all, contents...)
	all = append(all, *post)
	page.Update("Contents", all)
	return nil
}

// DrawItems adds items (per 1-based page) to a PDF and writes the result to out.
func DrawItems(in Input, items map[int][]Item, out string) error {
	ctx, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	for p, list := range items {
		if p < 1 || p > ctx.PageCount {
			continue
		}
		if err := drawOnPage(ctx, p, list, false); err != nil {
			return fmt.Errorf("page %d: %w", p, err)
		}
	}
	return writeCtx(ctx, out)
}

// DisplaySizes returns the displayed size of every page (points).
func DisplaySizes(ctx *model.Context) ([]PageSize, error) {
	out := make([]PageSize, ctx.PageCount)
	for p := 1; p <= ctx.PageCount; p++ {
		_, g, err := pageGeom(ctx, p)
		if err != nil {
			return nil, err
		}
		w, h := g.DisplaySize()
		out[p-1] = PageSize{w, h}
	}
	return out, nil
}
