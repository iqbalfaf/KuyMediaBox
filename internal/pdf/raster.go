package pdf

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"kuymediabox/internal/i18n"
)

// ImageExportOptions control PDF → images.
type ImageExportOptions struct {
	Mode    string `json:"mode"`    // pages | extract
	Format  string `json:"format"`  // jpg | png
	DPI     int    `json:"dpi"`     // 72..600 for pages
	Quality int    `json:"quality"` // JPEG quality
	Pages   string `json:"pages"`   // "" = every page
}

func saveImage(path string, img image.Image, format string, quality int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	if format == "png" {
		err = (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(w, img)
	} else {
		err = jpeg.Encode(w, flattenWhite(img), &jpeg.Options{Quality: quality})
	}
	if err == nil {
		err = w.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(path)
	}
	return err
}

func uniquePath(dir, name, ext string) string {
	p := filepath.Join(dir, name+"."+ext)
	for k := 2; fileExists(p); k++ {
		p = filepath.Join(dir, fmt.Sprintf("%s (%d).%s", name, k, ext))
	}
	return p
}

// ToImages renders pages to images, or extracts the pictures embedded in the PDF.
func ToImages(ctx context.Context, in Input, o ImageExportOptions, dir, name string, prog Progress) ([]string, error) {
	if o.Format != "png" {
		o.Format = "jpg"
	}
	if o.Quality < 1 || o.Quality > 100 {
		o.Quality = 90
	}
	if o.DPI < 36 || o.DPI > 600 {
		o.DPI = 150
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if o.Mode == "extract" {
		return extractImages(ctx, in, o, dir, name, prog)
	}
	doc, err := Open(ctx, in.Path, in.Password)
	if err != nil {
		return nil, friendly(err)
	}
	defer doc.Close()
	pages, err := selected(o.Pages, doc.Pages())
	if err != nil {
		return nil, err
	}
	width := len(fmt.Sprint(doc.Pages()))
	var files []string
	for i, p := range pages {
		if err := ctx.Err(); err != nil {
			return files, err
		}
		img, err := doc.RenderDPI(p-1, o.DPI)
		if err != nil {
			return files, err
		}
		out := uniquePath(dir, fmt.Sprintf("%s-%0*d", name, width, p), o.Format)
		if err := saveImage(out, img, o.Format, o.Quality); err != nil {
			return files, err
		}
		files = append(files, out)
		prog(float64(i+1) / float64(len(pages)))
	}
	return files, nil
}

func extractImages(ctx context.Context, in Input, o ImageExportOptions, dir, name string, prog Progress) ([]string, error) {
	f, err := os.Open(in.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var sel []string
	if strings.TrimSpace(o.Pages) != "" {
		pc, err := readCtx(in.Path, in.Password)
		if err != nil {
			return nil, err
		}
		pages, err := ParsePages(o.Pages, pc.PageCount)
		if err != nil {
			return nil, err
		}
		sel = pagesToStrings(pages)
	}
	var files []string
	n := 0
	digest := func(img model.Image, _ bool, _ int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if img.Thumb || img.IsImgMask {
			return nil
		}
		n++
		ext := img.FileType
		if ext == "" {
			ext = "png"
		}
		out := uniquePath(dir, fmt.Sprintf("%s-hal%d-%d", name, img.PageNr, n), ext)
		w, err := os.Create(out)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, img)
		if cerr := w.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
		files = append(files, out)
		prog(min(0.95, float64(n)*0.05))
		return nil
	}
	if err := api.ExtractImages(f, sel, digest, conf(in.Password)); err != nil {
		return files, friendly(err)
	}
	if len(files) == 0 {
		return nil, errors.New(i18n.L("tidak ada gambar di dalam PDF ini", "there are no pictures inside this PDF"))
	}
	return files, nil
}
