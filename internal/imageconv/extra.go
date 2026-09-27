package imageconv

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"kuymediabox/internal/fonts"
)

// ---- rotate, flip, crop ------------------------------------------------------------------

// parseRatio reads "16:9" → (16, 9).
func parseRatio(s string) (float64, float64, bool) {
	a, b, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok {
		return 0, 0, false
	}
	x, err1 := strconv.ParseFloat(a, 64)
	y, err2 := strconv.ParseFloat(b, 64)
	if err1 != nil || err2 != nil || x <= 0 || y <= 0 || x > 100 || y > 100 {
		return 0, 0, false
	}
	return x, y, true
}

// cropSize is the largest centred area of w×h with the ratio (0, 0 when no crop).
func cropSize(w, h int, ratio string) (int, int) {
	rx, ry, ok := parseRatio(ratio)
	if !ok || w <= 0 || h <= 0 {
		return 0, 0
	}
	want := rx / ry
	if float64(w)/float64(h) > want {
		return max(1, int(math.Round(float64(h)*want))), h
	}
	return w, max(1, int(math.Round(float64(w)/want)))
}

// transform applies rotation, mirroring and the aspect-ratio crop.
func transform(img image.Image, o Options) image.Image {
	switch o.Rotate {
	case 90:
		img = imaging.Rotate270(img) // imaging rotates counter-clockwise
	case 180:
		img = imaging.Rotate180(img)
	case 270:
		img = imaging.Rotate90(img)
	}
	if o.FlipH {
		img = imaging.FlipH(img)
	}
	if o.FlipV {
		img = imaging.FlipV(img)
	}
	if c := o.CropBox; c.W > 0 {
		b := img.Bounds()
		r := image.Rect(int(c.X*float64(b.Dx())), int(c.Y*float64(b.Dy())), int((c.X+c.W)*float64(b.Dx())), int((c.Y+c.H)*float64(b.Dy())))
		if r.Dx() >= 1 && r.Dy() >= 1 {
			img = imaging.Crop(img, r.Add(b.Min))
		}
	}
	b := img.Bounds()
	if cw, ch := cropSize(b.Dx(), b.Dy(), o.Crop); cw > 0 && (cw != b.Dx() || ch != b.Dy()) {
		img = imaging.CropCenter(img, cw, ch)
	}
	return img
}

// ---- ICO with several sizes -------------------------------------------------------------

var icoAllowed = map[int]bool{16: true, 20: true, 24: true, 32: true, 40: true, 48: true, 64: true, 96: true, 128: true, 256: true}

