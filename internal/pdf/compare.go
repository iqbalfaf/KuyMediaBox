package pdf

import (
	"context"
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
)

// Change is one difference between two documents.
type Change struct {
	Kind  string `json:"kind"`  // added | removed
	Text  string `json:"text"`  // the words that changed
	Page  int    `json:"page"`  // 1-based page in the document that has the words
	Boxes []Rect `json:"boxes"` // where they are (normalized, page numbers 1-based)
	words int
}

// CompareResult lists the differences between document A (old) and B (new).
type CompareResult struct {
	PagesA  int      `json:"pagesA"`
	PagesB  int      `json:"pagesB"`
	Changes []Change `json:"changes"`
	Added   int      `json:"added"`   // words
	Removed int      `json:"removed"` // words
	Same    bool     `json:"same"`
}

type token struct {
	text  string
	key   string
	boxes []Rect
}

func docTokens(ctx context.Context, path string) ([]token, int, error) {
	var toks []token
	pages := 0
	err := WithViewerDoc(ctx, path, func(d *Doc) error {
		pages = d.Pages()
		for p := 0; p < d.Pages(); p++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			chars, err := d.Chars(p)
			if err != nil {
				return err
			}
			joinNext := false
			for _, l := range Lines(Words(chars)) {
				for i, w := range l.Words {
					box := Rect{Page: p + 1, X: w.X0, Y: w.Y0, W: w.X1 - w.X0, H: w.Y1 - w.Y0}
					if joinNext && i == 0 && len(toks) > 0 {
						// A word hyphenated at the end of the previous line continues here.
						t := &toks[len(toks)-1]
						t.text += w.Text
						t.key = strings.ToLower(t.text)
						t.boxes = append(t.boxes, box)
					} else {
						toks = append(toks, token{text: w.Text, key: strings.ToLower(w.Text), boxes: []Rect{box}})
					}
					joinNext = false
				}
				if n := len(l.Words); n > 0 && strings.HasSuffix(l.Words[n-1].Text, "-") && len(l.Words[n-1].Text) > 1 {
					joinNext = true
				}
			}
		}
		return nil
	})
	return toks, pages, err
}

// diffOps returns, for each token of a and b, whether it is kept (true) or changed.
func diffOps(a, b []token) (keepA, keepB []bool) {
	keepA, keepB = make([]bool, len(a)), make([]bool, len(b))
	// Common prefix and suffix first; they are the bulk in similar documents.
	s := 0
	for s < len(a) && s < len(b) && a[s].key == b[s].key {
		keepA[s], keepB[s] = true, true
		s++
	}
	ea, eb := len(a), len(b)
	for ea > s && eb > s && a[ea-1].key == b[eb-1].key {
		ea--
		eb--
		keepA[ea], keepB[eb] = true, true
	}
	x, y := a[s:ea], b[s:eb]
	n, m := len(x), len(y)
	if n == 0 || m == 0 {
		return
	}
	// Myers' O(ND) algorithm with a snapshot of V per step to walk back the path.
	maxD := min(n+m, 2500) // beyond this the documents are simply "very different"
	off := maxD + 1
	v := make([]int, 2*maxD+3)
	var trace [][]int
	found := -1
	for d := 0; d <= maxD && found < 0; d++ {
		// Keep only the diagonals reachable at this step: k in [-d-1, d+1].
		trace = append(trace, append([]int(nil), v[off-d-1:off+d+2]...))
		for k := -d; k <= d; k += 2 {
			var i int
			if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
				i = v[off+k+1]
			} else {
				i = v[off+k-1] + 1
			}
			j := i - k
			for i < n && j < m && x[i].key == y[j].key {
				i++
				j++
			}
			v[off+k] = i
			if i >= n && j >= m {
				found = d
				break
			}
		}
	}
	if found < 0 {
		return // too different: everything in the middle counts as changed
	}
	i, j := n, m
	for d := found; d > 0; d-- {
		snap := trace[d]
		pv := func(k int) int { return snap[k+d+1] }
		k := i - j
		var pk int
		if k == -d || (k != d && pv(k-1) < pv(k+1)) {
			pk = k + 1
		} else {
			pk = k - 1
		}
		pi := pv(pk)
		pj := pi - pk
		for i > pi && j > pj {
			i--
			j--
			keepA[s+i], keepB[s+j] = true, true
		}
		i, j = pi, pj
	}
	for i > 0 && j > 0 {
		i--
		j--
		keepA[s+i], keepB[s+j] = true, true
	}
	return
}

// groupChanges joins runs of changed tokens (same page) into changes.
func groupChanges(toks []token, keep []bool, kind string) []Change {
	var out []Change
	var cur *Change
	for i, t := range toks {
		if keep[i] {
			if cur != nil {
				out = append(out, *cur)
				cur = nil
			}
			continue
		}
		if cur != nil && cur.Page != t.boxes[0].Page {
			out = append(out, *cur)
			cur = nil
		}
		if cur == nil {
			cur = &Change{Kind: kind, Page: t.boxes[0].Page}
		} else {
			cur.Text += " "
		}
		cur.Text += t.text
		cur.Boxes = append(cur.Boxes, t.boxes...)
		cur.words++
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

// Compare finds the words removed from a and added in b.
func Compare(ctx context.Context, a, b string) (CompareResult, error) {
	var res CompareResult
	ta, pa, err := docTokens(ctx, a)
	if err != nil {
		return res, err
	}
	tb, pb, err := docTokens(ctx, b)
	if err != nil {
		return res, err
	}
	if len(ta) == 0 && len(tb) == 0 {
		return res, errors.New(i18n.L("kedua PDF tidak berisi teks (hasil scan?) — jalankan OCR dulu", "neither PDF contains text (scans?) — run OCR first"))
	}
	res.PagesA, res.PagesB = pa, pb
	keepA, keepB := diffOps(ta, tb)
	removed := groupChanges(ta, keepA, "removed")
	added := groupChanges(tb, keepB, "added")
	for _, c := range removed {
		res.Removed += c.words
	}
	for _, c := range added {
		res.Added += c.words
	}
	res.Changes = append(removed, added...)
	res.Same = len(res.Changes) == 0
	return res, nil
}

//go:embed scripts/scan.ps1
var scanScript []byte

// Scan shows the Windows scanner dialog and returns the scanned JPEG ("" when cancelled).
func Scan(ctx context.Context, tmpDir string) (string, error) {
	script, err := writeScript(tmpDir, "kmb-scan.ps1", scanScript)
	if err != nil {
		return "", err
	}
	out := filepath.Join(tmpDir, "scan-"+randomHex(6)+".jpg")
	res, err := proc.Output(ctx, "powershell.exe", powershell(ctx, script, "-Out", out)...)
	if err != nil {
		return "", errors.New(i18n.L("scanner tidak bisa dipakai: ", "the scanner can't be used: ") + proc.LastLines(err.Error(), 2))
	}
	switch strings.TrimSpace(res) {
	case "ok":
		if _, err := os.Stat(out); err != nil {
			return "", errors.New(i18n.L("hasil scan tidak ditemukan", "the scan was not saved"))
		}
		return out, nil
	case "no-device":
		return "", errors.New(i18n.L("tidak ada scanner yang terhubung", "no scanner is connected"))
	}
	return "", nil
}
