package pdf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

	"kuymediabox/internal/i18n"
)

// Input is a source PDF with its password (empty when not protected).
type Input struct {
	Path     string `json:"path"`
	Password string `json:"password"`
}

func baseName(p string) string { return filepath.Base(p) }

// openPlain reads a source and makes sure its objects are decrypted, so pages can be
// copied into other documents.
func openPlain(in Input, tmpDir string) (*model.Context, error) {
	ctx, err := readCtx(in.Path, in.Password)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", baseName(in.Path), err)
	}
	if ctx.Encrypt == nil {
		return ctx, nil
	}
	tmp, err := os.CreateTemp(tmpDir, "plain-*.pdf")
	if err != nil {
		return nil, err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)
	if err := writeCtx(ctx, name); err != nil {
		return nil, fmt.Errorf("%s: %w", baseName(in.Path), err)
	}
	return readCtx(name, "")
}

func mergeInto(dest, src *model.Context, label string) error {
	if dest.XRefTable.Version() < model.V20 && src.XRefTable.Version() == model.V20 {
		return errors.New(i18n.L("PDF 2.0 tidak bisa digabung ke PDF versi lama", "PDF 2.0 can't be merged into an older PDF"))
	}
	if err := pdfcpu.MergeXRefTables(label, src, dest, false, false); err != nil {
		return err
	}
	return dest.EnsurePageCount()
}

// Merge joins PDFs in order into out.
func Merge(ctx context.Context, inputs []Input, out, tmpDir string, prog Progress) error {
	if len(inputs) < 2 {
		return errors.New(i18n.L("pilih minimal 2 PDF untuk digabung", "choose at least 2 PDFs to merge"))
	}
	dest, err := openPlain(inputs[0], tmpDir)
	if err != nil {
		return err
	}
	dest.EnsureVersionForWriting()
	for i, in := range inputs[1:] {
		if err := ctx.Err(); err != nil {
			return err
		}
		src, err := openPlain(in, tmpDir)
		if err != nil {
			return err
		}
		if err := mergeInto(dest, src, fmt.Sprint(i+1)); err != nil {
			return fmt.Errorf("%s: %w", baseName(in.Path), err)
		}
		prog(float64(i+2) / float64(len(inputs)) * 0.85)
	}
	return writeCtx(dest, out)
}

// extractTo writes the given 1-based pages (in order, repeats allowed) of src to out.
func extractTo(src *model.Context, pages []int, out string) error {
	dst, err := pdfcpu.ExtractPages(src, pages, false)
	if err != nil {
		return friendly(err)
	}
	return writeCtx(dst, out)
}

// SplitOptions control splitting.
type SplitOptions struct {
	Mode   string `json:"mode"`   // ranges | every | all
	Ranges string `json:"ranges"` // "1-3, 4-8" for ranges
	Every  int    `json:"every"`  // pages per file for every
}

