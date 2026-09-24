package pdf

import (
	"context"
	"sort"
	"strings"
	"unicode"
)

// Word is a run of non-space characters with its box (normalized display coordinates).
type Word struct {
	Text           string
	X0, Y0, X1, Y1 float64
	Size           float64
	Bold           bool
}

// Line is a row of words.
type Line struct {
	Words          []Word
	X0, Y0, X1, Y1 float64
	Size           float64
}

func (l Line) Text() string {
	parts := make([]string, len(l.Words))
	for i, w := range l.Words {
		parts[i] = w.Text
	}
	return strings.Join(parts, " ")
}

func isBreak(s string) bool {
	return s == "" || strings.TrimFunc(s, unicode.IsSpace) == ""
}

// Words groups characters into words, splitting on spaces, line breaks and big gaps.
func Words(chars []Char) []Word {
	var out []Word
	var cur *Word
	flush := func() {
		if cur != nil && strings.TrimSpace(cur.Text) != "" {
			out = append(out, *cur)
		}
		cur = nil
	}
	for _, c := range chars {
		if isBreak(c.Text) || c.X1 <= c.X0 && c.Y1 <= c.Y0 {
			flush()
			continue
		}
		if cur != nil {
			h := max(cur.Y1-cur.Y0, c.Y1-c.Y0)
			sameLine := c.Y0 < cur.Y1-h*0.3 && c.Y1 > cur.Y0+h*0.3
			gap := c.X0 - cur.X1
			if !sameLine || gap > h*0.6 || gap < -h*1.5 {
				flush()
			}
		}
		if cur == nil {
			cur = &Word{X0: c.X0, Y0: c.Y0, X1: c.X1, Y1: c.Y1, Size: c.Size, Bold: c.Bold}
		}
		cur.Text += c.Text
		cur.X0, cur.Y0 = min(cur.X0, c.X0), min(cur.Y0, c.Y0)
		cur.X1, cur.Y1 = max(cur.X1, c.X1), max(cur.Y1, c.Y1)
		cur.Size = max(cur.Size, c.Size)
	}
	flush()
	return out
}

// Lines groups words into rows (top to bottom, left to right).
func Lines(words []Word) []Line {
	ws := append([]Word(nil), words...)
	sort.SliceStable(ws, func(i, j int) bool { return (ws[i].Y0+ws[i].Y1)/2 < (ws[j].Y0+ws[j].Y1)/2 })
	var lines []Line
	for _, w := range ws {
		cy := (w.Y0 + w.Y1) / 2
		placed := false
		for i := len(lines) - 1; i >= 0 && i >= len(lines)-3; i-- {
			l := &lines[i]
			h := l.Y1 - l.Y0
			if cy > l.Y0+h*0.1 && cy < l.Y1-h*0.1 {
				l.Words = append(l.Words, w)
				l.X0, l.Y0, l.X1, l.Y1 = min(l.X0, w.X0), min(l.Y0, w.Y0), max(l.X1, w.X1), max(l.Y1, w.Y1)
				l.Size = max(l.Size, w.Size)
				placed = true
				break
			}
		}
		if !placed {
			lines = append(lines, Line{Words: []Word{w}, X0: w.X0, Y0: w.Y0, X1: w.X1, Y1: w.Y1, Size: w.Size})
		}
	}
	for i := range lines {
		sort.SliceStable(lines[i].Words, func(a, b int) bool { return lines[i].Words[a].X0 < lines[i].Words[b].X0 })
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Y0 < lines[j].Y0 })
	return lines
}

// FindText returns the boxes of every occurrence of query (case-insensitive unless
// matchCase), one rect per line segment.
func FindText(ctx context.Context, path, query string, matchCase bool) ([]Rect, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	fold := func(s string) string {
		if matchCase {
			return s
		}
		return strings.ToLower(s)
	}
	q := []rune(fold(query))
	var out []Rect
	err := WithViewerDoc(ctx, path, func(d *Doc) error {
		for p := 0; p < d.Pages(); p++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			chars, err := d.Chars(p)
			if err != nil {
				return err
			}
			// One rune per char entry, so indexes line up with boxes.
			runes := make([]rune, len(chars))
			for i, c := range chars {
				r := []rune(fold(c.Text))
				if len(r) == 0 || unicode.IsSpace(r[0]) {
					runes[i] = ' '
				} else {
					runes[i] = r[0]
				}
			}
			for i := 0; i+len(q) <= len(runes); i++ {
				match := true
				for k := range q {
					a, b := runes[i+k], q[k]
					if unicode.IsSpace(b) {
						if !unicode.IsSpace(a) {
							match = false
							break
						}
						continue
					}
					if a != b {
						match = false
						break
					}
				}
				if !match {
					continue
				}
				out = append(out, boxesFor(chars[i:i+len(q)], p+1)...)
				i += len(q) - 1
			}
		}
		return nil
	})
	return out, err
}

// boxesFor merges character boxes into one rect per line.
func boxesFor(chars []Char, page int) []Rect {
	var out []Rect
	var cur *Rect
	for _, c := range chars {
		if c.X1 <= c.X0 || c.Y1 <= c.Y0 {
			continue
		}
		if cur != nil {
			cy := (c.Y0 + c.Y1) / 2
			if cy < cur.Y || cy > cur.Y+cur.H {
				out = append(out, *cur)
				cur = nil
			}
		}
		if cur == nil {
			cur = &Rect{Page: page, X: c.X0, Y: c.Y0, W: c.X1 - c.X0, H: c.Y1 - c.Y0}
			continue
		}
		x1, y1 := max(cur.X+cur.W, c.X1), max(cur.Y+cur.H, c.Y1)
		cur.X, cur.Y = min(cur.X, c.X0), min(cur.Y, c.Y0)
		cur.W, cur.H = x1-cur.X, y1-cur.Y
	}
	if cur != nil {
		out = append(out, *cur)
	}
	// Pad a little so glyph edges are covered completely.
	for i := range out {
		padX, padY := out[i].H*0.12, out[i].H*0.15
		out[i].X, out[i].Y = out[i].X-padX, out[i].Y-padY
		out[i].W, out[i].H = out[i].W+2*padX, out[i].H+2*padY
	}
	return out
}
