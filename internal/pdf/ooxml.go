package pdf

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"image/jpeg"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"kuymediabox/internal/i18n"
)

// zipFile is one part of an OOXML package.
type zipFile struct {
	name string
	data []byte
}

func writeZip(path string, files []zipFile) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	for _, zf := range files {
		method := zip.Deflate
		if strings.HasSuffix(zf.name, ".jpeg") {
			method = zip.Store
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: zf.name, Method: method})
		if err != nil {
			f.Close()
			return err
		}
		if _, err := w.Write(zf.data); err != nil {
			f.Close()
			return err
		}
	}
	err = zw.Close()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(path)
	}
	return err
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(strings.Map(func(r rune) rune {
		// XML 1.0 forbids most control characters.
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		if r == 0xFFFE || r == 0xFFFF {
			return -1
		}
		return r
	}, s)))
	return b.String()
}

const xmlHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

// pageLines reads the text lines of every page, in points.
type pageText struct {
	W, H  float64
	Lines []Line // coordinates in points
}

func readPages(ctx context.Context, in Input, prog Progress) ([]pageText, error) {
	doc, err := Open(ctx, in.Path, in.Password)
	if err != nil {
		return nil, friendly(err)
	}
	defer doc.Close()
	out := make([]pageText, doc.Pages())
	for p := range out {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		w, h, err := doc.Size(p)
		if err != nil {
			return nil, err
		}
		chars, err := doc.Chars(p)
		if err != nil {
			return nil, err
		}
		words := Words(chars)
		for i := range words {
			words[i].X0 *= w
			words[i].X1 *= w
			words[i].Y0 *= h
			words[i].Y1 *= h
		}
		lines := Lines(words)
		out[p] = pageText{W: w, H: h, Lines: lines}
		prog(float64(p+1) / float64(len(out)) * 0.6)
	}
	return out, nil
}

// ---- Word ----------------------------------------------------------------------------------

type para struct {
	text string
	size float64
	bold bool
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[len(s)/2]
}

func paragraphs(lines []Line) []para {
	var sizes []float64
	for _, l := range lines {
		h := l.Size
		if h <= 0 {
			h = l.Y1 - l.Y0
		}
		sizes = append(sizes, h)
	}
	var out []para
	var cur *para
	var prev *Line
	for i := range lines {
		l := &lines[i]
		size := sizes[i]
		bold := true
		for _, w := range l.Words {
			bold = bold && w.Bold
		}
		text := l.Text()
		join := false
		if cur != nil && prev != nil {
			gap := l.Y0 - prev.Y1
			lh := prev.Y1 - prev.Y0
			join = gap < lh*0.8 && abs(size-cur.size) < 1.5 && bold == cur.bold && abs(l.X0-prev.X0) < lh*4
		}
		if join {
			if strings.HasSuffix(cur.text, "-") && len(text) > 0 && unicode.IsLower([]rune(text)[0]) {
				cur.text = strings.TrimSuffix(cur.text, "-") + text
			} else {
				cur.text += " " + text
			}
		} else {
			if cur != nil {
				out = append(out, *cur)
			}
			cur = &para{text: text, size: size, bold: bold}
		}
		prev = l
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// PDFToDocx writes the text of a PDF as an editable Word document (paragraphs, headings and
// page breaks; layout and pictures are not kept).
func PDFToDocx(ctx context.Context, in Input, out string, prog Progress) error {
	pages, err := readPages(ctx, in, prog)
	if err != nil {
		return err
	}
	var body strings.Builder
	any := false
	for i, p := range pages {
		if i > 0 {
			body.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
		}
		for _, pr := range paragraphs(p.Lines) {
			any = true
			sz := int(max(8, min(48, pr.size)) * 2)
			rpr := fmt.Sprintf(`<w:rPr>%s<w:sz w:val="%d"/><w:szCs w:val="%d"/></w:rPr>`, map[bool]string{true: "<w:b/>"}[pr.bold], sz, sz)
			fmt.Fprintf(&body, `<w:p><w:pPr><w:spacing w:after="120"/></w:pPr><w:r>%s<w:t xml:space="preserve">%s</w:t></w:r></w:p>`, rpr, esc(pr.text))
		}
	}
	if !any {
		return errors.New(i18n.L("PDF ini tidak berisi teks (hasil scan?) — jalankan OCR dulu", "this PDF has no text (a scan?) — run OCR first"))
	}
	w, h := 595.0, 842.0
	if len(pages) > 0 {
		w, h = pages[0].W, pages[0].H
	}
	orient := ""
	if w > h {
		orient = ` w:orient="landscape"`
	}
	doc := xmlHead + `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body.String() +
		fmt.Sprintf(`<w:sectPr><w:pgSz w:w="%d" w:h="%d"%s/><w:pgMar w:top="1134" w:right="1134" w:bottom="1134" w:left="1134" w:header="708" w:footer="708" w:gutter="0"/></w:sectPr>`, int(w*20), int(h*20), orient) +
		`</w:body></w:document>`
	styles := xmlHead + `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:cs="Calibri"/><w:sz w:val="22"/></w:rPr></w:rPrDefault></w:docDefaults></w:styles>`
	prog(0.9)
	return writeZip(out, []zipFile{
		{"[Content_Types].xml", []byte(xmlHead + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>`)},
		{"_rels/.rels", []byte(xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`)},
		{"word/_rels/document.xml.rels", []byte(xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`)},
		{"word/document.xml", []byte(doc)},
		{"word/styles.xml", []byte(styles)},
	})
}