func normalizeIcoSizes(in []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, s := range in {
		if icoAllowed[s] && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out
}

// encodeICOSizes writes an ICO holding the picture at every size (square, transparent padding).
// Large sizes are stored as PNG, small ones as 32-bit BMP for old programs.
func encodeICOSizes(w io.Writer, img image.Image, sizes []int) error {
	b := img.Bounds()
	side := max(b.Dx(), b.Dy())
	square := image.NewNRGBA(image.Rect(0, 0, side, side))
	draw.Draw(square, image.Rect((side-b.Dx())/2, (side-b.Dy())/2, (side-b.Dx())/2+b.Dx(), (side-b.Dy())/2+b.Dy()), img, b.Min, draw.Src)

	var payloads [][]byte
	for _, s := range sizes {
		im := imaging.Resize(square, s, s, imaging.Lanczos)
		var buf bytes.Buffer
		if s >= 64 {
			if err := png.Encode(&buf, im); err != nil {
				return err
			}
		} else {
			writeIcoBMP(&buf, im)
		}
		payloads = append(payloads, buf.Bytes())
	}
	header := []byte{0, 0, 1, 0, byte(len(sizes)), byte(len(sizes) >> 8)}
	dir := make([]byte, 16*len(sizes))
	off := 6 + 16*len(sizes)
	for i, s := range sizes {
		e := dir[16*i:]
		if s < 256 {
			e[0], e[1] = byte(s), byte(s)
		}
		binary.LittleEndian.PutUint16(e[4:], 1)
		binary.LittleEndian.PutUint16(e[6:], 32)
		binary.LittleEndian.PutUint32(e[8:], uint32(len(payloads[i])))
		binary.LittleEndian.PutUint32(e[12:], uint32(off))
		off += len(payloads[i])
	}
	for _, part := range append([][]byte{header, dir}, payloads...) {
		if _, err := w.Write(part); err != nil {
			return err
		}
	}
	return nil
}

// writeIcoBMP writes a 32-bit BGRA DIB with an AND mask, as ICO files expect.
func writeIcoBMP(buf *bytes.Buffer, im *image.NRGBA) {
	w, h := im.Bounds().Dx(), im.Bounds().Dy()
	maskRow := ((w + 31) / 32) * 4
	hdr := make([]byte, 40)
	binary.LittleEndian.PutUint32(hdr[0:], 40)
	binary.LittleEndian.PutUint32(hdr[4:], uint32(w))
	binary.LittleEndian.PutUint32(hdr[8:], uint32(2*h))
	binary.LittleEndian.PutUint16(hdr[12:], 1)
	binary.LittleEndian.PutUint16(hdr[14:], 32)
	binary.LittleEndian.PutUint32(hdr[20:], uint32(w*h*4+maskRow*h))
	buf.Write(hdr)
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := im.NRGBAAt(x, y)
			buf.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	mask := make([]byte, maskRow)
	for y := h - 1; y >= 0; y-- {
		for i := range mask {
			mask[i] = 0
		}
		for x := 0; x < w; x++ {
			if im.NRGBAAt(x, y).A == 0 {
				mask[x/8] |= 0x80 >> (x % 8)
			}
		}
		buf.Write(mask)
	}
}

// ---- watermark -------------------------------------------------------------------------

// Watermark stamps text or a logo on every picture.
type Watermark struct {
	Enabled  bool    `json:"enabled"`
	Type     string  `json:"type"`     // text | image
	Text     string  `json:"text"`     // text watermark
	Bold     bool    `json:"bold"`     //
	Color    string  `json:"color"`    // #rrggbb
	Image    string  `json:"image"`    // logo file
	Size     float64 `json:"size"`     // text: height in % of the short side; image: width in % of the picture width
	Opacity  float64 `json:"opacity"`  // 0..1
	Angle    float64 `json:"angle"`    // degrees, counter-clockwise
	Position string  `json:"position"` // tl t tr l c r bl b br | tile
	Margin   float64 `json:"margin"`   // % of the short side
}

func (w *Watermark) normalize() {
	if w.Type != "image" {
		w.Type = "text"
	}
	if w.Size <= 0 {
		w.Size = 5
		if w.Type == "image" {
			w.Size = 20
		}
	}
	w.Size = math.Min(w.Size, 100)
	if w.Opacity <= 0 || w.Opacity > 1 {
		w.Opacity = 0.5
	}
	if _, err := parseHex(w.Color); err != nil {
		w.Color = "#ffffff"
	}
	// The position grid of the UI sends tc/ml/mc/mr/bc.
	if p, ok := map[string]string{"tc": "t", "ml": "l", "mc": "c", "mr": "r", "bc": "b"}[w.Position]; ok {
		w.Position = p
	}
	switch w.Position {
	case "tl", "t", "tr", "l", "c", "r", "bl", "b", "br", "tile":
	default:
		w.Position = "br"
	}
	if w.Margin < 0 || w.Margin > 30 {
		w.Margin = 3
	}
	if w.Enabled && ((w.Type == "text" && strings.TrimSpace(w.Text) == "") || (w.Type == "image" && w.Image == "")) {
		w.Enabled = false
	}
}

// Normalize fixes invalid watermark settings (and turns it off when it has nothing to draw).
func (w *Watermark) Normalize() { w.normalize() }

// WatermarkLayer draws the watermark on a transparent width×height picture and saves it as
// PNG (the Video page lays it over every frame).
func WatermarkLayer(w Watermark, width, height int, out, ffmpegPath string) error {
	w.normalize()
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 {
		return errors.New("invalid watermark size")
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	img, err := applyWatermark(context.Background(), canvas, w, ffmpegPath)
	if err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// renderText draws text in a tight transparent image; px is the font size in pixels.
// Every line gets a font that covers it; Arabic, Hebrew and Indic lines are shaped (joined
// forms, conjuncts, right-to-left).
func renderText(text string, px float64, bold bool, col color.Color) (*image.NRGBA, error) {
	type lineFont struct {
		sf *opentype.Font
		fc font.Face
		m  font.Metrics
	}
	faces := map[string]*lineFont{}
	defer func() {
		for _, lf := range faces {
			lf.fc.Close()
		}
	}()
	get := func(face *fonts.Face) (*lineFont, error) {
		if lf, ok := faces[face.Name]; ok {
			return lf, nil
		}
		sf, err := opentype.Parse(face.Data)
		if err != nil {
			return nil, err
		}
		fc, err := opentype.NewFace(sf, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingNone})
		if err != nil {
			return nil, err
		}
		lf := &lineFont{sf: sf, fc: fc, m: fc.Metrics()}
		faces[face.Name] = lf
		return lf, nil
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	type laid struct {
		lf     *lineFont
		shaped *fonts.Shaped
		w      int
	}
	all := make([]laid, len(lines))
	width, lineH, ascent := 0, 0, 0
	for i, l := range lines {
		face := fonts.For(l, bold)
		lf, err := get(face)
		if err != nil {
			return nil, err
		}
		all[i].lf = lf
		if fonts.NeedsShaping(l) {
			if sh, err := fonts.Shape(face, l); err == nil {
				all[i].shaped = &sh
				all[i].w = int(math.Ceil(sh.Width * px / 1000))
			}
		}
		if all[i].shaped == nil {
			all[i].w = font.MeasureString(lf.fc, l).Ceil()
		}
		width = max(width, all[i].w)
		lineH = max(lineH, (lf.m.Ascent + lf.m.Descent).Ceil())
		ascent = max(ascent, lf.m.Ascent.Ceil())
	}
	pad := int(px / 6)
	dst := image.NewNRGBA(image.Rect(0, 0, width+2*pad, lineH*len(lines)+2*pad))
	for i, l := range lines {
		x := pad + (width-all[i].w)/2
		base := pad + i*lineH + ascent
		if all[i].shaped != nil {
			drawShaped(dst, all[i].lf.sf, *all[i].shaped, float64(x), float64(base), px, col)
			continue
		}
		d := &font.Drawer{Dst: dst, Src: image.NewUniform(col), Face: all[i].lf.fc, Dot: fixed.P(x, base)}
		d.DrawString(l)
	}
	return dst, nil
}

// drawShaped rasterises shaped glyphs by glyph id (the font's character map can't reach
// ligatures and contextual forms).
func drawShaped(dst *image.NRGBA, f *opentype.Font, sh fonts.Shaped, x0, base, px float64, col color.Color) {
	var buf sfnt.Buffer
	b := dst.Bounds()
	r := vector.NewRasterizer(b.Dx(), b.Dy())
	ppem := fixed.Int26_6(math.Round(px * 64))
	for _, g := range sh.Glyphs {
		segs, err := f.LoadGlyph(&buf, sfnt.GlyphIndex(g.GID), ppem, nil)
		if err != nil {
			continue
		}
		ox, oy := float32(x0+g.X*px/1000), float32(base-g.Y*px/1000)
		pt := func(p fixed.Point26_6) (float32, float32) { return ox + float32(p.X)/64, oy + float32(p.Y)/64 }
		for _, s := range segs {
			switch s.Op {
			case sfnt.SegmentOpMoveTo:
				r.MoveTo(pt(s.Args[0]))
			case sfnt.SegmentOpLineTo:
				r.LineTo(pt(s.Args[0]))
			case sfnt.SegmentOpQuadTo:
				ax, ay := pt(s.Args[0])
				bx, by := pt(s.Args[1])
				r.QuadTo(ax, ay, bx, by)
			case sfnt.SegmentOpCubeTo:
				ax, ay := pt(s.Args[0])
				bx, by := pt(s.Args[1])
				cx, cy := pt(s.Args[2])
				r.CubeTo(ax, ay, bx, by, cx, cy)
			}
		}
		r.ClosePath()
	}
	r.Draw(dst, b, image.NewUniform(col), image.Point{})
}

func applyWatermark(ctx context.Context, img image.Image, w Watermark, ffmpegPath string) (image.Image, error) {
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	short := float64(min(W, H))
	var layer image.Image
	if w.Type == "image" {
		logo, err := decode(ctx, w.Image, true, ffmpegPath)
		if err != nil {
			return nil, err
		}
		lw := max(1, int(float64(W)*w.Size/100))
		layer = imaging.Resize(logo, lw, 0, imaging.Lanczos)
	} else {
		col, _ := parseHex(w.Color)
		t, err := renderText(w.Text, math.Max(6, short*w.Size/100), w.Bold, col)
		if err != nil {
			return nil, err
		}
		layer = t
	}
	if w.Angle != 0 {
		layer = imaging.Rotate(layer, w.Angle, color.Transparent)
	}
	dst := imaging.Clone(img)
	mask := image.NewUniform(color.Alpha{A: uint8(math.Round(w.Opacity * 255))})
	lb := layer.Bounds()
	lw, lh := lb.Dx(), lb.Dy()
	put := func(x, y int) {
		r := image.Rect(x, y, x+lw, y+lh)
		draw.DrawMask(dst, r, layer, lb.Min, mask, image.Point{}, draw.Over)
	}
	margin := int(short * w.Margin / 100)
	if w.Position == "tile" {
		gapX, gapY := max(lw/2, 8), max(lh, 8)
		for y, row := -lh/2, 0; y < H; y, row = y+lh+gapY, row+1 {
			off := 0
			if row%2 == 1 {
				off = (lw + gapX) / 2
			}
			for x := -off; x < W; x += lw + gapX {
				put(x, y)
			}
		}
		return dst, nil
	}
	x := map[byte]int{'l': margin, 'c': (W - lw) / 2, 'r': W - lw - margin}
	y := map[byte]int{'t': margin, 'c': (H - lh) / 2, 'b': H - lh - margin}
	var hx, vy byte = 'c', 'c'
	switch w.Position {
	case "tl":
		hx, vy = 'l', 't'
	case "t":
		vy = 't'
	case "tr":
		hx, vy = 'r', 't'
	case "l":
		hx = 'l'
	case "r":
		hx = 'r'
	case "bl":
		hx, vy = 'l', 'b'
	case "b":
		vy = 'b'
	case "br":
		hx, vy = 'r', 'b'
	}
	put(x[hx], y[vy])
	return dst, nil
}

// ---- EXIF ------------------------------------------------------------------------------

// readExif returns the raw EXIF block ("Exif\0\0" + TIFF data) of a JPEG or PNG file.
func readExif(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 64<<20))
	if err != nil {
		return nil
	}
	switch {
	case len(data) > 4 && data[0] == 0xFF && data[1] == 0xD8:
		for p := 2; p+4 <= len(data); {
			if data[p] != 0xFF {
				return nil
			}
			marker := data[p+1]
			if marker == 0xD9 || marker == 0xDA {
				return nil
			}
			n := int(binary.BigEndian.Uint16(data[p+2:]))
			if p+2+n > len(data) || n < 2 {
				return nil
			}
			seg := data[p+4 : p+2+n]
			if marker == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
				return append([]byte(nil), seg...)
			}
			p += 2 + n
		}
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		for p := 8; p+12 <= len(data); {
			n := int(binary.BigEndian.Uint32(data[p:]))
			if p+12+n > len(data) {
				return nil
			}
			if string(data[p+4:p+8]) == "eXIf" {
				return append([]byte("Exif\x00\x00"), data[p+8:p+8+n]...)
			}
			p += 12 + n
		}
	}
	return nil
}

