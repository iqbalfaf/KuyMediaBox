package pdf

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/imageconv"
)

// ImagePage is one page of an image PDF. Img may be nil for a blank page.
type ImagePage struct {
	Img          image.Image
	PageW, PageH float64    // page size in points
	Box          [4]float64 // x, y, w, h of the image in points (origin bottom-left)
}

// WriteImagePDF writes pages that each show one image (JPEG compressed).
func WriteImagePDF(path string, pages []ImagePage, quality int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	err = writeImagePDF(w, pages, quality)
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

type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func writeImagePDF(out io.Writer, pages []ImagePage, quality int) error {
	if quality < 1 || quality > 100 {
		quality = 90
	}
	w := &countWriter{w: out}
	// Objects: 1 catalog, 2 pages, then per page: page, content, image.
	n := 2 + len(pages)*3
	offsets := make([]int64, n+1)
	obj := func(id int, body string) {
		offsets[id] = w.n
		fmt.Fprintf(w, "%d 0 obj\n%s\nendobj\n", id, body)
	}
	fmt.Fprint(w, "%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	obj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	kids := ""
	for i := range pages {
		kids += fmt.Sprintf("%d 0 R ", 3+i*3)
	}
	obj(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids, len(pages)))
	var jpg bytes.Buffer
	for i, p := range pages {
		pid, cid, iid := 3+i*3, 4+i*3, 5+i*3
		res := "<< >>"
		content := ""
		if p.Img != nil {
			res = fmt.Sprintf("<< /XObject << /Im0 %d 0 R >> >>", iid)
			content = fmt.Sprintf("q %.3f 0 0 %.3f %.3f %.3f cm /Im0 Do Q", p.Box[2], p.Box[3], p.Box[0], p.Box[1])
		}
		obj(pid, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.3f %.3f] /Resources %s /Contents %d 0 R >>", p.PageW, p.PageH, res, cid))
		offsets[cid] = w.n
		fmt.Fprintf(w, "%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", cid, len(content), content)
		offsets[iid] = w.n
		if p.Img == nil {
			fmt.Fprintf(w, "%d 0 obj\nnull\nendobj\n", iid)
			continue
		}
		jpg.Reset()
		img := flattenWhite(p.Img)
		if err := jpeg.Encode(&jpg, img, &jpeg.Options{Quality: quality}); err != nil {
			return err
		}
		b := img.Bounds()
		cs := "/DeviceRGB"
		if _, gray := img.(*image.Gray); gray {
			cs = "/DeviceGray"
		}
		fmt.Fprintf(w, "%d 0 obj\n<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace %s /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n",
			iid, b.Dx(), b.Dy(), cs, jpg.Len())
		if _, err := w.Write(jpg.Bytes()); err != nil {
			return err
		}
		fmt.Fprint(w, "\nendstream\nendobj\n")
	}
	xref := w.n
	fmt.Fprintf(w, "xref\n0 %d\n0000000000 65535 f \n", n+1)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(w, "%010d 00000 n \n", offsets[i])
	}
	_, err := fmt.Fprintf(w, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", n+1, xref)
	return err
}

// Paper sizes in points (portrait).
var paper = map[string][2]float64{
	"a4":     {595.28, 841.89},
	"a3":     {841.89, 1190.55},
	"a5":     {419.53, 595.28},
	"letter": {612, 792},
	"legal":  {612, 1008},
	"f4":     {609.45, 935.43}, // 215 × 330 mm (folio), common in Indonesia
}

// ImagesOptions control Images → PDF.
type ImagesOptions struct {
	PageSize    string `json:"pageSize"`    // fit | a4 | letter | legal | f4 | a3 | a5
	Orientation string `json:"orientation"` // auto | portrait | landscape
	Margin      string `json:"margin"`      // none | small | big
	Quality     int    `json:"quality"`     // JPEG quality 1..100
	Combine     bool   `json:"combine"`     // one PDF for all images
}

// LayoutImage places an image of w×h pixels on a page according to o.
func LayoutImage(o ImagesOptions, w, h int) (pageW, pageH float64, box [4]float64) {
	iw, ih := float64(w)*0.75, float64(h)*0.75 // 96 dpi
	margin := map[string]float64{"small": 20, "big": 50}[o.Margin]
	size, ok := paper[o.PageSize]
	if !ok {
		// Fit the page to the image, capped at 20 inches on the long side.
		if long := max(iw, ih); long > 1440 {
			iw, ih = iw*1440/long, ih*1440/long
		}
		pageW, pageH = iw+2*margin, ih+2*margin
		return pageW, pageH, [4]float64{margin, margin, iw, ih}
	}
	pageW, pageH = size[0], size[1]
	landscape := o.Orientation == "landscape" || (o.Orientation != "portrait" && w > h)
	if landscape {
		pageW, pageH = pageH, pageW
	}
	aw, ah := pageW-2*margin, pageH-2*margin
	// Fit inside the printable area (small images grow to fill it, like a print would).
	scale := min(aw/iw, ah/ih)
	iw, ih = iw*scale, ih*scale
	return pageW, pageH, [4]float64{(pageW - iw) / 2, (pageH - ih) / 2, iw, ih}
}

// ImagesToPDF converts image files into one PDF.
func ImagesToPDF(ctx context.Context, files []string, out string, o ImagesOptions, ffmpeg string, prog Progress) error {
	if len(files) == 0 {
		return errors.New(i18n.L("tidak ada gambar", "no images"))
	}
	pages := make([]ImagePage, 0, len(files))
	for i, f := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		img, err := imageconv.Decode(ctx, f, ffmpeg)
		if err != nil {
			return fmt.Errorf("%s: %w", baseName(f), err)
		}
		b := img.Bounds()
		pw, ph, box := LayoutImage(o, b.Dx(), b.Dy())
		pages = append(pages, ImagePage{Img: img, PageW: pw, PageH: ph, Box: box})
		prog(float64(i+1) / float64(len(files)) * 0.7)
	}
	q := o.Quality
	if q == 0 {
		q = 90
	}
	err := WriteImagePDF(out, pages, q)
	prog(1)
	return err
}

// blankPDF writes a PDF with one empty page of the given size.
func blankPDF(path string, w, h float64) error {
	if w <= 0 || h <= 0 {
		w, h = paper["a4"][0], paper["a4"][1]
	}
	return WriteImagePDF(path, []ImagePage{{PageW: w, PageH: h}}, 90)
}
