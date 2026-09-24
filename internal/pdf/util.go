package pdf

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"kuymediabox/internal/i18n"
)

func init() {
	// pdfcpu would otherwise create a config folder in %APPDATA%.
	api.DisableConfigDir()
}

// conf returns a relaxed pdfcpu configuration for a (possibly protected) input.
func conf(password string) *model.Configuration {
	c := model.NewDefaultConfiguration()
	c.ValidationMode = model.ValidationRelaxed
	c.UserPW = password
	c.OwnerPW = password
	return c
}

// readCtx loads a PDF for editing. Output written from it is never encrypted.
func readCtx(path, password string) (*model.Context, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ctx, err := api.ReadAndValidate(f, conf(password))
	if err != nil {
		return nil, friendly(err)
	}
	if err := ctx.EnsurePageCount(); err != nil {
		return nil, friendly(err)
	}
	return ctx, nil
}

// writeCtx writes ctx to path without encryption.
func writeCtx(ctx *model.Context, path string) error {
	ctx.Cmd = model.DECRYPT // drop any encryption of the source
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	err = api.WriteContext(ctx, w)
	if err == nil {
		err = w.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(path)
		return friendly(err)
	}
	return nil
}

// friendly turns common pdfcpu errors into readable messages.
func friendly(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrPassword) {
		return errLocked()
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "password"), strings.Contains(msg, "encrypt"), strings.Contains(msg, "permission"):
		return errLocked()
	case strings.Contains(msg, "no pdf") || strings.Contains(msg, "header") || strings.Contains(msg, "xref") || strings.Contains(msg, "eof"):
		return fmt.Errorf(i18n.L("PDF rusak atau tidak valid — coba alat Perbaiki PDF (%v)", "the PDF is damaged or invalid — try the Repair PDF tool (%v)"), err)
	}
	return err
}

func errLocked() error {
	return errors.New(i18n.L("PDF dikunci password — masukkan password yang benar atau buka dulu dengan Buka Kunci PDF", "the PDF is password protected — enter the right password or unlock it first"))
}

// ParsePages turns "1-3, 5, 8-" into sorted unique 1-based page numbers within 1..max.
// An empty string selects nothing.
func ParsePages(s string, max int) ([]int, error) {
	set := map[int]bool{}
	s = strings.NewReplacer("–", "-", "—", "-", ";", ",").Replace(s)
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }) {
		a, b, err := parseSpan(part, max)
		if err != nil {
			return nil, err
		}
		for i := a; i <= b; i++ {
			set[i] = true
		}
	}
	out := make([]int, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Ints(out)
	return out, nil
}

// ParseRanges turns "1-3, 4-6, 9" into ordered spans (kept in the given order).
func ParseRanges(s string, max int) ([][2]int, error) {
	var out [][2]int
	s = strings.NewReplacer("–", "-", "—", "-", ";", ",").Replace(s)
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' }) {
		part = strings.ReplaceAll(strings.TrimSpace(part), " ", "")
		if part == "" {
			continue
		}
		a, b, err := parseSpan(part, max)
		if err != nil {
			return nil, err
		}
		out = append(out, [2]int{a, b})
	}
	return out, nil
}

func parseSpan(part string, max int) (int, int, error) {
	bad := fmt.Errorf(i18n.L("rentang halaman tidak valid: %q", "invalid page range: %q"), part)
	num := func(t string, def int) (int, error) {
		t = strings.TrimSpace(t)
		if t == "" {
			return def, nil
		}
		if strings.EqualFold(t, "akhir") || strings.EqualFold(t, "end") || strings.EqualFold(t, "last") {
			return max, nil
		}
		return strconv.Atoi(t)
	}
	var a, b int
	var err error
	if i := strings.Index(part, "-"); i >= 0 {
		if a, err = num(part[:i], 1); err != nil {
			return 0, 0, bad
		}
		if b, err = num(part[i+1:], max); err != nil {
			return 0, 0, bad
		}
	} else {
		if a, err = num(part, 0); err != nil || a == 0 {
			return 0, 0, bad
		}
		b = a
	}
	if a > b {
		a, b = b, a
	}
	if a < 1 || b > max {
		return 0, 0, fmt.Errorf(i18n.L("halaman %s di luar dokumen (1–%d)", "page %s is outside the document (1–%d)"), part, max)
	}
	return a, b, nil
}

func pagesToStrings(pages []int) []string {
	out := make([]string, len(pages))
	for i, p := range pages {
		out[i] = strconv.Itoa(p)
	}
	return out
}

func flattenWhite(img image.Image) image.Image {
	switch img.(type) {
	case *image.YCbCr, *image.Gray:
		return img
	}
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Over)
	return dst
}

// Progress reports 0..1.
type Progress func(p float64)

func noProgress(float64) {}