// ---- PowerPoint ----------------------------------------------------------------------------

const pptTheme = `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office"><a:themeElements><a:clrScheme name="Office"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="1F497D"/></a:dk2><a:lt2><a:srgbClr val="EEECE1"/></a:lt2><a:accent1><a:srgbClr val="4F81BD"/></a:accent1><a:accent2><a:srgbClr val="C0504D"/></a:accent2><a:accent3><a:srgbClr val="9BBB59"/></a:accent3><a:accent4><a:srgbClr val="8064A2"/></a:accent4><a:accent5><a:srgbClr val="4BACC6"/></a:accent5><a:accent6><a:srgbClr val="F79646"/></a:accent6><a:hlink><a:srgbClr val="0000FF"/></a:hlink><a:folHlink><a:srgbClr val="800080"/></a:folHlink></a:clrScheme><a:fontScheme name="Office"><a:majorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont><a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme><a:fmtScheme name="Office"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst><a:lnStyleLst><a:ln w="9525"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="25400"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="38100"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements></a:theme>`

const pNS = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`

const emptyTree = `<p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>`

func rels(items ...string) string {
	var b strings.Builder
	b.WriteString(xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i, it := range items {
		parts := strings.SplitN(it, "|", 2)
		fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/%s" Target="%s"/>`, i+1, parts[0], parts[1])
	}
	b.WriteString(`</Relationships>`)
	return b.String()
}

