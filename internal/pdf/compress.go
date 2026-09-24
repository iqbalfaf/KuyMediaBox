package pdf

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"os"

	"github.com/disintegration/imaging"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// ErrNoGain means the compressed file would not be smaller than the original.
var ErrNoGain = errors.New("no gain")

// CompressOptions control compression.
type CompressOptions struct {
	Level string `json:"level"` // extreme | recommended | low
	Gray  bool   `json:"gray"`  // also turn pictures grey
}

type compressLevel struct {
	maxPx   int
	quality int
}

var compressLevels = map[string]compressLevel{
	"extreme":     {1300, 45},
	"recommended": {2000, 65},
	"low":         {3000, 82},
}

// Compress shrinks pictures and removes duplicate objects.
func Compress(ctx context.Context, in Input, o CompressOptions, out string, prog Progress) error {
	lv, ok := compressLevels[o.Level]
	if !ok {
		lv = compressLevels["recommended"]
	}
	pc, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	xt := pc.XRefTable
	// Images used as soft masks stay untouched (they must match their parent's layout).
	masks := map[int]bool{}
	var images []int
	for nr, e := range xt.Table {
		if e == nil || e.Free || e.Object == nil {
			continue
		}
		sd, ok := e.Object.(types.StreamDict)
		if !ok || sd.Subtype() == nil || *sd.Subtype() != "Image" {
			continue
		}
		if ir := sd.IndirectRefEntry("SMask"); ir != nil {
			masks[ir.ObjectNumber.Value()] = true
		}
		images = append(images, nr)
	}
	for i, nr := range images {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !masks[nr] {
			recompressImage(xt, nr, lv, o.Gray)
		}
		prog(float64(i+1) / float64(len(images)) * 0.8)
	}
	if err := api.OptimizeContext(pc); err != nil {
		return err
	}
	if err := writeCtx(pc, out); err != nil {
		return err
	}
	a, errA := os.Stat(in.Path)
	b, errB := os.Stat(out)
	if errA == nil && errB == nil && b.Size() >= a.Size() {
		_ = os.Remove(out)
		return ErrNoGain
	}
	return nil
}

func recompressImage(xt *model.XRefTable, nr int, lv compressLevel, gray bool) {
	e := xt.Table[nr]
	sd := e.Object.(types.StreamDict)
	if b := sd.BooleanEntry("ImageMask"); b != nil && *b {
		return
	}
	if _, ok := sd.Find("Mask"); ok {
		return // colour-key masks break with lossy compression
	}
	if bpc := sd.IntEntry("BitsPerComponent"); bpc == nil || *bpc != 8 {
		return
	}
	w, h := sd.IntEntry("Width"), sd.IntEntry("Height")
	if w == nil || h == nil || *w < 16 || *h < 16 || *w**h > 80_000_000 {
		return
	}
	img := decodeImageStream(xt, &sd, *w, *h)
	if img == nil {
		return
	}
	if gray {
		img = imaging.Grayscale(img)
	}
	if long := max(*w, *h); long > lv.maxPx {
		nw, nh := *w*lv.maxPx/long, *h*lv.maxPx/long
		img = imaging.Resize(img, max(1, nw), max(1, nh), imaging.Lanczos)
	}
	isGray := gray
	if _, ok := img.(*image.Gray); ok {
		isGray = true
	}
	if isGray {
		img = toGray(img)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: lv.quality}); err != nil {
		return
	}
	if int64(buf.Len()) >= int64(len(sd.Raw))*9/10 {
		return // not worth it
	}
	b := img.Bounds()
	sd.Raw = buf.Bytes()
	sd.Content = nil
	l := int64(buf.Len())
	sd.StreamLength = &l
	sd.FilterPipeline = []types.PDFFilter{{Name: "DCTDecode"}}
	sd.Update("Filter", types.Name("DCTDecode"))
	sd.Delete("DecodeParms")
	sd.Delete("Decode")
	sd.Update("Width", types.Integer(b.Dx()))
	sd.Update("Height", types.Integer(b.Dy()))
	sd.Update("BitsPerComponent", types.Integer(8))
	sd.Update("Length", types.Integer(l))
	if isGray {
		sd.Update("ColorSpace", types.Name("DeviceGray"))
	} else {
		sd.Update("ColorSpace", types.Name("DeviceRGB"))
	}
	e.Object = sd
}

func toGray(img image.Image) *image.Gray {
	if g, ok := img.(*image.Gray); ok {
		return g
	}
	b := img.Bounds()
	g := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			g.Set(x, y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return g
}

// components returns 1 or 3 for grey/RGB colour spaces we can rebuild, else 0.
func components(xt *model.XRefTable, cs types.Object) int {
	o, err := xt.Dereference(cs)
	if err != nil || o == nil {
		return 0
	}
	switch v := o.(type) {
	case types.Name:
		switch v {
		case "DeviceRGB", "CalRGB":
			return 3
		case "DeviceGray", "CalGray":
			return 1
		}
	case types.Array:
		if len(v) == 0 {
			return 0
		}
		name, _ := v[0].(types.Name)
		switch name {
		case "ICCBased":
			if len(v) < 2 {
				return 0
			}
			ref, err := xt.Dereference(v[1])
			if err != nil {
				return 0
			}
			if isd, ok := ref.(types.StreamDict); ok {
				if n := isd.IntEntry("N"); n != nil && (*n == 1 || *n == 3) {
					return *n
				}
			}
		case "CalRGB":
			return 3
		case "CalGray":
			return 1
		}
	}
	return 0
}

func decodeImageStream(xt *model.XRefTable, sd *types.StreamDict, w, h int) image.Image {
	cs, _ := sd.Find("ColorSpace")
	n := components(xt, cs)
	if len(sd.FilterPipeline) == 1 && sd.FilterPipeline[0].Name == "DCTDecode" {
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(sd.Raw))
		if err != nil || n == 0 || cfg.Width == 0 {
			return nil
		}
		img, err := jpeg.Decode(bytes.NewReader(sd.Raw))
		if err != nil {
			return nil
		}
		if _, cmyk := img.(*image.CMYK); cmyk {
			return nil
		}
		return img
	}
	if n == 0 {
		return nil
	}
	for _, f := range sd.FilterPipeline {
		if f.Name != "FlateDecode" && f.Name != "LZWDecode" {
			return nil
		}
	}
	if err := sd.Decode(); err != nil || len(sd.Content) < w*h*n {
		return nil
	}
	px := sd.Content
	if n == 1 {
		g := image.NewGray(image.Rect(0, 0, w, h))
		copy(g.Pix, px[:w*h])
		return g
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i, j := 0, 0; i < w*h*3; i, j = i+3, j+4 {
		img.Pix[j], img.Pix[j+1], img.Pix[j+2], img.Pix[j+3] = px[i], px[i+1], px[i+2], 255
	}
	return img
}
