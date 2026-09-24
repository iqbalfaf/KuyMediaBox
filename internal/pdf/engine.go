// Package pdf holds the offline PDF tools: page organising, compression, security, stamps,
// conversions to and from PDF, OCR, redaction and comparison. Rendering and text extraction
// use PDFium compiled to WebAssembly (no cgo, no external program); file manipulation uses
// pdfcpu.
package pdf

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	pdfiumerr "github.com/klippa-app/go-pdfium/errors"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/tetratelabs/wazero"
	wapi "github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/i18n"
)

// ErrPassword is returned when a document needs a (different) password.
var ErrPassword = errors.New("password")

var (
	poolOnce sync.Once
	poolVal  pdfium.Pool
	poolErr  error
)

// Warmup starts the PDF engine in the background so the first page shows quickly.
func Warmup() { go func() { _, _ = enginePool() }() }

func enginePool() (pdfium.Pool, error) {
	poolOnce.Do(func() {
		rc := wazero.NewRuntimeConfig().WithCoreFeatures(wapi.CoreFeaturesV2 | experimental.CoreFeaturesExceptionHandling)
		// Caching the compiled module makes every start after the first one fast.
		if cache, err := wazero.NewCompilationCacheWithDir(filepath.Join(appdir.DataDir(), "cache", "pdfium")); err == nil {
			rc = rc.WithCompilationCache(cache)
		}
		poolVal, poolErr = webassembly.Init(webassembly.Config{
			MinIdle:       0,
			MaxIdle:       2,
			MaxTotal:      6,
			FSConfig:      wazero.NewFSConfig(),
			RuntimeConfig: rc,
			Stdout:        io.Discard,
			Stderr:        io.Discard,
		})
		if poolErr != nil {
			poolErr = fmt.Errorf(i18n.L("mesin PDF tidak bisa dijalankan: %w", "the PDF engine can't start: %w"), poolErr)
		}
	})
	return poolVal, poolErr
}

func instance(ctx context.Context) (pdfium.Pdfium, error) {
	pool, err := enginePool()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return pool.GetInstanceWithContext(ctx)
}

// Doc is an open document inside its own engine instance. It is not safe for concurrent use.
type Doc struct {
	inst  pdfium.Pdfium
	ref   references.FPDF_DOCUMENT
	file  *os.File
	pages int
	owned bool // instance belongs to this doc
}

// Open loads a PDF with its own engine instance.
func Open(ctx context.Context, path, password string) (*Doc, error) {
	inst, err := instance(ctx)
	if err != nil {
		return nil, err
	}
	d, err := openIn(inst, path, password)
	if err != nil {
		inst.Close()
		return nil, err
	}
	d.owned = true
	return d, nil
}

func openIn(inst pdfium.Pdfium, path, password string) (*Doc, error) {
	return openWith(inst, path, password, false)
}

// openWith opens a document; inMemory reads the whole file first so no handle stays open
// (Windows would otherwise refuse to rename or delete the file while it is previewed).
func openWith(inst pdfium.Pdfium, path, password string, inMemory bool) (*Doc, error) {
	if inMemory {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		req := &requests.OpenDocument{File: &data}
		if password != "" {
			req.Password = &password
		}
		return finishOpen(inst, req, nil)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	req := &requests.OpenDocument{FileReader: f, FileReaderSize: st.Size()}
	if password != "" {
		req.Password = &password
	}
	return finishOpen(inst, req, f)
}

func finishOpen(inst pdfium.Pdfium, req *requests.OpenDocument, f *os.File) (*Doc, error) {
	res, err := inst.OpenDocument(req)
	if err != nil {
		if f != nil {
			f.Close()
		}
		return nil, engineErr(err)
	}
	d := &Doc{inst: inst, ref: res.Document, file: f}
	n, err := inst.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: res.Document})
	if err != nil {
		d.Close()
		return nil, engineErr(err)
	}
	d.pages = n.PageCount
	return d, nil
}

