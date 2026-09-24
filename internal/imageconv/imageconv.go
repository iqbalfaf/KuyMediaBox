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
	progress(0.35)

	b := img.Bounds()
	ts := TargetSize(b.Dx(), b.Dy(), o)
	if ts.W != b.Dx() || ts.H != b.Dy() {
		img = imaging.Resize(img, ts.W, ts.H, imaging.Lanczos)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	progress(0.6)

	bg, _ := parseHex(o.Background)
	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("tidak bisa membuat file hasil: %w", err)
	}
	w := bufio.NewWriterSize(f, 1<<20)
	err = encode(w, img, o, bg)
	if err == nil {
		err = w.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(out)
		return err
	}
	progress(1)
	return nil
}

func decode(ctx context.Context, path string, autoRotate bool, ffmpegPath string) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("file tidak bisa dibuka: %w", err)
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
	return nil, fmt.Errorf("format gambar tidak didukung atau file rusak (%v)", err)
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
		return encodeICO(w, img)
	case "pdf":
		return encodePDF(w, flatten(img, bg), o.Quality)
	}
	return errors.New("format hasil tidak dikenal")
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
