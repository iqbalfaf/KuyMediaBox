package imageconv

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"strings"

	"github.com/disintegration/imaging"
)

// SheetCell is one picture of a contact sheet with its caption (e.g. a timestamp).
type SheetCell struct {
	Path  string
	Label string
}

// ContactSheet lays pictures out in a grid of cols columns under a header (one or two lines
// of text) and saves it as JPG or PNG. Every cell is thumbW pixels wide.
func ContactSheet(cells []SheetCell, header []string, cols, thumbW int, out, format string) error {
	if len(cells) == 0 || cols <= 0 {
		return errors.New("no pictures")
	}
	thumbs := make([]image.Image, len(cells))
	thumbH := 0
	for i, c := range cells {
		img, err := imaging.Open(c.Path)
		if err != nil {
			return err
		}
		t := imaging.Resize(img, thumbW, 0, imaging.Lanczos)
		thumbs[i] = t
		thumbH = max(thumbH, t.Bounds().Dy())
	}
	gap := max(4, thumbW/40)
	rows := (len(cells) + cols - 1) / cols
	fontPx := float64(max(12, thumbW/18))
	headPx := fontPx * 1.25
	var headLines []*image.NRGBA
	headH := 0
	for i, line := range header {
		if strings.TrimSpace(line) == "" {
			continue
		}
		px, col := headPx, color.Color(color.NRGBA{235, 237, 242, 255})
		if i > 0 {
			px, col = fontPx, color.NRGBA{160, 165, 178, 255}
		}
		t, err := renderText(line, px, i == 0, col)
		if err != nil {
			return err
		}
		headLines = append(headLines, t)
		headH += t.Bounds().Dy()
	}
	if headH > 0 {
		headH += gap * 2
	}
	W := cols*thumbW + (cols+1)*gap
	H := headH + rows*thumbH + (rows+1)*gap
	sheet := image.NewNRGBA(image.Rect(0, 0, W, H))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.NRGBA{18, 20, 26, 255}), image.Point{}, draw.Src)
	y := gap
	for _, t := range headLines {
		draw.Draw(sheet, t.Bounds().Add(image.Pt(gap, y)), t, image.Point{}, draw.Over)
		y += t.Bounds().Dy()
	}
	for i, t := range thumbs {
		cx := gap + (i%cols)*(thumbW+gap)
		cy := headH + gap + (i/cols)*(thumbH+gap)
		b := t.Bounds()
		off := (thumbH - b.Dy()) / 2
		draw.Draw(sheet, image.Rect(cx, cy+off, cx+b.Dx(), cy+off+b.Dy()), t, b.Min, draw.Src)
		if label := cells[i].Label; label != "" {
			txt, err := renderText(label, fontPx*0.85, true, color.White)
			if err != nil {
				return err
			}
			tb := txt.Bounds()
			bx, by := cx+b.Dx()-tb.Dx()-gap/2, cy+off+b.Dy()-tb.Dy()-gap/2
			bg := image.Rect(bx, by, bx+tb.Dx(), by+tb.Dy())
			draw.DrawMask(sheet, bg, image.NewUniform(color.Black), image.Point{}, image.NewUniform(color.Alpha{A: 150}), image.Point{}, draw.Over)
			draw.Draw(sheet, bg, txt, tb.Min, draw.Over)
		}
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if format == "png" {
		err = png.Encode(f, sheet)
	} else {
		err = jpeg.Encode(f, sheet, &jpeg.Options{Quality: 90})
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
