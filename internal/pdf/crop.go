package pdf

import (
	"errors"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

	"kuymediabox/internal/i18n"
)

// FromDisplay maps normalized display coordinates (0..1, origin top-left) to user space.
func (g Geom) FromDisplay(u, v float64) (x, y float64) {
	w, h := g.X1-g.X0, g.Y1-g.Y0
	switch g.Rotate {
	case 90:
		return g.X0 + v*w, g.Y0 + u*h
	case 180:
		return g.X1 - u*w, g.Y0 + v*h
	case 270:
		return g.X1 - v*w, g.Y1 - u*h
	}
	return g.X0 + u*w, g.Y1 - v*h
}

// Rect is a rectangle in normalized display coordinates of one page (1-based).
type Rect struct {
	Page int     `json:"page"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
}

// CropOptions control cropping.
type CropOptions struct {
	Mode   string  `json:"mode"` // margins | box
	Top    float64 `json:"top"`  // millimetres, for margins
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
	Box    Rect    `json:"box"`   // for box (page ignored)
	Pages  string  `json:"pages"` // "" = every page
}

// Crop sets the visible area of pages.
func Crop(in Input, o CropOptions, out string) error {
	ctx, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	pages, err := selected(o.Pages, ctx.PageCount)
	if err != nil {
		return err
	}
	const mm = 72 / 25.4
	for _, p := range pages {
		page, g, err := pageGeom(ctx, p)
		if err != nil {
			return err
		}
		var u0, v0, u1, v1 float64
		if o.Mode == "box" {
			u0, v0, u1, v1 = o.Box.X, o.Box.Y, o.Box.X+o.Box.W, o.Box.Y+o.Box.H
		} else {
			dw, dh := g.DisplaySize()
			u0, v0 = o.Left*mm/dw, o.Top*mm/dh
			u1, v1 = 1-o.Right*mm/dw, 1-o.Bottom*mm/dh
		}
		u0, v0 = clamp01(u0), clamp01(v0)
		u1, v1 = clamp01(u1), clamp01(v1)
		if u1-u0 < 0.02 || v1-v0 < 0.02 {
			return errors.New(i18n.L("area potong terlalu kecil", "the crop area is too small"))
		}
		x0, y0 := g.FromDisplay(u0, v0)
		x1, y1 := g.FromDisplay(u1, v1)
		r := types.NewRectangle(min(x0, x1), min(y0, y1), max(x0, x1), max(y0, y1))
		page.Update("CropBox", r.Array())
		// Keep the other boxes inside the new crop box.
		for _, k := range []string{"TrimBox", "BleedBox", "ArtBox"} {
			page.Delete(k)
		}
	}
	return writeCtx(ctx, out)
}

func clamp01(f float64) float64 { return max(0, min(1, f)) }
