// Package fonts finds a font that can draw a given text (Windows fonts first, the bundled Go
// fonts as a Latin fallback) and cuts TrueType subsets for embedding in PDFs.
package fonts

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

// Face is a parsed font plus the standalone TrueType data it came from.
type Face struct {
	Name string     // file name (or "Go") for messages
	Font *sfnt.Font // parsed font
	Data []byte     // a single TrueType font (collections are unpacked)
}

// Fallback lists, in order of preference. Segoe UI and Arial cover Latin, Greek, Cyrillic,
// Hebrew and Arabic; the rest add Indic, Thai, CJK and symbols.
var (
	regularFiles = []string{"segoeui.ttf", "arial.ttf", "Nirmala.ttf", "LeelawUI.ttf", "malgun.ttf", "msyh.ttc", "YuGothM.ttc", "msjh.ttc", "ebrima.ttf", "gadugi.ttf", "seguisym.ttf"}
	boldFiles    = []string{"segoeuib.ttf", "arialbd.ttf", "NirmalaB.ttf", "LeelaUIb.ttf", "malgunbd.ttf", "msyhbd.ttc", "YuGothB.ttc", "msjhbd.ttc", "ebrimabd.ttf", "gadugib.ttf", "seguisym.ttf"}
)

var (
	mu    sync.Mutex
	cache = map[string]*Face{} // path → face (nil when unusable)
)

func fontsDir() string {
	win := os.Getenv("WINDIR")
	if win == "" {
		win = `C:\Windows`
	}
	return filepath.Join(win, "Fonts")
}

// load parses a font file (the first font of a collection), cached.
func load(path string) *Face {
	mu.Lock()
	defer mu.Unlock()
	if f, ok := cache[path]; ok {
		return f
	}
	cache[path] = nil
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	data := raw
	if strings.EqualFold(filepath.Ext(path), ".ttc") {
		if data, err = firstOfCollection(raw); err != nil {
			return nil
		}
	}
	f, err := sfnt.Parse(data)
	if err != nil || !isTrueType(data) {
		return nil
	}
	face := &Face{Name: filepath.Base(path), Font: f, Data: data}
	cache[path] = face
	return face
}

var goFaces struct {
	once          sync.Once
	regular, bold *Face
}

// Go returns the bundled Go font (Latin only; always available).
func Go(bold bool) *Face {
	goFaces.once.Do(func() {
		r, _ := sfnt.Parse(goregular.TTF)
		b, _ := sfnt.Parse(gobold.TTF)
		goFaces.regular = &Face{Name: "Go", Font: r, Data: goregular.TTF}
		goFaces.bold = &Face{Name: "Go Bold", Font: b, Data: gobold.TTF}
	})
	if bold {
		return goFaces.bold
	}
	return goFaces.regular
}

// Covers reports whether the face has a glyph for every visible rune of text.
func (f *Face) Covers(text string) bool {
	var buf sfnt.Buffer
	for _, r := range text {
		if r < 32 || r == '\u00a0' || r == '\u200b' {
			continue
		}
		gid, err := f.Font.GlyphIndex(&buf, r)
		if err != nil || gid == 0 {
			return false
		}
	}
	return true
}

// For returns the first font that covers all of text; when none does, the one covering the
// most characters (missing glyphs are drawn as the font's "notdef" box). A bold request
// falls back to regular fonts before giving up.
func For(text string, bold bool) *Face {
	lists := [][]string{regularFiles}
	if bold {
		lists = [][]string{boldFiles, regularFiles}
	}
	var best *Face
	bestMissing := -1
	for _, files := range lists {
		for _, name := range files {
			f := load(filepath.Join(fontsDir(), name))
			if f == nil {
				continue
			}
			m := f.missing(text)
			if m == 0 {
				return f
			}
			if bestMissing < 0 || m < bestMissing {
				best, bestMissing = f, m
			}
		}
	}
	if g := Go(bold); best == nil || g.missing(text) < bestMissing {
		return g
	}
	return best
}

func (f *Face) missing(text string) int {
	var buf sfnt.Buffer
	n := 0
	for _, r := range text {
		if r < 32 {
			continue
		}
		if gid, err := f.Font.GlyphIndex(&buf, r); err != nil || gid == 0 {
			n++
		}
	}
	return n
}

func isTrueType(data []byte) bool {
	t, err := tableDir(data)
	if err != nil {
		return false
	}
	_, hasGlyf := t["glyf"]
	_, hasLoca := t["loca"]
	return hasGlyf && hasLoca
}

var errBadFont = errors.New("unsupported font data")
