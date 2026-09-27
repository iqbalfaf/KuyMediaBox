package pdf

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/form"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

	"kuymediabox/internal/i18n"
)

// ---- header & footer -------------------------------------------------------------------------

// HeaderFooterOptions put text in the six corners and middles of the page edges.
// Text can use {n} (page), {total}, {date} and {file}.
type HeaderFooterOptions struct {
	TopLeft      string  `json:"topLeft"`
	TopCenter    string  `json:"topCenter"`
	TopRight     string  `json:"topRight"`
	BottomLeft   string  `json:"bottomLeft"`
	BottomCenter string  `json:"bottomCenter"`
	BottomRight  string  `json:"bottomRight"`
	Size         float64 `json:"size"`
	Color        string  `json:"color"`
	Bold         bool    `json:"bold"`
	Margin       float64 `json:"margin"` // points from the edge
	Line         bool    `json:"line"`   // a thin rule under the header / above the footer
	Pages        string  `json:"pages"`  // "" = every page
	SkipFirst    bool    `json:"skipFirst"`
	Mirror       bool    `json:"mirror"` // swap left and right on even pages
}

// AddHeaderFooter writes the header and footer texts on the selected pages.
func AddHeaderFooter(in Input, o HeaderFooterOptions, out string) error {
	slots := map[string]string{"tl": o.TopLeft, "tc": o.TopCenter, "tr": o.TopRight, "bl": o.BottomLeft, "bc": o.BottomCenter, "br": o.BottomRight}
	empty := true
	for _, v := range slots {
		if strings.TrimSpace(v) != "" {
			empty = false
		}
	}
	if empty {
		return errors.New(i18n.L("isi teks header atau footer", "enter a header or footer text"))
	}
	ctx, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	pages, err := selected(o.Pages, ctx.PageCount)
	if err != nil {
		return err
	}
	if o.Size <= 0 {
		o.Size = 9
	}
	if o.Margin <= 0 {
		o.Margin = 24
	}
	if o.Color == "" {
		o.Color = "#444444"
	}
	sizes, err := DisplaySizes(ctx)
	if err != nil {
		return err
	}
	file := strings.TrimSuffix(filepath.Base(in.Path), filepath.Ext(in.Path))
	date := time.Now().Format("02/01/2006")
	if i18n.Lang() == "en" {
		date = time.Now().Format("Jan 2, 2006")
	}
	for _, p := range pages {
		if o.SkipFirst && p == 1 {
			continue
		}
		fill := strings.NewReplacer("{n}", strconv.Itoa(p), "{total}", strconv.Itoa(ctx.PageCount), "{N}", strconv.Itoa(ctx.PageCount), "{date}", date, "{file}", file)
		dw, dh := sizes[p-1].W, sizes[p-1].H
		var items []Item
		hasTop, hasBottom := false, false
		for pos, text := range slots {
			if strings.TrimSpace(text) == "" {
				continue
			}
			col := pos[1]
			if o.Mirror && p%2 == 0 {
				switch col {
				case 'l':
					col = 'r'
				case 'r':
					col = 'l'
				}
			}
			it := Item{Kind: "text", Text: fill.Replace(text), Size: o.Size, Bold: o.Bold, Color: o.Color}
			switch col {
			case 'l':
				it.X, it.Align = o.Margin, "left"
			case 'r':
				it.X, it.Align = dw-o.Margin, "right"
			default:
				it.X, it.Align = dw/2, "center"
			}
			if pos[0] == 't' {
				it.Y = o.Margin + o.Size*0.72
				hasTop = true
			} else {
				it.Y = dh - o.Margin
				hasBottom = true
			}
			items = append(items, it)
		}
		if o.Line {
			if hasTop {
				y := o.Margin + o.Size*0.72 + 5
				items = append(items, Item{Kind: "line", X: o.Margin, Y: y, X2: dw - o.Margin, Y2: y, Color: o.Color, Stroke: 0.5, Opacity: 1})
			}
			if hasBottom {
				y := dh - o.Margin - o.Size - 3
				items = append(items, Item{Kind: "line", X: o.Margin, Y: y, X2: dw - o.Margin, Y2: y, Color: o.Color, Stroke: 0.5, Opacity: 1})
			}
		}
		if err := drawOnPage(ctx, p, items, false); err != nil {
			return err
		}
	}
	return writeCtx(ctx, out)
}

// ---- n-up & booklet --------------------------------------------------------------------------

