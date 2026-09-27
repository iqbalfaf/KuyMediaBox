package imageconv

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"

	"github.com/disintegration/imaging"

	"kuymediabox/internal/i18n"
)

// FitCanvas places img on a w×h canvas: "cover" fills it (cropping the middle), "blur" puts
// the whole picture over a blurred, enlarged copy of itself, "contain" adds bars of bg.
func FitCanvas(img image.Image, w, h int, fit string, bg color.Color) *image.NRGBA {
	switch fit {
	case "cover":
		return imaging.Fill(img, w, h, imaging.Center, imaging.Lanczos)
	case "blur":
		back := imaging.Fill(img, max(1, w/8), max(1, h/8), imaging.Center, imaging.Linear)
		back = imaging.Blur(back, 3)
		canvas := imaging.Resize(back, w, h, imaging.Linear)
		return imaging.PasteCenter(canvas, scaleInto(img, w, h))
	}
	canvas := imaging.New(w, h, bg)
	fg := scaleInto(img, w, h)
	b := fg.Bounds()
	draw.Draw(canvas, image.Rect((w-b.Dx())/2, (h-b.Dy())/2, (w-b.Dx())/2+b.Dx(), (h-b.Dy())/2+b.Dy()), fg, b.Min, draw.Over)
	return canvas
}

// scaleInto resizes img to the largest size that fits w×h (small pictures are enlarged).
func scaleInto(img image.Image, w, h int) *image.NRGBA {
	b := img.Bounds()
	f := math.Min(float64(w)/float64(b.Dx()), float64(h)/float64(b.Dy()))
	nw, nh := max(1, min(w, int(math.Round(float64(b.Dx())*f)))), max(1, min(h, int(math.Round(float64(b.Dy())*f))))
	return imaging.Resize(img, nw, nh, imaging.Lanczos)
}

// RatioSize returns w×h with the longest side long for a ratio "16:9" ("" = ref's ratio).
// Both sides are even (video encoders need that).
func RatioSize(ratio string, ref image.Rectangle, long int) (int, int) {
	rx, ry, ok := parseRatio(ratio)
	if !ok {
		rx, ry = float64(max(1, ref.Dx())), float64(max(1, ref.Dy()))
	}
	even := func(v float64) int { return max(2, int(math.Round(v/2))*2) }
	if rx >= ry {
		return even(float64(long)), even(float64(long) * ry / rx)
	}
	return even(float64(long) * rx / ry), even(float64(long))
}

// CollageOptions lay pictures out in a grid.
type CollageOptions struct {
	Cols       int    `json:"cols"`       // 0 = about square
	Width      int    `json:"width"`      // width of the result in pixels
	Gap        int    `json:"gap"`        // pixels between and around the pictures
	Cell       string `json:"cell"`       // ratio of every cell: 1:1, 4:5, 3:4, 16:9, …
	Fit        string `json:"fit"`        // cover | contain
	Background string `json:"background"` // #rrggbb
	Radius     int    `json:"radius"`     // rounded corners in pixels
}

// Collage decodes the pictures and lays them out in a grid.
func Collage(ctx context.Context, paths []string, o CollageOptions, ffmpegPath string, progress func(float64)) (*image.NRGBA, error) {
	n := len(paths)
	if n < 2 {
		return nil, errors.New(i18n.L("Tambahkan minimal 2 gambar untuk kolase", "Add at least 2 pictures for a collage"))
	}
	cols := o.Cols
	if cols <= 0 {
		cols = int(math.Ceil(math.Sqrt(float64(n))))
	}
	cols = max(1, min(cols, n))
	rows := (n + cols - 1) / cols
	width := max(200, min(o.Width, 12000))
	gap := max(0, min(o.Gap, width/10))
	rx, ry, ok := parseRatio(o.Cell)
	if !ok {
		rx, ry = 1, 1
	}
	cellW := (width - gap*(cols+1)) / cols
	if cellW < 8 {
		return nil, errors.New(i18n.L("Kolase terlalu sempit untuk jumlah kolom ini", "The collage is too narrow for this many columns"))
	}
	cellH := max(8, int(float64(cellW)*ry/rx))
	height := rows*cellH + gap*(rows+1)
	var bg color.Color = color.White
	if c, err := parseHex(o.Background); err == nil {
		bg = c
	}
	out := imaging.New(width, height, bg)
	for i, p := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		img, err := decode(ctx, p, true, ffmpegPath)
		if err != nil {
			return nil, err
		}
		fit := o.Fit
		if fit != "contain" {
			fit = "cover"
		}
		cell := FitCanvas(img, cellW, cellH, fit, bg)
		if o.Radius > 0 {
			roundCorners(cell, min(o.Radius, min(cellW, cellH)/2))
		}
		// The last row is centred when it isn't full.
		row, col := i/cols, i%cols
		inRow := cols
		if row == rows-1 && n%cols != 0 {
			inRow = n % cols
		}
		x0 := gap + (width-gap*(inRow+1)-inRow*cellW)/2 + col*(cellW+gap)
		if inRow == cols {
			x0 = gap + col*(cellW+gap)
		}
		y0 := gap + row*(cellH+gap)
		draw.Draw(out, image.Rect(x0, y0, x0+cellW, y0+cellH), cell, image.Point{}, draw.Over)
		progress(float64(i+1) / float64(n))
	}
	return out, nil
}

// roundCorners makes the corners of img transparent (anti-aliased).
func roundCorners(img *image.NRGBA, r int) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	for y := 0; y < r; y++ {
		for x := 0; x < r; x++ {
			dx, dy := float64(r-x)-0.5, float64(r-y)-0.5
			d := math.Sqrt(dx*dx+dy*dy) - float64(r)
			if d <= -1 {
				continue
			}
			a := 1 - math.Min(1, math.Max(0, d+1))
			for _, p := range [][2]int{{x, y}, {w - 1 - x, y}, {x, h - 1 - y}, {w - 1 - x, h - 1 - y}} {
				i := p[1]*img.Stride + p[0]*4
				img.Pix[i+3] = uint8(float64(img.Pix[i+3]) * a)
			}
		}
	}
}

// EncodeFile saves img with the Image page's encoder (format, quality, background, EXIF-free).
func EncodeFile(ctx context.Context, img image.Image, out string, o Options) error {
	o.Normalize()
	bg, _ := parseHex(o.Background)
	data, err := encodeBytes(ctx, img, o, bg, func(float64) {})
	if err != nil {
		return err
	}
	return os.WriteFile(out, data, 0o644)
}