func engineErr(err error) error {
	switch {
	case errors.Is(err, pdfiumerr.ErrPassword), errors.Is(err, pdfiumerr.ErrSecurity):
		return ErrPassword
	case errors.Is(err, pdfiumerr.ErrFormat), errors.Is(err, pdfiumerr.ErrFile):
		return errors.New(i18n.L("bukan PDF yang valid atau file rusak", "not a valid PDF or the file is damaged"))
	}
	return err
}

// Close releases the document (and its instance when it owns one).
func (d *Doc) Close() {
	if d == nil {
		return
	}
	_, _ = d.inst.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: d.ref})
	if d.file != nil {
		d.file.Close()
	}
	if d.owned {
		d.inst.Close()
	}
}

// Pages is the page count.
func (d *Doc) Pages() int { return d.pages }

func (d *Doc) page(i int) requests.Page {
	return requests.Page{ByIndex: &requests.PageByIndex{Document: d.ref, Index: i}}
}

// Encrypted reports whether the document uses a security handler (password or permissions).
func (d *Doc) Encrypted() bool {
	r, err := d.inst.FPDF_GetSecurityHandlerRevision(&requests.FPDF_GetSecurityHandlerRevision{Document: d.ref})
	return err == nil && r.SecurityHandlerRevision >= 0
}

// Size is the page size in points as displayed (rotation applied).
func (d *Doc) Size(i int) (w, h float64, err error) {
	s, err := d.inst.FPDF_GetPageSizeByIndex(&requests.FPDF_GetPageSizeByIndex{Document: d.ref, Index: i})
	if err != nil {
		return 0, 0, engineErr(err)
	}
	return s.Width, s.Height, nil
}

// Geom describes how a page's user space maps to what is displayed.
type Geom struct {
	X0, Y0, X1, Y1 float64 // visible box (crop box, else media box) in user space
	Rotate         int     // 0, 90, 180, 270 (clockwise)
}

// DisplaySize is the displayed width and height in points.
func (g Geom) DisplaySize() (float64, float64) {
	w, h := g.X1-g.X0, g.Y1-g.Y0
	if g.Rotate == 90 || g.Rotate == 270 {
		return h, w
	}
	return w, h
}

// ToDisplay maps a user-space point to normalized display coordinates (0..1, origin top-left).
func (g Geom) ToDisplay(x, y float64) (u, v float64) {
	w, h := g.X1-g.X0, g.Y1-g.Y0
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	switch g.Rotate {
	case 90:
		return (y - g.Y0) / h, (x - g.X0) / w
	case 180:
		return (g.X1 - x) / w, (y - g.Y0) / h
	case 270:
		return (g.Y1 - y) / h, (g.X1 - x) / w
	}
	return (x - g.X0) / w, (g.Y1 - y) / h
}

// Geometry reads the visible box and rotation of a page.
func (d *Doc) Geometry(i int) (Geom, error) {
	var g Geom
	r, err := d.inst.FPDFPage_GetRotation(&requests.FPDFPage_GetRotation{Page: d.page(i)})
	if err != nil {
		return g, engineErr(err)
	}
	g.Rotate = int(r.PageRotation) * 90
	if b, err := d.inst.FPDFPage_GetCropBox(&requests.FPDFPage_GetCropBox{Page: d.page(i)}); err == nil && b.Right > b.Left && b.Top > b.Bottom {
		g.X0, g.Y0, g.X1, g.Y1 = float64(b.Left), float64(b.Bottom), float64(b.Right), float64(b.Top)
		return g, nil
	}
	if b, err := d.inst.FPDFPage_GetMediaBox(&requests.FPDFPage_GetMediaBox{Page: d.page(i)}); err == nil && b.Right > b.Left && b.Top > b.Bottom {
		g.X0, g.Y0, g.X1, g.Y1 = float64(b.Left), float64(b.Bottom), float64(b.Right), float64(b.Top)
		return g, nil
	}
	// Fall back to the displayed size with the default origin.
	w, h, err := d.Size(i)
	if err != nil {
		return g, err
	}
	if g.Rotate == 90 || g.Rotate == 270 {
		w, h = h, w
	}
	g.X1, g.Y1 = w, h
	return g, nil
}