// PDFToPptx turns every page into a slide showing that page.
func PDFToPptx(ctx context.Context, in Input, out string, prog Progress) error {
	doc, err := Open(ctx, in.Path, in.Password)
	if err != nil {
		return friendly(err)
	}
	defer doc.Close()
	n := doc.Pages()
	if n == 0 {
		return errors.New(i18n.L("PDF tidak punya halaman", "the PDF has no pages"))
	}
	w0, h0, err := doc.Size(0)
	if err != nil {
		return err
	}
	// Slide size follows the first page (EMU: 12700 per point), scaled to a 10-inch width.
	cx := 9144000
	cy := int(float64(cx) * h0 / w0)
	var files []zipFile
	var slideIDs, ctypes strings.Builder
	relItems := []string{"slideMaster|slideMasters/slideMaster1.xml"}
	for i := 0; i < n; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		pw, ph, _ := doc.Size(i)
		img, err := doc.RenderDPI(i, 150)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, flattenWhite(img), &jpeg.Options{Quality: 88}); err != nil {
			return err
		}
		// Fit the page into the slide, centred.
		scale := min(float64(cx)/pw, float64(cy)/ph)
		iw, ih := int(pw*scale), int(ph*scale)
		ox, oy := (cx-iw)/2, (cy-ih)/2
		k := i + 1
		files = append(files,
			zipFile{fmt.Sprintf("ppt/media/page%d.jpeg", k), buf.Bytes()},
			zipFile{fmt.Sprintf("ppt/slides/slide%d.xml", k), []byte(xmlHead + `<p:sld ` + pNS + `><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>` +
				fmt.Sprintf(`<p:pic><p:nvPicPr><p:cNvPr id="2" name="Page %d"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId2"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`, k, ox, oy, iw, ih) +
				`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`)},
			zipFile{fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", k), []byte(rels("slideLayout|../slideLayouts/slideLayout1.xml", fmt.Sprintf("image|../media/page%d.jpeg", k)))},
		)
		fmt.Fprintf(&slideIDs, `<p:sldId id="%d" r:id="rId%d"/>`, 255+k, k+1)
		relItems = append(relItems, fmt.Sprintf("slide|slides/slide%d.xml", k))
		fmt.Fprintf(&ctypes, `<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, k)
		prog(float64(k) / float64(n) * 0.9)
	}
	relItems = append(relItems, "theme|theme/theme1.xml")
	pres := xmlHead + `<p:presentation ` + pNS + `><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst><p:sldIdLst>` + slideIDs.String() + `</p:sldIdLst>` +
		fmt.Sprintf(`<p:sldSz cx="%d" cy="%d"/><p:notesSz cx="6858000" cy="9144000"/>`, cx, cy) + `</p:presentation>`
	master := xmlHead + `<p:sldMaster ` + pNS + `>` + emptyTree + `<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/><p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst></p:sldMaster>`
	layout := xmlHead + `<p:sldLayout ` + pNS + ` type="blank" preserve="1">` + emptyTree + `<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`
	files = append(files,
		zipFile{"[Content_Types].xml", []byte(xmlHead + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="jpeg" ContentType="image/jpeg"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/><Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/><Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/><Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` + ctypes.String() + `</Types>`)},
		zipFile{"_rels/.rels", []byte(xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`)},
		zipFile{"ppt/presentation.xml", []byte(pres)},
		zipFile{"ppt/_rels/presentation.xml.rels", []byte(rels(relItems...))},
		zipFile{"ppt/slideMasters/slideMaster1.xml", []byte(master)},
		zipFile{"ppt/slideMasters/_rels/slideMaster1.xml.rels", []byte(rels("slideLayout|../slideLayouts/slideLayout1.xml", "theme|../theme/theme1.xml"))},
		zipFile{"ppt/slideLayouts/slideLayout1.xml", []byte(layout)},
		zipFile{"ppt/slideLayouts/_rels/slideLayout1.xml.rels", []byte(rels("slideMaster|../slideMasters/slideMaster1.xml"))},
		zipFile{"ppt/theme/theme1.xml", []byte(xmlHead + pptTheme)},
	)
	return writeZip(out, files)
}

// ---- Excel ---------------------------------------------------------------------------------

type cell struct {
	x0, x1 float64
	text   string
}

var reNumber = regexp.MustCompile(`^-?\d{1,15}$`)

// tableRows splits the lines of a page into cells and aligns them to shared columns.
func tableRows(lines []Line) [][]string {
	var rows [][]cell
	var anchors []float64
	for _, l := range lines {
		h := l.Y1 - l.Y0
		var cells []cell
		for _, w := range l.Words {
			if n := len(cells); n > 0 && w.X0-cells[n-1].x1 < h*0.9 {
				cells[n-1].text += " " + w.Text
				cells[n-1].x1 = w.X1
				continue
			}
			cells = append(cells, cell{w.X0, w.X1, w.Text})
		}
		rows = append(rows, cells)
		for _, c := range cells {
			anchors = append(anchors, c.x0)
		}
	}
	sort.Float64s(anchors)
	var cols []float64
	for _, a := range anchors {
		if len(cols) == 0 || a-cols[len(cols)-1] > 10 {
			cols = append(cols, a)
		}
	}
	out := make([][]string, len(rows))
	for i, cells := range rows {
		row := make([]string, len(cols))
		for _, c := range cells {
			// Column whose start is closest to the left of the cell.
			best := 0
			for k, a := range cols {
				if a <= c.x0+10 {
					best = k
				}
			}
			if row[best] != "" {
				row[best] += " "
			}
			row[best] += c.text
		}
		out[i] = row
	}
	// Drop columns that are empty everywhere.
	keep := make([]bool, len(cols))
	for _, r := range out {
		for k, v := range r {
			if v != "" {
				keep[k] = true
			}
		}
	}
	for i, r := range out {
		var nr []string
		for k, v := range r {
			if keep[k] {
				nr = append(nr, v)
			}
		}
		out[i] = nr
	}
	return out
}

func colName(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}

// PDFToXlsx puts the text of each page into a worksheet, lined up in columns so tables
// become spreadsheet tables.
func PDFToXlsx(ctx context.Context, in Input, out string, prog Progress) error {
	pages, err := readPages(ctx, in, prog)
	if err != nil {
		return err
	}
	var files []zipFile
	var sheets, wbRels, ctypes strings.Builder
	anyText := false
	for i, p := range pages {
		rows := tableRows(p.Lines)
		var sd strings.Builder
		widths := map[int]int{}
		for r, row := range rows {
			fmt.Fprintf(&sd, `<row r="%d">`, r+1)
			for c, v := range row {
				if v == "" {
					continue
				}
				anyText = true
				widths[c] = max(widths[c], len([]rune(v)))
				ref := fmt.Sprintf("%s%d", colName(c), r+1)
				if reNumber.MatchString(v) {
					fmt.Fprintf(&sd, `<c r="%s"><v>%s</v></c>`, ref, v)
				} else {
					fmt.Fprintf(&sd, `<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, esc(v))
				}
			}
			sd.WriteString(`</row>`)
		}
		var cols strings.Builder
		if len(widths) > 0 {
			cols.WriteString("<cols>")
			for c := 0; c < 512; c++ {
				if w, ok := widths[c]; ok {
					fmt.Fprintf(&cols, `<col min="%d" max="%d" width="%d" customWidth="1"/>`, c+1, c+1, min(60, max(8, w+2)))
				}
			}
			cols.WriteString("</cols>")
		}
		k := i + 1
		files = append(files, zipFile{fmt.Sprintf("xl/worksheets/sheet%d.xml", k), []byte(xmlHead +
			`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` + cols.String() + `<sheetData>` + sd.String() + `</sheetData></worksheet>`)})
		fmt.Fprintf(&sheets, `<sheet name="%s %d" sheetId="%d" r:id="rId%d"/>`, i18n.L("Hal", "Page"), k, k, k)
		fmt.Fprintf(&wbRels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, k, k)
		fmt.Fprintf(&ctypes, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, k)
	}
	if !anyText {
		return errors.New(i18n.L("PDF ini tidak berisi teks (hasil scan?) — jalankan OCR dulu", "this PDF has no text (a scan?) — run OCR first"))
	}
	files = append(files,
		zipFile{"[Content_Types].xml", []byte(xmlHead + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` + ctypes.String() + `</Types>`)},
		zipFile{"_rels/.rels", []byte(xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`)},
		zipFile{"xl/workbook.xml", []byte(xmlHead + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` + sheets.String() + `</sheets></workbook>`)},
		zipFile{"xl/_rels/workbook.xml.rels", []byte(xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + wbRels.String() + `</Relationships>`)},
	)
	prog(0.95)
	return writeZip(out, files)
}
