package pdf

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg" // image config for watermark images
	_ "image/png"
	"math"
	"os"
	"strconv"
	"strings"

	"kuymediabox/internal/i18n"
)

// WatermarkOptions control the watermark tool.
type WatermarkOptions struct {
	Type     string  `json:"type"` // text | image
	Text     string  `json:"text"`
	Size     float64 `json:"size"`
	Bold     bool    `json:"bold"`
	Color    string  `json:"color"`
	Image    string  `json:"image"` // path of a PNG/JPEG
	Scale    float64 `json:"scale"` // image width as percent of the page width
	Opacity  float64 `json:"opacity"`
	Angle    float64 `json:"angle"`
	Position string  `json:"position"` // tl tc tr ml mc mr bl bc br | tile
	Pages    string  `json:"pages"`    // "" = every page
	Under    bool    `json:"under"`    // below the page content
}

// selected returns the 1-based pages chosen by sel (every page when empty).
func selected(sel string, n int) ([]int, error) {
	if strings.TrimSpace(sel) == "" {
		return allPages(n), nil
	}
	return ParsePages(sel, n)
}

func anchor(pos string, dw, dh, w, h, margin float64) (x, y float64) {
	// x, y is the centre of the stamp.
	if len(pos) != 2 {
		pos = "mc"
	}
	switch pos[1] {
	case 'l':
		x = margin + w/2
	case 'r':
		x = dw - margin - w/2
	default:
		x = dw / 2
	}
	switch pos[0] {
	case 't':
		y = margin + h/2
	case 'b':
		y = dh - margin - h/2
	default:
		y = dh / 2
	}
	return x, y
}

// rotatedBox is the bounding box of a w×h box turned by deg.
func rotatedBox(w, h, deg float64) (float64, float64) {
	a := deg * math.Pi / 180
	c, s := math.Abs(math.Cos(a)), math.Abs(math.Sin(a))
	return w*c + h*s, w*s + h*c
}

// Watermark stamps text or an image on the selected pages.
func Watermark(in Input, o WatermarkOptions, out string) error {
	ctx, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	pages, err := selected(o.Pages, ctx.PageCount)
	if err != nil {
		return err
	}
	if o.Opacity <= 0 || o.Opacity > 1 {
		o.Opacity = 0.5
	}
	var img []byte
	var iw, ih int
	if o.Type == "image" {
		if img, err = os.ReadFile(o.Image); err != nil {
			return errors.New(i18n.L("gambar watermark tidak bisa dibuka", "the watermark image can't be opened"))
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(img))
		if err != nil {
			return errors.New(i18n.L("gambar watermark harus PNG atau JPG", "the watermark image must be PNG or JPG"))
		}
		iw, ih = cfg.Width, cfg.Height
	} else if strings.TrimSpace(o.Text) == "" {
		return errors.New(i18n.L("isi teks watermark", "enter the watermark text"))
	}
	if o.Size <= 0 {
		o.Size = 48
	}
	if o.Scale <= 0 {
		o.Scale = 40
	}
	sizes, err := DisplaySizes(ctx)
	if err != nil {
		return err
	}
	for _, p := range pages {
		dw, dh := sizes[p-1].W, sizes[p-1].H
		var items []Item
		// Stamp size before rotation.
		var sw, sh float64
		if img != nil {
			sw = dw * o.Scale / 100
			sh = sw * float64(ih) / float64(iw)
		} else {
			sw = TextWidth(o.Text, o.Size, o.Bold)
			sh = o.Size * 0.72
		}
		bw, bh := rotatedBox(sw, sh, o.Angle)
		place := func(cx, cy float64) {
			if img != nil {
				items = append(items, Item{Kind: "image", X: cx - sw/2, Y: cy - sh/2, W: sw, H: sh, Image: img, Opacity: o.Opacity, Angle: o.Angle})
				return
			}
			// Text anchor is the baseline centre; move it so the cap height is centred.
			a := o.Angle * math.Pi / 180
			off := sh / 2
			items = append(items, Item{Kind: "text", Text: o.Text, Size: o.Size, Bold: o.Bold, Color: o.Color, Opacity: o.Opacity, Align: "center",
				X: cx - math.Sin(a)*off, Y: cy + math.Cos(a)*off, Angle: o.Angle})
		}
		if o.Position == "tile" {
			stepX, stepY := bw+max(40, bw*0.5), bh+max(60, bh*1.2)
			for y := stepY / 2; y < dh+stepY; y += stepY {
				row := int(math.Round(y / stepY))
				shift := 0.0
				if row%2 == 1 {
					shift = stepX / 2
				}
				for x := stepX/2 - shift; x < dw+stepX; x += stepX {
					place(x, y)
				}
			}
		} else {
			place(anchor(o.Position, dw, dh, bw, bh, 24))
		}
		if err := drawOnPage(ctx, p, items, o.Under); err != nil {
			return err
		}
	}
	return writeCtx(ctx, out)
}

// NumberOptions control page numbering.
type NumberOptions struct {
	Position string  `json:"position"` // tl tc tr bl bc br
	Margin   float64 `json:"margin"`   // points from the edge
	Start    int     `json:"start"`    // first number
	Pages    string  `json:"pages"`    // pages that get a number ("" = all)
	Format   string  `json:"format"`   // e.g. "{n}", "Halaman {n} dari {total}"
	Size     float64 `json:"size"`
	Color    string  `json:"color"`
	Bold     bool    `json:"bold"`
	Mirror   bool    `json:"mirror"` // swap left/right on even pages (books)
}

// AddPageNumbers writes page numbers on the selected pages.
func AddPageNumbers(in Input, o NumberOptions, out string) error {
	ctx, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	pages, err := selected(o.Pages, ctx.PageCount)
	if err != nil {
		return err
	}
	if len(pages) == 0 {
		return errors.New(i18n.L("pilih halaman yang diberi nomor", "choose the pages to number"))
	}
	if o.Size <= 0 {
		o.Size = 11
	}
	if o.Margin <= 0 {
		o.Margin = 28
	}
	if o.Start == 0 {
		o.Start = 1
	}
	if strings.TrimSpace(o.Format) == "" {
		o.Format = "{n}"
	}
	if len(o.Position) != 2 {
		o.Position = "bc"
	}
	total := o.Start + len(pages) - 1
	sizes, err := DisplaySizes(ctx)
	if err != nil {
		return err
	}
	for i, p := range pages {
		n := o.Start + i
		text := strings.NewReplacer("{n}", strconv.Itoa(n), "{total}", strconv.Itoa(total), "{N}", strconv.Itoa(total)).Replace(o.Format)
		dw, dh := sizes[p-1].W, sizes[p-1].H
		pos := o.Position
		if o.Mirror && p%2 == 0 {
			switch pos[1] {
			case 'l':
				pos = pos[:1] + "r"
			case 'r':
				pos = pos[:1] + "l"
			}
		}
		it := Item{Kind: "text", Text: text, Size: o.Size, Bold: o.Bold, Color: o.Color}
		switch pos[1] {
		case 'l':
			it.X, it.Align = o.Margin, "left"
		case 'r':
			it.X, it.Align = dw-o.Margin, "right"
		default:
			it.X, it.Align = dw/2, "center"
		}
		if pos[0] == 't' {
			it.Y = o.Margin + o.Size*0.72
		} else {
			it.Y = dh - o.Margin
		}
		if err := drawOnPage(ctx, p, []Item{it}, false); err != nil {
			return err
		}
	}
	return writeCtx(ctx, out)
}
