// Package imageconv converts still images in pure Go (JPG, PNG, WEBP, AVIF, HEIC, BMP, TIFF,
// GIF in; JPG, PNG, WEBP, AVIF, BMP, TIFF, ICO, PDF out), with an ffmpeg fallback decoder
// for anything else.
package imageconv

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/gen2brain/avif"
	_ "github.com/gen2brain/heic" // registers "heic"
	"github.com/gen2brain/webp"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
)

func init() {
	// gif and bmp/tiff register themselves via their packages; keep references so imports stay.
	_ = gif.Decode
}

// Options come from the Image page.
type Options struct {
	Format     string `json:"format"`     // jpg png webp avif pdf ico bmp tiff
	Quality    int    `json:"quality"`    // 1..100 for jpg webp avif pdf
	ResizeMode string `json:"resizeMode"` // original longest percent box
	Longest    int    `json:"longest"`
	Percent    int    `json:"percent"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Background string `json:"background"` // #rrggbb for formats without transparency
	AutoRotate bool   `json:"autoRotate"`

	KeepMetadata bool      `json:"keepMetadata"` // copy EXIF (camera, date, GPS) from JPG/PNG into JPG/PNG
	TargetKB     int       `json:"targetKB"`     // >0: make the file at most this size
	IcoSizes     []int     `json:"icoSizes"`     // ICO: sizes to include (16, 24, 32, 48, 64, 128, 256)
	Rotate       int       `json:"rotate"`       // clockwise: 0 90 180 270
	FlipH        bool      `json:"flipH"`
	FlipV        bool      `json:"flipV"`
	Crop         string    `json:"crop"`    // "" or an aspect ratio "1:1", "16:9", "4:5", …
	CropBox      Box       `json:"cropBox"` // manual crop in fractions of the (rotated) picture
	Watermark    Watermark `json:"watermark"`

	AIUpscale   int    `json:"aiUpscale"`   // 0, 2, 3 or 4: enlarge with Real-ESRGAN first
	AIModel     string `json:"aiModel"`     // photo | anime
	RemoveBG    bool   `json:"removeBg"`    // cut the subject out (transparent background)
	BGModel     string `json:"bgModel"`     // general | people | fast
	BGMask      bool   `json:"bgMask"`      // save the black-and-white mask instead
	PNGCompress string `json:"pngCompress"` // "" | lossless (oxipng) | small (pngquant + oxipng)
}

// Box is a manual crop in fractions (0..1). W == 0 means none.
type Box struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Hooks run the optional AI tools; the app sets them when the tools are installed.
var Hooks struct {
	Upscale     func(ctx context.Context, img image.Image, scale int, model string) (image.Image, error)
	RemoveBG    func(ctx context.Context, img image.Image, model string, maskOnly bool) (image.Image, error)
	OptimizePNG func(ctx context.Context, path, mode string) error
}

// Formats lists the output formats.
var Formats = []string{"jpg", "png", "webp", "avif", "pdf", "ico", "bmp", "tiff"}

// InputExts lists the extensions accepted by the Image page.
var InputExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".jfif": true, ".png": true, ".webp": true, ".gif": true, ".bmp": true,
	".tif": true, ".tiff": true, ".heic": true, ".heif": true, ".avif": true, ".ico": true,
}

// Normalize fills defaults.
func (o *Options) Normalize() {
	valid := false
	for _, f := range Formats {
		if f == o.Format {
			valid = true
		}
	}
	if !valid {
		o.Format = "jpg"
	}
	if o.Quality < 1 || o.Quality > 100 {
		o.Quality = 85
	}
	switch o.ResizeMode {
	case "longest", "percent", "box":
	default:
		o.ResizeMode = "original"
	}
	if _, err := parseHex(o.Background); err != nil {
		o.Background = "#ffffff"
	}
	o.Rotate = ((o.Rotate % 360) + 360) % 360 / 90 * 90
	if _, _, ok := parseRatio(o.Crop); !ok {
		o.Crop = ""
	}
	if o.TargetKB < 0 {
		o.TargetKB = 0
	}
	o.IcoSizes = normalizeIcoSizes(o.IcoSizes)
	o.Watermark.normalize()
	switch o.AIUpscale {
	case 2, 3, 4:
	default:
		o.AIUpscale = 0
	}
	if o.AIModel != "anime" {
		o.AIModel = "photo"
	}
	switch o.BGModel {
	case "people", "fast":
	default:
		o.BGModel = "general"
	}
	if !o.RemoveBG {
		o.BGMask = false
	}
	if o.Format != "png" || (o.PNGCompress != "lossless" && o.PNGCompress != "small") {
		o.PNGCompress = ""
	}
	b := &o.CropBox
	clamp := func(v float64) float64 { return math.Max(0, math.Min(v, 1)) }
	b.X, b.Y = clamp(b.X), clamp(b.Y)
	b.W, b.H = math.Min(clamp(b.W), 1-b.X), math.Min(clamp(b.H), 1-b.Y)
	if b.W < 0.01 || b.H < 0.01 || (b.W > 0.999 && b.H > 0.999) {
		*b = Box{}
	}
}

// Size is width × height.
type Size struct {
	W int `json:"w"`
	H int `json:"h"`
}

// TargetSize returns the output dimensions for a source size (never upscaling).
func TargetSize(w, h int, o Options) Size {
	o.Normalize()
	if w <= 0 || h <= 0 {
		return Size{w, h}
	}
	if o.Rotate == 90 || o.Rotate == 270 {
		w, h = h, w
	}
	if c := o.CropBox; c.W > 0 {
		w, h = max(1, int((c.X+c.W)*float64(w))-int(c.X*float64(w))), max(1, int((c.Y+c.H)*float64(h))-int(c.Y*float64(h)))
	}
	if cw, ch := cropSize(w, h, o.Crop); cw > 0 {
		w, h = cw, ch
	}
	if o.AIUpscale > 0 {
		w, h = w*o.AIUpscale, h*o.AIUpscale
	}
	nw, nh := w, h
	switch o.ResizeMode {
	case "longest":
		if o.Longest > 0 && max(w, h) > o.Longest {
			if w >= h {
				nw, nh = o.Longest, roundDiv(h*o.Longest, w)
			} else {
				nw, nh = roundDiv(w*o.Longest, h), o.Longest
			}
		}
	case "percent":
		if o.Percent > 0 && o.Percent < 100 {
			nw, nh = roundDiv(w*o.Percent, 100), roundDiv(h*o.Percent, 100)
		}
	case "box":
		bw, bh := o.Width, o.Height
		if bw <= 0 {
			bw = w
		}
		if bh <= 0 {
			bh = h
		}
		if w > bw || h > bh {
			scale := minF(float64(bw)/float64(w), float64(bh)/float64(h))
			nw, nh = int(float64(w)*scale+0.5), int(float64(h)*scale+0.5)
		}
	}
	if o.Format == "ico" && len(o.IcoSizes) > 0 {
		return Size{o.IcoSizes[0], o.IcoSizes[0]}
	}
	if o.Format == "ico" && max(nw, nh) > 256 {
		if nw >= nh {
			nw, nh = 256, roundDiv(nh*256, nw)
		} else {
			nw, nh = roundDiv(nw*256, nh), 256
		}
	}
	return Size{max(1, nw), max(1, nh)}
}

// Config reads the dimensions and format of an image without decoding it fully.
func Config(path string) (Size, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return Size{}, "", err
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(bufio.NewReader(f))
	if err != nil {
		return Size{}, "", err
	}
	return Size{cfg.Width, cfg.Height}, format, nil
}

// Convert decodes in, applies the options and writes out. ffmpegPath may be empty.
func Convert(ctx context.Context, in, out string, o Options, ffmpegPath string, progress func(float64)) error {
	o.Normalize()
	progress(0.05)
	img, err := decode(ctx, in, o.AutoRotate, ffmpegPath)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	progress(0.3)

	img = transform(img, o)
	if o.AIUpscale > 0 {
		if Hooks.Upscale == nil {
			return errors.New(i18n.L("Real-ESRGAN belum terpasang (Pengaturan › Tools pendukung)", "Real-ESRGAN isn't installed (Settings › Supporting tools)"))
		}
		if img, err = Hooks.Upscale(ctx, img, o.AIUpscale, o.AIModel); err != nil {
			return err
		}
	}
	if o.RemoveBG {
		if Hooks.RemoveBG == nil {
			return errors.New(i18n.L("Hapus latar belum siap: pasang ONNX Runtime dan unduh modelnya", "Background removal isn't ready: install ONNX Runtime and download a model"))
		}
		if img, err = Hooks.RemoveBG(ctx, img, o.BGModel, o.BGMask); err != nil {
			return err
		}
	}
	progress(0.4)
	b := img.Bounds()
	ts := TargetSize(b.Dx(), b.Dy(), Options{Format: o.Format, ResizeMode: o.ResizeMode, Longest: o.Longest, Percent: o.Percent, Width: o.Width, Height: o.Height})
	if ts.W != b.Dx() || ts.H != b.Dy() {
		img = imaging.Resize(img, ts.W, ts.H, imaging.Lanczos)
	}
	if o.Watermark.Enabled {
		if img, err = applyWatermark(ctx, img, o.Watermark, ffmpegPath); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	progress(0.5)

	var exif []byte
	if o.KeepMetadata && (o.Format == "jpg" || o.Format == "png") {
		exif = readExif(in)
		if exif != nil && (o.AutoRotate || o.Rotate != 0 || o.FlipH || o.FlipV) {
			exif = resetOrientation(exif) // pixels are already upright
		}
	}

	bg, _ := parseHex(o.Background)
	data, err := encodeBytes(ctx, img, o, bg, progress)
	if err != nil {
		return err
	}
	if exif != nil {
		data = embedExif(data, o.Format, exif)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		_ = os.Remove(out)
		return fmt.Errorf(i18n.L("tidak bisa membuat file hasil: %w", "can't create the output file: %w"), err)
	}
	if o.PNGCompress != "" {
		if Hooks.OptimizePNG == nil {
			return errors.New(i18n.L("oxipng/pngquant belum terpasang (Pengaturan › Tools pendukung)", "oxipng/pngquant isn't installed (Settings › Supporting tools)"))
		}
		progress(0.9)
		if err := Hooks.OptimizePNG(ctx, out, o.PNGCompress); err != nil {
			return err
		}
	}
	progress(1)
	return nil
}

// encodeBytes encodes img, shrinking quality and then size until the file fits TargetKB.
func encodeBytes(ctx context.Context, img image.Image, o Options, bg color.Color, progress func(float64)) ([]byte, error) {
	enc := func(im image.Image, q int) ([]byte, error) {
		oo := o
		oo.Quality = q
		var buf bytes.Buffer
		if err := encode(&buf, im, oo, bg); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	data, err := enc(img, o.Quality)
	if err != nil || o.TargetKB <= 0 {
		return data, err
	}
	target := o.TargetKB * 1024
	if len(data) <= target {
		return data, nil
	}
	lossy := o.Format == "jpg" || o.Format == "webp" || o.Format == "avif" || o.Format == "pdf"
	best := data
	for round := 0; round < 8; round++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		progress(0.5 + 0.45*float64(round)/8)
		if lossy {
			// Highest quality (down to 10) that fits.
			lo, hi := 10, o.Quality
			var fit []byte
			for lo <= hi {
				q := (lo + hi) / 2
				d, err := enc(img, q)
				if err != nil {
					return nil, err
				}
				if len(d) <= target {
					fit, lo = d, q+1
				} else {
					hi = q - 1
					if len(d) < len(best) {
						best = d
					}
				}
			}
			if fit != nil {
				return fit, nil
			}
		}
		// Still too big: make the picture smaller and try again.
		b := img.Bounds()
		f := math.Sqrt(float64(target)/float64(len(best))) * 0.95
		if lossy {
			f = math.Min(f, 0.85)
		}
		f = math.Max(0.3, math.Min(f, 0.95))
		nw, nh := int(float64(b.Dx())*f), int(float64(b.Dy())*f)
		if nw < 16 || nh < 16 {
			break
		}
		img = imaging.Resize(img, nw, nh, imaging.Lanczos)
		d, err := enc(img, o.Quality)
		if err != nil {
			return nil, err
		}
		if len(d) <= target {
			return d, nil
		}
		best = d
	}
	return best, nil
}

// Decode reads any supported image (EXIF orientation applied); ffmpegPath may be empty.
func Decode(ctx context.Context, path, ffmpegPath string) (image.Image, error) {
	return decode(ctx, path, true, ffmpegPath)
}

// Flatten draws img over a solid background so it can be stored without alpha.
func Flatten(img image.Image, bg color.Color) image.Image { return flatten(img, bg) }

func decode(ctx context.Context, path string, autoRotate bool, ffmpegPath string) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(i18n.L("file tidak bisa dibuka: %w", "file can't be opened: %w"), err)
	}
	_, format, cfgErr := image.DecodeConfig(bytes.NewReader(data))
	var img image.Image
	if cfgErr == nil {
		switch format {
		case "jpeg":
			img, err = imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(autoRotate))
		case "webp":
			img, err = webp.Decode(bytes.NewReader(data), webp.Options{AutoRotate: autoRotate})
		case "avif":
			img, err = avif.Decode(bytes.NewReader(data), avif.Options{AutoRotate: autoRotate})
		default:
			img, _, err = image.Decode(bytes.NewReader(data))
		}
		if err == nil && img != nil {
			return img, nil
		}
	}
	if ext := strings.ToLower(filepath.Ext(path)); ext == ".ico" {
		if ico, icoErr := decodeICO(data); icoErr == nil {
			return ico, nil
		}
	}
	if ffmpegPath != "" {
		if fimg, ferr := decodeWithFFmpeg(ctx, path, ffmpegPath); ferr == nil {
			return fimg, nil
		}
	}
	if err == nil {
		err = cfgErr
	}
	return nil, fmt.Errorf(i18n.L("format gambar tidak didukung atau file rusak (%v)", "unsupported image format or damaged file (%v)"), err)
}

func decodeWithFFmpeg(ctx context.Context, path, ffmpegPath string) (image.Image, error) {
	tmp, err := os.CreateTemp("", "kmb-*.png")
	if err != nil {
		return nil, err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)
	if _, err := proc.Output(ctx, ffmpegPath, "-hide_banner", "-loglevel", "error", "-y", "-i", path, "-frames:v", "1", name); err != nil {
		return nil, err
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func encode(w io.Writer, img image.Image, o Options, bg color.Color) error {
	switch o.Format {
	case "jpg":
		return jpeg.Encode(w, flatten(img, bg), &jpeg.Options{Quality: o.Quality})
	case "png":
		return (&png.Encoder{CompressionLevel: png.DefaultCompression}).Encode(w, img)
	case "webp":
		return webp.Encode(w, img, webp.Options{Quality: o.Quality, Method: 4})
	case "avif":
		return avif.Encode(w, img, avif.Options{Quality: o.Quality, QualityAlpha: o.Quality, Speed: 8})
	case "bmp":
		return bmp.Encode(w, flatten(img, bg))
	case "tiff":
		return tiff.Encode(w, img, &tiff.Options{Compression: tiff.Deflate})
	case "ico":
		if len(o.IcoSizes) > 0 {
			return encodeICOSizes(w, img, o.IcoSizes)
		}
		return encodeICO(w, img)
	case "pdf":
		return encodePDF(w, flatten(img, bg), o.Quality)
	}
	return errors.New(i18n.L("format hasil tidak dikenal", "unknown output format"))
}

// flatten draws img over a solid background (for formats without alpha).
func flatten(img image.Image, bg color.Color) image.Image {
	if !hasAlpha(img) {
		return img
	}
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Over)
	return dst
}

func hasAlpha(img image.Image) bool {
	switch m := img.(type) {
	case *image.YCbCr, *image.Gray, *image.Gray16, *image.CMYK:
		return false
	case *image.RGBA:
		return !m.Opaque()
	case *image.NRGBA:
		return !m.Opaque()
	case *image.RGBA64:
		return !m.Opaque()
	case *image.NRGBA64:
		return !m.Opaque()
	case *image.Paletted:
		return !m.Opaque()
	}
	return true
}

func parseHex(s string) (color.RGBA, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return color.RGBA{}, errors.New("bad color")
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}, nil
}

func roundDiv(a, b int) int { return (a + b/2) / b }

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