// Split writes parts of in into dir as name-1.pdf, name-2.pdf, …; it returns the files.
func Split(ctx context.Context, in Input, o SplitOptions, dir, name string, prog Progress) ([]string, error) {
	src, err := readCtx(in.Path, in.Password)
	if err != nil {
		return nil, err
	}
	n := src.PageCount
	var spans [][2]int
	switch o.Mode {
	case "ranges":
		spans, err = ParseRanges(o.Ranges, n)
		if err != nil {
			return nil, err
		}
		if len(spans) == 0 {
			return nil, errors.New(i18n.L("isi rentang halaman, mis. 1-3, 4-8", "enter page ranges, e.g. 1-3, 4-8"))
		}
	case "every":
		every := max(1, o.Every)
		for a := 1; a <= n; a += every {
			spans = append(spans, [2]int{a, min(n, a+every-1)})
		}
	default:
		for p := 1; p <= n; p++ {
			spans = append(spans, [2]int{p, p})
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var files []string
	for i, sp := range spans {
		if err := ctx.Err(); err != nil {
			return files, err
		}
		var pages []int
		for p := sp[0]; p <= sp[1]; p++ {
			pages = append(pages, p)
		}
		label := fmt.Sprintf("%d", sp[0])
		if sp[1] != sp[0] {
			label = fmt.Sprintf("%d-%d", sp[0], sp[1])
		}
		out := filepath.Join(dir, fmt.Sprintf("%s_%s.pdf", name, label))
		for k := 2; fileExists(out); k++ {
			out = filepath.Join(dir, fmt.Sprintf("%s_%s (%d).pdf", name, label, k))
		}
		if err := extractTo(src, pages, out); err != nil {
			return files, err
		}
		files = append(files, out)
		prog(float64(i+1) / float64(len(spans)))
	}
	return files, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// RemovePages deletes the given pages (e.g. "2, 5-7").
func RemovePages(in Input, sel, out string) error {
	src, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	drop, err := ParsePages(sel, src.PageCount)
	if err != nil {
		return err
	}
	if len(drop) == 0 {
		return errors.New(i18n.L("pilih halaman yang mau dihapus", "choose the pages to remove"))
	}
	if len(drop) >= src.PageCount {
		return errors.New(i18n.L("tidak bisa menghapus semua halaman", "can't remove every page"))
	}
	gone := map[int]bool{}
	for _, p := range drop {
		gone[p] = true
	}
	var keep []int
	for p := 1; p <= src.PageCount; p++ {
		if !gone[p] {
			keep = append(keep, p)
		}
	}
	return extractTo(src, keep, out)
}

// ExtractPages writes the selected pages into one new PDF.
func ExtractPages(in Input, sel, out string) error {
	src, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	pages, err := ParsePages(sel, src.PageCount)
	if err != nil {
		return err
	}
	if len(pages) == 0 {
		return errors.New(i18n.L("pilih halaman yang mau diambil", "choose the pages to extract"))
	}
	return extractTo(src, pages, out)
}

// ExtractPagesSeparate writes each selected page to its own PDF in dir.
func ExtractPagesSeparate(ctx context.Context, in Input, sel, dir, name string, prog Progress) ([]string, error) {
	src, err := readCtx(in.Path, in.Password)
	if err != nil {
		return nil, err
	}
	pages, err := ParsePages(sel, src.PageCount)
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, errors.New(i18n.L("pilih halaman yang mau diambil", "choose the pages to extract"))
	}
	var r []string
	for _, p := range pages {
		r = append(r, fmt.Sprint(p))
	}
	return Split(ctx, in, SplitOptions{Mode: "ranges", Ranges: strings.Join(r, ",")}, dir, name, prog)
}

// Rotate turns pages clockwise by deg (90, 180, 270); sel "" means every page.
func Rotate(in Input, deg int, sel, out string) error {
	src, err := readCtx(in.Path, in.Password)
	if err != nil {
		return err
	}
	pages := allPages(src.PageCount)
	if strings.TrimSpace(sel) != "" {
		if pages, err = ParsePages(sel, src.PageCount); err != nil {
			return err
		}
	}
	if err := pdfcpu.RotatePages(src, intSet(pages), deg); err != nil {
		return err
	}
	return writeCtx(src, out)
}

func allPages(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

func intSet(pages []int) types.IntSet {
	s := types.IntSet{}
	for _, p := range pages {
		s[p] = true
	}
	return s
}

// PageRef is one page of an organised document.
type PageRef struct {
	Src    int     `json:"src"`    // index into sources; -1 for a blank page
	Page   int     `json:"page"`   // 1-based page of the source
	Rotate int     `json:"rotate"` // extra clockwise rotation (0, 90, 180, 270)
	W      float64 `json:"w"`      // blank page size in points
	H      float64 `json:"h"`
}

// Organize builds a new document from pages of one or more sources (reordered, rotated,
// duplicated or blank).
func Organize(ctx context.Context, sources []Input, pages []PageRef, out, tmpDir string, prog Progress) error {
	if len(pages) == 0 {
		return errors.New(i18n.L("dokumen tidak punya halaman", "the document has no pages"))
	}
	// Merge every used source (plus blank pages) into one context and remember offsets.
	var dest *model.Context
	offset := map[int]int{}
	add := func(src *model.Context, key int) error {
		if dest == nil {
			dest = src
			dest.EnsureVersionForWriting()
			offset[key] = 0
			return nil
		}
		offset[key] = dest.PageCount
		return mergeInto(dest, src, fmt.Sprint(key))
	}
	used := map[int]bool{}
	for _, p := range pages {
		used[p.Src] = true
	}
	for i, s := range sources {
		if !used[i] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		src, err := openPlain(s, tmpDir)
		if err != nil {
			return err
		}
		if err := add(src, i); err != nil {
			return err
		}
	}
	prog(0.5)
	blankOf := map[int]int{} // position → page number in dest
	for i, p := range pages {
		if p.Src >= 0 {
			continue
		}
		tmp := filepath.Join(tmpDir, fmt.Sprintf("blank-%d.pdf", i))
		if err := blankPDF(tmp, p.W, p.H); err != nil {
			return err
		}
		bctx, err := readCtx(tmp, "")
		os.Remove(tmp)
		if err != nil {
			return err
		}
		key := -1000 - i
		if err := add(bctx, key); err != nil {
			return err
		}
		blankOf[i] = offset[key] + 1
	}
	order := make([]int, len(pages))
	for i, p := range pages {
		if p.Src < 0 {
			order[i] = blankOf[i]
			continue
		}
		base, ok := offset[p.Src]
		if !ok {
			return errors.New(i18n.L("sumber halaman tidak ditemukan", "page source not found"))
		}
		order[i] = base + p.Page
	}
	res, err := pdfcpu.ExtractPages(dest, order, false)
	if err != nil {
		return friendly(err)
	}
	if err := res.EnsurePageCount(); err != nil {
		return err
	}
	byRot := map[int][]int{}
	for i, p := range pages {
		r := ((p.Rotate % 360) + 360) % 360
		if r != 0 {
			byRot[r] = append(byRot[r], i+1)
		}
	}
	for r, list := range byRot {
		if err := pdfcpu.RotatePages(res, intSet(list), r); err != nil {
			return err
		}
	}
	prog(0.8)
	return writeCtx(res, out)
}