// NUpOptions put several pages on one sheet.
type NUpOptions struct {
	Mode   string `json:"mode"`   // nup | booklet
	N      int    `json:"n"`      // pages per sheet: 2, 4, 6, 8, 9, 12, 16 (booklet: 2 or 4)
	Paper  string `json:"paper"`  // A4, A3, Letter, F4 …
	Border bool   `json:"border"` // frame around every page
	Margin int    `json:"margin"` // points between pages
}

// NUp writes the pages n-up (or as a booklet for printing on both sides and folding).
func NUp(in Input, o NUpOptions, out string) error {
	c := conf(in.Password)
	paper := strings.ToUpper(strings.TrimSpace(o.Paper))
	if paper == "" {
		paper = "A4"
	}
	// Two pages side by side fit upright on a landscape sheet; every other layout (and the
	// booklets, whose back sides pdfcpu turns for long-edge duplex) uses portrait sheets.
	landscape := o.Mode != "booklet" && o.N == 2
	desc := "formsize:" + paper
	if landscape {
		desc += "L"
	}
	if paper == "F4" {
		desc = "dimensions:609 935" // Indonesian F4 / folio, 215 × 330 mm in points
		if landscape {
			desc = "dimensions:935 609"
		}
	}
	if o.Border {
		desc += ", border:on"
	} else {
		desc += ", border:off"
	}
	if o.Margin > 0 {
		desc += ", margin:" + strconv.Itoa(min(o.Margin, 72))
	}
	var nup *model.NUp
	var err error
	if o.Mode == "booklet" {
		if o.N != 4 {
			o.N = 2
		}
		nup, err = api.PDFBookletConfig(o.N, desc, c)
	} else {
		switch o.N {
		case 2, 3, 4, 6, 8, 9, 12, 16:
		default:
			o.N = 2
		}
		nup, err = api.PDFNUpConfig(o.N, desc, c)
	}
	if err != nil {
		return friendly(err)
	}
	src, err := os.ReadFile(in.Path)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if o.Mode == "booklet" {
		err = api.Booklet(bytes.NewReader(src), &buf, nil, nil, nup, c)
	} else {
		err = api.NUp(bytes.NewReader(src), &buf, nil, nil, nup, c)
	}
	if err != nil {
		return friendly(err)
	}
	// Re-save without the source's encryption.
	tmp := out + ".nup"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	defer os.Remove(tmp)
	ctx, err := readCtx(tmp, "")
	if err != nil {
		return err
	}
	return writeCtx(ctx, out)
}

// ---- metadata & bookmarks --------------------------------------------------------------------

// Meta are the document properties shown by PDF readers.
type Meta struct {
	Title    string `json:"title"`
	Author   string `json:"author"`
	Subject  string `json:"subject"`
	Keywords string `json:"keywords"`
	Creator  string `json:"creator"`
}

// Bookmark is one entry of the table of contents (page is 1-based).
type Bookmark struct {
	Title string     `json:"title"`
	Page  int        `json:"page"`
	Kids  []Bookmark `json:"kids"`
}

// DocDetails are a document's properties, bookmarks and page count.
type DocDetails struct {
	Meta      Meta       `json:"meta"`
	Bookmarks []Bookmark `json:"bookmarks"`
	Pages     int        `json:"pages"`
}

var infoKeys = map[string]string{"Title": "title", "Author": "author", "Subject": "subject", "Keywords": "keywords", "Creator": "creator"}

// ReadDetails reads the properties and bookmarks of a PDF.
func ReadDetails(in Input) (DocDetails, error) {
	ctx, err := readCtx(in.Path, in.Password)
	if err != nil {
		return DocDetails{}, err
	}
	d := DocDetails{Pages: ctx.PageCount, Bookmarks: []Bookmark{}}
	if ctx.Info != nil {
		if info, err := ctx.DereferenceDict(*ctx.Info); err == nil && info != nil {
			get := func(k string) string {
				o, ok := info.Find(k)
				if !ok {
					return ""
				}
				o, _ = ctx.Dereference(o)
				switch v := o.(type) {
				case types.StringLiteral:
					s, _ := types.StringLiteralToString(v)
					return s
				case types.HexLiteral:
					s, _ := types.HexLiteralToString(v)
					return s
				}
				return ""
			}
			d.Meta = Meta{Title: get("Title"), Author: get("Author"), Subject: get("Subject"), Keywords: get("Keywords"), Creator: get("Creator")}
		}
	}
	f, err := os.Open(in.Path)
	if err == nil {
		defer f.Close()
		if bms, err := api.Bookmarks(f, conf(in.Password)); err == nil {
			d.Bookmarks = fromPdfcpu(bms)
		}
	}
	return d, nil
}