// resetOrientation sets the EXIF orientation tag (0x0112) of IFD0 to 1.
func resetOrientation(exif []byte) []byte {
	out := append([]byte(nil), exif...)
	t := out[6:]
	if len(t) < 8 {
		return out
	}
	var bo binary.ByteOrder = binary.BigEndian
	if string(t[:2]) == "II" {
		bo = binary.LittleEndian
	}
	ifd := int(bo.Uint32(t[4:]))
	if ifd+2 > len(t) {
		return out
	}
	n := int(bo.Uint16(t[ifd:]))
	for i := 0; i < n; i++ {
		e := ifd + 2 + 12*i
		if e+12 > len(t) {
			break
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			bo.PutUint16(t[e+8:], 1)
		}
	}
	return out
}

// embedExif puts an EXIF block into encoded JPEG or PNG data.
func embedExif(data []byte, format string, exif []byte) []byte {
	switch format {
	case "jpg":
		if len(data) < 2 || len(exif)+2 > 0xFFFF {
			return data
		}
		seg := []byte{0xFF, 0xE1, 0, 0}
		binary.BigEndian.PutUint16(seg[2:], uint16(len(exif)+2))
		out := append([]byte{}, data[:2]...)
		out = append(out, seg...)
		out = append(out, exif...)
		return append(out, data[2:]...)
	case "png":
		body := exif[6:]
		if len(data) < 33 {
			return data
		}
		chunk := make([]byte, 8, 12+len(body))
		binary.BigEndian.PutUint32(chunk, uint32(len(body)))
		copy(chunk[4:], "eXIf")
		chunk = append(chunk, body...)
		crc := crc32.ChecksumIEEE(chunk[4:])
		chunk = binary.BigEndian.AppendUint32(chunk, crc)
		// After the signature (8) and IHDR (25 bytes).
		out := append([]byte{}, data[:33]...)
		out = append(out, chunk...)
		return append(out, data[33:]...)
	}
	return data
}
