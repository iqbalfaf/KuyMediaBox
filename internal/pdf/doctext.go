package pdf

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"kuymediabox/internal/i18n"
)

// docPages is how many pages of a PDF DocLines reads: the identity block of a report
// card is at the start.
const docPages = 3

// ReadsNatively reports whether DocLines reads the file without Office or LibreOffice.
func ReadsNatively(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf", ".docx", ".docm", ".dotx", ".odt":
		return true
	}
	return false
}

// DocLines reads the text of a PDF or Word document as lines; "\t" separates table cells
// and columns far apart, so a label and its value can be told from the next column.
func DocLines(ctx context.Context, in Input, tmpDir string, eng Engines) ([]string, error) {
	var lines []string
	var err error
	switch strings.ToLower(filepath.Ext(in.Path)) {
	case ".pdf":
		lines, err = pdfLines(ctx, in)
	case ".docx", ".docm", ".dotx":
		lines, err = zipXMLLines(in.Path, "word/document.xml", false)
	case ".odt":
		lines, err = zipXMLLines(in.Path, "content.xml", true)
	default:
		// Old .doc, .rtf, …: let Office or LibreOffice print it to PDF first.
		dir, derr := os.MkdirTemp(tmpDir, "rename-")
		if derr != nil {
			return nil, derr
		}
		defer os.RemoveAll(dir)
		out := filepath.Join(dir, "doc.pdf")
		if _, err = OfficeToPDF(ctx, in.Path, out, dir, eng); err != nil {
			return nil, err
		}
		lines, err = pdfLines(ctx, Input{Path: out})
	}
	if err != nil {
		return nil, err
	}
	for i, l := range lines {
		lines[i] = tidyLine(l)
	}
	return lines, nil
}

func pdfLines(ctx context.Context, in Input) ([]string, error) {
	doc, err := Open(ctx, in.Path, in.Password)
	if err != nil {
		return nil, friendly(err)
	}
	defer doc.Close()
	var out []string
	for p := 0; p < doc.Pages() && p < docPages; p++ {
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
		for _, l := range Lines(words) {
			out = append(out, lineWithColumns(l))
		}
	}
	if strings.TrimSpace(strings.Join(out, "")) == "" {
		return nil, errors.New(i18n.L("tidak ada teks di PDF ini (hasil scan?) — jalankan OCR PDF dulu", "this PDF has no text (a scan?) — run OCR PDF first"))
	}
	return out, nil
}

// lineWithColumns joins the words of a line, with a tab where the gap is wider than about
// four spaces (the next column of a form or table).
func lineWithColumns(l Line) string {
	var b strings.Builder
	for i, w := range l.Words {
		if i > 0 {
			prev := l.Words[i-1]
			em := max(prev.Size, w.Size)
			if em <= 0 {
				em = l.Y1 - l.Y0
			}
			if w.X0-prev.X1 > em {
				b.WriteByte('\t')
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteString(w.Text)
	}
	return b.String()
}

// zipXMLLines reads the text of a Word (.docx) or OpenDocument (.odt) file: paragraphs
// become lines, table rows become lines with their cells split by tabs.
func zipXMLLines(path, part string, odt bool) ([]string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, errors.New(i18n.L("dokumen rusak atau bukan file Word", "the document is damaged or not a Word file"))
	}
	defer zr.Close()
	f, err := zr.Open(part)
	if err != nil {
		return nil, errors.New(i18n.L("dokumen rusak atau bukan file Word", "the document is damaged or not a Word file"))
	}
	defer f.Close()
	lines := xmlLines(f, odt)
	if strings.TrimSpace(strings.Join(lines, "")) == "" {
		return nil, errors.New(i18n.L("dokumen ini tidak berisi teks", "this document has no text"))
	}
	return lines, nil
}

func xmlLines(r io.Reader, odt bool) []string {
	// Element names (local part) of both formats.
	para, cell, row, tab, brk, text := "p", "tc", "tr", "tab", "br", "t"
	if odt {
		cell, row, tab, brk = "table-cell", "table-row", "tab", "line-break"
	}
	var lines []string
	var cur strings.Builder
	// inStops: Word lists a paragraph's tab stops as <w:tab> too (inside <w:tabs>).
	inTable, inText, inStops := 0, 0, 0
	newline := func() {
		lines = append(lines, cur.String())
		cur.Reset()
	}
	dec := xml.NewDecoder(r)
	dec.Strict = false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch name := t.Name.Local; {
			case name == cell:
				inTable++
			case !odt && name == "tabs":
				inStops++
			case name == tab:
				if inStops == 0 {
					cur.WriteByte('\t')
				}
			case name == brk || !odt && name == "cr":
				if inTable > 0 {
					cur.WriteByte(' ')
				} else {
					newline()
				}
			case odt && name == "s":
				cur.WriteByte(' ')
			case odt && (name == "p" || name == "h"), !odt && name == text:
				inText++
			}
		case xml.EndElement:
			switch name := t.Name.Local; {
			case name == cell:
				inTable--
				cur.WriteByte('\t')
			case name == row:
				newline()
			case !odt && name == "tabs":
				inStops--
			case odt && (name == "p" || name == "h"), !odt && name == text:
				inText--
			}
			// A paragraph ends the line, except inside a table cell where it is a space.
			if t.Name.Local == para || odt && t.Name.Local == "h" {
				if inTable > 0 {
					cur.WriteByte(' ')
				} else {
					newline()
				}
			}
		case xml.CharData:
			if inText > 0 {
				cur.Write(t)
			}
		}
	}
	if cur.Len() > 0 {
		newline()
	}
	return lines
}

var (
	spaceRun = regexp.MustCompile(`[ \p{Zs}]+`)
	tabRun   = regexp.MustCompile(` *\t[ \t]*`)
)

// tidyLine collapses runs of spaces, no-break spaces included (keeping tabs as column
// breaks).
func tidyLine(s string) string {
	s = spaceRun.ReplaceAllString(s, " ")
	s = tabRun.ReplaceAllString(s, "\t")
	return strings.Trim(s, " ")
}