func fromPdfcpu(bms []pdfcpu.Bookmark) []Bookmark {
	out := make([]Bookmark, 0, len(bms))
	for _, b := range bms {
		out = append(out, Bookmark{Title: b.Title, Page: b.PageFrom, Kids: fromPdfcpu(b.Kids)})
	}
	return out
}

func toPdfcpu(bms []Bookmark, pages int) []pdfcpu.Bookmark {
	var out []pdfcpu.Bookmark
	for _, b := range bms {
		if strings.TrimSpace(b.Title) == "" {
			continue
		}
		out = append(out, pdfcpu.Bookmark{Title: strings.TrimSpace(b.Title), PageFrom: max(1, min(b.Page, pages)), Kids: toPdfcpu(b.Kids, pages)})
	}
	return out
}

// WriteDetails saves new properties and bookmarks (nil bookmarks keeps the existing ones,
// an empty list removes them).
func WriteDetails(in Input, m Meta, bms []Bookmark, out string) error {
	ctx, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	info := types.Dict{}
	if ctx.Info != nil {
		if d, err := ctx.DereferenceDict(*ctx.Info); err == nil && d != nil {
			info = d
		}
	}
	vals := map[string]string{"Title": m.Title, "Author": m.Author, "Subject": m.Subject, "Keywords": m.Keywords, "Creator": m.Creator}
	for k := range infoKeys {
		v := strings.TrimSpace(vals[k])
		if v == "" {
			delete(info, k)
			continue
		}
		s, err := types.EscapedUTF16String(v)
		if err != nil {
			return err
		}
		info[k] = types.StringLiteral(*s)
	}
	if ctx.Info != nil {
		if entry, ok := ctx.FindTableEntryForIndRef(ctx.Info); ok {
			entry.Object = info
		}
	} else if ir, err := ctx.IndRefForNewObject(info); err == nil {
		ctx.Info = ir
	}
	tmp := out + ".meta"
	if err := writeCtx(ctx, tmp); err != nil {
		return err
	}
	defer os.Remove(tmp)
	if bms == nil {
		return os.Rename(tmp, out)
	}
	src, err := os.ReadFile(tmp)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	c := conf("")
	if len(bms) == 0 {
		err = api.RemoveBookmarks(bytes.NewReader(src), &buf, c)
		if err != nil && strings.Contains(err.Error(), "no outlines") {
			return os.Rename(tmp, out)
		}
	} else {
		err = api.AddBookmarks(bytes.NewReader(src), &buf, toPdfcpu(bms, ctx.PageCount), true, c)
	}
	if err != nil {
		return friendly(err)
	}
	return os.WriteFile(out, buf.Bytes(), 0o644)
}

// ---- forms -----------------------------------------------------------------------------------

// FormField is one fillable field of a PDF form.
// NoFormError reports a PDF without fillable fields.
type NoFormError struct{}

func (NoFormError) Error() string {
	return i18n.L("PDF ini tidak punya formulir yang bisa diisi", "This PDF has no fillable form")
}

type FormField struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"` // text | date | check | radio | combo | list
	Value     string   `json:"value"`
	Values    []string `json:"values"` // list boxes
	Checked   bool     `json:"checked"`
	Options   []string `json:"options"`
	Multiline bool     `json:"multiline"`
	Multi     bool     `json:"multi"`
	Locked    bool     `json:"locked"`
	Page      int      `json:"page"`
	Format    string   `json:"format"` // date fields
}

func firstPage(p []int) int {
	if len(p) == 0 {
		return 0
	}
	return p[0]
}

func exportForm(in Input) (*form.FormGroup, error) {
	f, err := os.Open(in.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fg, err := api.ExportForm(f, filepath.Base(in.Path), conf(in.Password))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no form") || strings.Contains(strings.ToLower(err.Error()), "acroform") {
			return nil, NoFormError{}
		}
		return nil, friendly(err)
	}
	if len(fg.Forms) == 0 {
		return nil, NoFormError{}
	}
	return fg, nil
}