// Render draws a page into an image that fits within maxW × maxH pixels.
func (d *Doc) Render(i, maxW, maxH int) (image.Image, error) {
	r, err := d.inst.RenderPageInPixels(&requests.RenderPageInPixels{
		Page:        d.page(i),
		Width:       maxW,
		Height:      maxH,
		RenderFlags: enums.FPDF_RENDER_FLAG_ANNOT,
	})
	if err != nil {
		return nil, engineErr(err)
	}
	return r.Result.Image, nil
}

// RenderDPI draws a page at a resolution (dots per inch).
func (d *Doc) RenderDPI(i, dpi int) (image.Image, error) {
	r, err := d.inst.RenderPageInDPI(&requests.RenderPageInDPI{
		Page:        d.page(i),
		DPI:         dpi,
		RenderFlags: enums.FPDF_RENDER_FLAG_ANNOT | enums.FPDF_RENDER_FLAG_PRINTING,
	})
	if err != nil {
		return nil, engineErr(err)
	}
	return r.Result.Image, nil
}

// Char is one character with its box in normalized display coordinates.
type Char struct {
	Text   string
	X0, Y0 float64 // top-left
	X1, Y1 float64 // bottom-right
	Size   float64 // font size in points
	Bold   bool
}

// Chars returns the characters of a page in reading order.
func (d *Doc) Chars(i int) ([]Char, error) {
	g, err := d.Geometry(i)
	if err != nil {
		return nil, err
	}
	res, err := d.inst.GetPageTextStructured(&requests.GetPageTextStructured{
		Page:                   d.page(i),
		Mode:                   requests.GetPageTextStructuredModeChars,
		CollectFontInformation: true,
	})
	if err != nil {
		return nil, engineErr(err)
	}
	out := make([]Char, 0, len(res.Chars))
	for _, c := range res.Chars {
		p := c.PointPosition
		u0, v0 := g.ToDisplay(p.Left, p.Top)
		u1, v1 := g.ToDisplay(p.Right, p.Bottom)
		if u0 > u1 {
			u0, u1 = u1, u0
		}
		if v0 > v1 {
			v0, v1 = v1, v0
		}
		ch := Char{Text: c.Text, X0: u0, Y0: v0, X1: u1, Y1: v1}
		if fi := c.FontInformation; fi != nil {
			ch.Size = fi.RenderedSize
			if ch.Size <= 0 {
				ch.Size = fi.Size
			}
			ch.Bold = fi.Weight >= 600 || strings.Contains(strings.ToLower(fi.Name), "bold")
		}
		out = append(out, ch)
	}
	return out, nil
}

// PageText returns the plain text of a page.
func (d *Doc) PageText(i int) (string, error) {
	r, err := d.inst.GetPageText(&requests.GetPageText{Page: d.page(i)})
	if err != nil {
		return "", engineErr(err)
	}
	return r.Text, nil
}

// SaveCopy writes the document as PDFium sees it (used to rebuild damaged files).
func (d *Doc) SaveCopy(w io.Writer) error {
	_, err := d.inst.FPDF_SaveAsCopy(&requests.FPDF_SaveAsCopy{Document: d.ref, FileWriter: w})
	return engineErr(err)
}

// Info summarises a PDF for file lists.
type Info struct {
	Pages     int     `json:"pages"`
	Encrypted bool    `json:"encrypted"` // has a security handler
	Locked    bool    `json:"locked"`    // needs a password to open
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
}

// Inspect reads the page count and lock state of a PDF.
func Inspect(ctx context.Context, path string) (Info, error) {
	var info Info
	err := viewer.with(ctx, func(inst pdfium.Pdfium) error {
		d, err := openWith(inst, path, "", true)
		if errors.Is(err, ErrPassword) {
			info.Locked, info.Encrypted = true, true
			return nil
		}
		if err != nil {
			return err
		}
		defer d.Close()
		info.Pages = d.Pages()
		info.Encrypted = d.Encrypted()
		if info.Pages > 0 {
			info.Width, info.Height, _ = d.Size(0)
		}
		return nil
	})
	return info, err
}
