package pdf

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
)

//go:embed scripts/ocr.ps1
var ocrScript []byte

// OCRLanguage is a recognition language installed in Windows.
type OCRLanguage struct {
	Tag  string `json:"tag"`
	Name string `json:"name"`
}

// OCROptions control text recognition.
type OCROptions struct {
	Lang     string `json:"lang"`     // BCP-47 tag, "" = the Windows profile languages
	Pages    string `json:"pages"`    // "" = every page
	SkipText bool   `json:"skipText"` // leave pages that already contain text alone
}

// writeScript saves an embedded PowerShell script into dir and returns its path.
func writeScript(dir, name string, body []byte) (string, error) {
	p := filepath.Join(dir, name)
	// A BOM makes Windows PowerShell 5.1 read the script as UTF-8.
	data := append([]byte{0xEF, 0xBB, 0xBF}, body...)
	return p, os.WriteFile(p, data, 0o644)
}

func powershell(ctx context.Context, script string, args ...string) (cmdArgs []string) {
	return append([]string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script}, args...)
}

// OCRLanguages lists the OCR languages Windows can recognise.
func OCRLanguages(ctx context.Context, tmpDir string) ([]OCRLanguage, error) {
	script, err := writeScript(tmpDir, "kmb-ocr.ps1", ocrScript)
	if err != nil {
		return nil, err
	}
	out, err := proc.Output(ctx, "powershell.exe", powershell(ctx, script, "-List")...)
	if err != nil {
		return nil, fmt.Errorf(i18n.L("OCR Windows tidak tersedia: %w", "Windows OCR isn't available: %w"), err)
	}
	var list []OCRLanguage
	for _, l := range strings.Split(out, "\n") {
		parts := strings.SplitN(strings.TrimSpace(l), "\t", 2)
		if len(parts) == 2 && parts[0] != "" {
			list = append(list, OCRLanguage{Tag: parts[0], Name: parts[1]})
		}
	}
	return list, nil
}

type ocrResult struct {
	Lines []struct {
		W [][]any `json:"w"`
	} `json:"lines"`
	Error string `json:"error"`
}

// ocrWorker keeps one PowerShell process that recognises images one by one.
type ocrWorker struct {
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr *strings.Builder
	wait   func() error
}

func startOCR(ctx context.Context, tmpDir, lang string) (*ocrWorker, error) {
	script, err := writeScript(tmpDir, "kmb-ocr.ps1", ocrScript)
	if err != nil {
		return nil, err
	}
	args := []string{}
	if lang != "" {
		args = append(args, "-Lang", lang)
	}
	cmd := proc.Command(ctx, "powershell.exe", powershell(ctx, script, args...)...)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	w := &ocrWorker{stdin: in, stdout: bufio.NewReaderSize(out, 1<<20), stderr: &stderr, wait: cmd.Wait}
	line, err := w.stdout.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "ready" {
		_ = in.Close()
		_ = w.wait()
		if strings.Contains(stderr.String(), "no-engine") {
			return nil, errors.New(i18n.L("bahasa OCR ini belum terpasang di Windows (Pengaturan › Waktu & Bahasa › Bahasa)", "this OCR language isn't installed in Windows (Settings › Time & Language › Language)"))
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf(i18n.L("OCR Windows tidak bisa dijalankan: %s", "Windows OCR can't start: %s"), proc.LastLines(stderr.String()+line, 4))
	}
	return w, nil
}

func (w *ocrWorker) recognize(path string) (*ocrResult, error) {
	if _, err := io.WriteString(w.stdin, path+"\n"); err != nil {
		return nil, err
	}
	line, err := w.stdout.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("ocr: %v %s", err, proc.LastLines(w.stderr.String(), 3))
	}
	var r ocrResult
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		return nil, err
	}
	if r.Error != "" {
		return nil, errors.New(r.Error)
	}
	return &r, nil
}

func (w *ocrWorker) close() {
	_ = w.stdin.Close()
	_ = w.wait()
}

func hasText(s string) bool {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
		}
	}
	return n >= 20
}

// OCR adds an invisible, searchable text layer over scanned pages. It returns how many
// pages were recognised.
func OCR(ctx context.Context, in Input, o OCROptions, out, tmpDir string, prog Progress) (int, error) {
	doc, err := Open(ctx, in.Path, in.Password)
	if err != nil {
		return 0, friendly(err)
	}
	defer doc.Close()
	pages, err := selected(o.Pages, doc.Pages())
	if err != nil {
		return 0, err
	}
	worker, err := startOCR(ctx, tmpDir, o.Lang)
	if err != nil {
		return 0, err
	}
	defer worker.close()
	const dpi = 300
	items := map[int][]Item{}
	done := 0
	img := filepath.Join(tmpDir, "ocr-"+randomHex(6)+".png")
	defer os.Remove(img)
	for i, p := range pages {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if o.SkipText {
			if t, err := doc.PageText(p - 1); err == nil && hasText(t) {
				prog(float64(i+1) / float64(len(pages)) * 0.9)
				continue
			}
		}
		pic, err := doc.RenderDPI(p-1, dpi)
		if err != nil {
			return 0, err
		}
		f, err := os.Create(img)
		if err != nil {
			return 0, err
		}
		err = (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(f, flattenWhite(pic))
		f.Close()
		if err != nil {
			return 0, err
		}
		res, err := worker.recognize(img)
		if err != nil {
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			return 0, fmt.Errorf(i18n.L("halaman %d: %w", "page %d: %w"), p, err)
		}
		s := 72.0 / dpi
		for _, line := range res.Lines {
			for _, w := range line.W {
				if len(w) < 5 {
					continue
				}
				text, _ := w[0].(string)
				x, _ := w[1].(float64)
				y, _ := w[2].(float64)
				wd, _ := w[3].(float64)
				ht, _ := w[4].(float64)
				if strings.TrimSpace(text) == "" || wd <= 0 || ht <= 0 {
					continue
				}
				size := ht * s
				items[p] = append(items[p], Item{Kind: "text", Text: text, X: x * s, Y: (y + ht*0.8) * s, Size: size, Hidden: true, FitW: wd * s})
			}
		}
		done++
		prog(float64(i+1) / float64(len(pages)) * 0.9)
	}
	if done == 0 {
		return 0, errors.New(i18n.L("semua halaman sudah berisi teks — tidak ada yang perlu di-OCR", "every page already has text — nothing to recognise"))
	}
	if err := DrawItems(in, items, out); err != nil {
		return 0, err
	}
	return done, nil
}