// ReadForm lists the fields of a PDF form in page order.
func ReadForm(in Input) ([]FormField, error) {
	fg, err := exportForm(in)
	if err != nil {
		return nil, err
	}
	fm := fg.Forms[0]
	var out []FormField
	name := func(n, alt, id string) string {
		if alt != "" {
			return alt
		}
		if n != "" {
			return n
		}
		return id
	}
	for _, t := range fm.TextFields {
		out = append(out, FormField{ID: t.ID, Name: name(t.Name, t.AltName, t.ID), Kind: "text", Value: t.Value, Multiline: t.Multiline, Locked: t.Locked, Page: firstPage(t.Pages)})
	}
	for _, t := range fm.DateFields {
		out = append(out, FormField{ID: t.ID, Name: name(t.Name, t.AltName, t.ID), Kind: "date", Value: t.Value, Format: t.Format, Locked: t.Locked, Page: firstPage(t.Pages)})
	}
	for _, t := range fm.CheckBoxes {
		out = append(out, FormField{ID: t.ID, Name: name(t.Name, t.AltName, t.ID), Kind: "check", Checked: t.Value, Locked: t.Locked, Page: firstPage(t.Pages)})
	}
	for _, t := range fm.RadioButtonGroups {
		out = append(out, FormField{ID: t.ID, Name: name(t.Name, t.AltName, t.ID), Kind: "radio", Value: t.Value, Options: t.Options, Locked: t.Locked, Page: firstPage(t.Pages)})
	}
	for _, t := range fm.ComboBoxes {
		out = append(out, FormField{ID: t.ID, Name: name(t.Name, t.AltName, t.ID), Kind: "combo", Value: t.Value, Options: t.Options, Locked: t.Locked, Page: firstPage(t.Pages)})
	}
	for _, t := range fm.ListBoxes {
		out = append(out, FormField{ID: t.ID, Name: name(t.Name, t.AltName, t.ID), Kind: "list", Values: t.Values, Options: t.Options, Multi: t.Multi, Locked: t.Locked, Page: firstPage(t.Pages)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Page < out[j].Page })
	return out, nil
}

// FillForm writes the given values into the form; flatten makes the result a plain,
// non-editable document.
func FillForm(ctx context.Context, in Input, fields []FormField, flatten bool, out string) error {
	fg, err := exportForm(in)
	if err != nil {
		return err
	}
	byID := map[string]FormField{}
	for _, f := range fields {
		byID[f.ID] = f
	}
	fm := &fg.Forms[0]
	for _, t := range fm.TextFields {
		if f, ok := byID[t.ID]; ok {
			t.Value = f.Value
		}
	}
	for _, t := range fm.DateFields {
		if f, ok := byID[t.ID]; ok {
			t.Value = f.Value
		}
	}
	for _, t := range fm.CheckBoxes {
		if f, ok := byID[t.ID]; ok {
			t.Value = f.Checked
		}
	}
	for _, t := range fm.RadioButtonGroups {
		if f, ok := byID[t.ID]; ok {
			t.Value = f.Value
		}
	}
	for _, t := range fm.ComboBoxes {
		if f, ok := byID[t.ID]; ok {
			t.Value = f.Value
		}
	}
	for _, t := range fm.ListBoxes {
		if f, ok := byID[t.ID]; ok {
			t.Values = f.Values
		}
	}
	if flatten {
		// pdfcpu draws choice fields (combo & list boxes) only for locked fields; flattening
		// removes the fields anyway.
		for _, t := range fm.TextFields {
			t.Locked = true
		}
		for _, t := range fm.DateFields {
			t.Locked = true
		}
		for _, t := range fm.CheckBoxes {
			t.Locked = true
		}
		for _, t := range fm.RadioButtonGroups {
			t.Locked = true
		}
		for _, t := range fm.ComboBoxes {
			t.Locked = true
		}
		for _, t := range fm.ListBoxes {
			t.Locked = true
		}
	}
	data, err := json.Marshal(fg)
	if err != nil {
		return err
	}
	src, err := os.ReadFile(in.Path)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := api.FillForm(bytes.NewReader(src), bytes.NewReader(data), &buf, conf(in.Password)); err != nil {
		return friendly(err)
	}
	filled := out + ".form"
	if err := os.WriteFile(filled, buf.Bytes(), 0o644); err != nil {
		return err
	}
	defer os.Remove(filled)
	if flatten {
		return Flatten(ctx, Input{Path: filled}, out)
	}
	c, err := readCtx(filled, "")
	if err != nil {
		return err
	}
	return writeCtx(c, out)
}

// Flatten turns form fields and annotations into ordinary page content.
func Flatten(ctx context.Context, in Input, out string) error {
	doc, err := Open(ctx, in.Path, in.Password)
	if err != nil {
		return friendly(err)
	}
	defer doc.Close()
	for i := 0; i < doc.Pages(); i++ {
		if _, err := doc.inst.FPDFPage_Flatten(&requests.FPDFPage_Flatten{Page: doc.page(i), Usage: requests.FPDFPage_FlattenUsagePrint}); err != nil {
			return engineErr(err)
		}
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := doc.SaveCopy(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ---- text, Markdown, CSV ---------------------------------------------------------------------

// PDFToText saves the text of a PDF: plain (one blank line between paragraphs, pages
// separated by a form feed) or Markdown (bigger or bold lines become headings).
func PDFToText(ctx context.Context, in Input, markdown bool, out string, prog Progress) error {
	pages, err := readPages(ctx, in, prog)
	if err != nil {
		return err
	}
	var all []float64
	for _, p := range pages {
		for _, l := range p.Lines {
			if l.Size > 0 {
				all = append(all, l.Size)
			}
		}
	}
	body := median(all)
	var b strings.Builder
	anyText := false
	for i, p := range pages {
		if i > 0 {
			if markdown {
				b.WriteString("\n---\n\n")
			} else {
				b.WriteString("\f\n")
			}
		}
		for _, seg := range segments(p.Lines) {
			if seg.Table != nil {
				anyText = true
				if markdown {
					b.WriteString(mdTable(seg.Table))
				} else {
					for _, row := range seg.Table {
						b.WriteString(strings.Join(row, "\t"))
						b.WriteString("\n")
					}
				}
				b.WriteString("\n")
				continue
			}
			for _, para := range paragraphs(seg.Lines) {
				t := strings.TrimSpace(para.text)
				if t == "" {
					continue
				}
				anyText = true
				if markdown {
					switch {
					case body > 0 && para.size >= body*1.6:
						t = "# " + t
					case body > 0 && para.size >= body*1.25:
						t = "## " + t
					case para.bold && len([]rune(t)) < 90 && !strings.HasSuffix(t, "."):
						t = "### " + t
					default:
						t = mdList(t)
					}
				}
				b.WriteString(t)
				b.WriteString("\n\n")
			}
		}
	}
	if !anyText {
		return errors.New(i18n.L("PDF ini tidak berisi teks (hasil scan?). Jalankan OCR PDF dulu.", "This PDF has no text (a scan?). Run OCR PDF first."))
	}
	prog(1)
	return os.WriteFile(out, []byte(strings.TrimRight(b.String(), "\n")+"\n"), 0o644)
}

// mdTable writes rows as a Markdown table; the first row is the header.
func mdTable(rows [][]string) string {
	n := 0
	for _, r := range rows {
		n = max(n, len(r))
	}
	var b strings.Builder
	line := func(r []string) {
		b.WriteString("|")
		for i := 0; i < n; i++ {
			v := ""
			if i < len(r) {
				v = strings.ReplaceAll(r[i], "|", `\|`)
			}
			b.WriteString(" " + v + " |")
		}
		b.WriteString("\n")
	}
	line(rows[0])
	b.WriteString("|" + strings.Repeat(" --- |", n) + "\n")
	for _, r := range rows[1:] {
		line(r)
	}
	return b.String()
}

// mdList turns "• item" / "- item" / "1) item" into Markdown list items.
func mdList(t string) string {
	for _, bullet := range []string{"•", "●", "▪", "◦", "–", "- ", "* "} {
		if strings.HasPrefix(t, bullet) {
			return "- " + strings.TrimSpace(strings.TrimPrefix(t, bullet))
		}
	}
	return t
}

// PDFToCSV writes the tables of a PDF as CSV (all pages one after another; Excel opens it
// directly). semicolon suits Excel in languages that use a decimal comma.
func PDFToCSV(ctx context.Context, in Input, semicolon bool, out string, prog Progress) error {
	pages, err := readPages(ctx, in, prog)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF}) // Excel reads UTF-8 only with a BOM
	w := csv.NewWriter(&buf)
	if semicolon {
		w.Comma = ';'
	}
	// Only the tables; a PDF without recognisable tables gets all its lines instead.
	var tables [][][]string
	for _, p := range pages {
		for _, seg := range segments(p.Lines) {
			if seg.Table != nil {
				tables = append(tables, seg.Table)
			}
		}
	}
	if len(tables) == 0 {
		for _, p := range pages {
			tables = append(tables, tableRows(p.Lines))
		}
	}
	rows := 0
	for _, t := range tables {
		if rows > 0 {
			w.Write(nil) // a blank row between tables
		}
		for _, row := range t {
			if len(row) == 0 {
				continue
			}
			if err := w.Write(row); err != nil {
				return err
			}
			rows++
		}
	}
	w.Flush()
	if rows == 0 {
		return errors.New(i18n.L("Tidak ada teks/tabel di PDF ini (hasil scan?). Jalankan OCR PDF dulu.", "No text/tables in this PDF (a scan?). Run OCR PDF first."))
	}
	prog(1)
	return os.WriteFile(out, buf.Bytes(), 0o644)
}

var _ = fmt.Sprint
