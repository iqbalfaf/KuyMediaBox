package pdf

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"sort"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

	"kuymediabox/internal/i18n"
)

// RedactOptions control redaction.
type RedactOptions struct {
	Boxes []Rect `json:"boxes"`
	DPI   int    `json:"dpi"`   // resolution of redacted pages (default 200)
	Color string `json:"color"` // box colour (default black)
}

// Redact blacks out areas for good: every page with a box is turned into an image with the
// boxes burned in, so the hidden text and graphics are gone from the file. Other pages stay
// as they are.
func Redact(ctx context.Context, in Input, o RedactOptions, out string, prog Progress) error {
	if len(o.Boxes) == 0 {
		return errors.New(i18n.L("tandai dulu area yang mau disensor", "mark the areas to redact first"))
	}
	if o.DPI < 72 || o.DPI > 600 {
		o.DPI = 200
	}
	fill := color.RGBA{0, 0, 0, 255}
	if r, g, b, ok := rgb(o.Color); ok {
		fill = color.RGBA{uint8(r * 255), uint8(g * 255), uint8(b * 255), 255}
	}
	byPage := map[int][]Rect{}
	for _, b := range o.Boxes {
		byPage[b.Page] = append(byPage[b.Page], b)
	}
	pc, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	doc, err := Open(ctx, in.Path, in.Password)
	if err != nil {
		return friendly(err)
	}
	defer doc.Close()
	pages := make([]int, 0, len(byPage))
	for p := range byPage {
		if p >= 1 && p <= pc.PageCount {
			pages = append(pages, p)
		}
	}
	sort.Ints(pages)
	widgets := map[int]bool{}
	for i, p := range pages {
		if err := ctx.Err(); err != nil {
			return err
		}
		img, err := doc.RenderDPI(p-1, o.DPI)
		if err != nil {
			return err
		}
		rgba := image.NewRGBA(img.Bounds())
		draw.Draw(rgba, rgba.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
		draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Over)
		W, H := float64(rgba.Bounds().Dx()), float64(rgba.Bounds().Dy())
		for _, b := range byPage[p] {
			r := image.Rect(int(b.X*W), int(b.Y*H), int((b.X+b.W)*W+0.999), int((b.Y+b.H)*H+0.999))
			draw.Draw(rgba, r.Intersect(rgba.Bounds()), &image.Uniform{C: fill}, image.Point{}, draw.Src)
		}
		for nr := range annotRefs(pc, p) {
			widgets[nr] = true
		}
		if err := replaceWithImage(pc, p, rgba); err != nil {
			return err
		}
		prog(float64(i+1) / float64(len(pages)) * 0.9)
	}
	dropHidden(pc, widgets)
	return writeCtx(pc, out)
}

// annotRefs returns the object numbers of a page's annotations (form widgets among them).
func annotRefs(ctx *model.Context, pageNr int) map[int]bool {
	out := map[int]bool{}
	d, _, _, err := ctx.PageDict(pageNr, false)
	if err != nil || d == nil {
		return out
	}
	o, ok := d.Find("Annots")
	if !ok {
		return out
	}
	arr, err := ctx.DereferenceArray(o)
	if err != nil {
		return out
	}
	for _, a := range arr {
		if ir, ok := a.(types.IndirectRef); ok {
			out[ir.ObjectNumber.Value()] = true
		}
	}
	return out
}

// dropHidden removes what could still reveal redacted content outside the pages: the
// form fields whose widgets were on redacted pages (their values stay in the form
// otherwise) and the structure tree of tagged PDFs (alternate texts of the old content).
func dropHidden(ctx *model.Context, widgets map[int]bool) {
	root, err := ctx.Catalog()
	if err != nil || root == nil {
		return
	}
	root.Delete("StructTreeRoot")
	root.Delete("MarkInfo")
	if len(widgets) == 0 {
		return
	}
	o, ok := root.Find("AcroForm")
	if !ok {
		return
	}
	af, err := ctx.DereferenceDict(o)
	if err != nil || af == nil {
		return
	}
	fo, ok := af.Find("Fields")
	if !ok {
		return
	}
	fields, err := ctx.DereferenceArray(fo)
	if err != nil {
		return
	}
	kept := pruneFields(ctx, fields, widgets, 0)
	if len(kept) == 0 {
		root.Delete("AcroForm")
		return
	}
	af.Update("Fields", kept)
}

// pruneFields returns the fields that keep a widget outside the redacted pages.
func pruneFields(ctx *model.Context, arr types.Array, widgets map[int]bool, depth int) types.Array {
	out := types.Array{}
	for _, f := range arr {
		if ir, ok := f.(types.IndirectRef); ok && widgets[ir.ObjectNumber.Value()] {
			continue // the field is its own widget
		}
		d, err := ctx.DereferenceDict(f)
		if err != nil || d == nil || depth > 32 {
			out = append(out, f)
			continue
		}
		if ko, ok := d.Find("Kids"); ok {
			if kids, err := ctx.DereferenceArray(ko); err == nil && len(kids) > 0 {
				left := pruneFields(ctx, kids, widgets, depth+1)
				if len(left) == 0 {
					continue
				}
				d.Update("Kids", left)
			}
		}
		out = append(out, f)
	}
	return out
}

// replaceWithImage turns page pageNr into a single full-page image (rotation baked in).
func replaceWithImage(ctx *model.Context, pageNr int, img image.Image) error {
	page, g, err := pageGeom(ctx, pageNr)
	if err != nil {
		return err
	}
	dw, dh := g.DisplaySize()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 88}); err != nil {
		return err
	}
	ref, _, _, err := model.CreateImageResource(ctx.XRefTable, &buf)
	if err != nil {
		return err
	}
	content, err := newStream(ctx, []byte("q "+num(dw)+" 0 0 "+num(dh)+" 0 0 cm /KmbPage Do Q\n"))
	if err != nil {
		return err
	}
	xo := types.NewDict()
	xo.Insert("KmbPage", *ref)
	res := types.NewDict()
	res.Insert("XObject", xo)
	page.Update("Resources", res)
	page.Update("Contents", *content)
	page.Update("MediaBox", types.NewRectangle(0, 0, dw, dh).Array())
	page.Update("Rotate", types.Integer(0))
	for _, k := range []string{"CropBox", "TrimBox", "BleedBox", "ArtBox", "Annots", "Thumb", "StructParents", "Group"} {
		page.Delete(k)
	}
	return nil
}
