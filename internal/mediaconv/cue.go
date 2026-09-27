package mediaconv

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	"kuymediabox/internal/i18n"
)

// Cue is a CD cue sheet: one audio file split into tracks.
type Cue struct {
	Title     string     `json:"title"`
	Performer string     `json:"performer"`
	Date      string     `json:"date"`
	Genre     string     `json:"genre"`
	File      string     `json:"file"` // audio file named in the sheet
	Tracks    []CueTrack `json:"tracks"`
}

// CueTrack is one song of a cue sheet; Start is in seconds.
type CueTrack struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	Performer string  `json:"performer"`
	Start     float64 `json:"start"`
}

// ParseCue reads a cue sheet (UTF-8, or Windows-1252 as most old rips use).
func ParseCue(path string) (*Cue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(data) {
		if dec, err := charmap.Windows1252.NewDecoder().Bytes(data); err == nil {
			data = dec
		}
	}
	cue := &Cue{}
	var cur *CueTrack
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		key, rest, _ := strings.Cut(line, " ")
		rest = strings.TrimSpace(rest)
		switch strings.ToUpper(key) {
		case "REM":
			k, v, _ := strings.Cut(rest, " ")
			switch strings.ToUpper(k) {
			case "DATE":
				cue.Date = unquote(v)
			case "GENRE":
				cue.Genre = unquote(v)
			}
		case "FILE":
			if cue.File == "" {
				// FILE "name.flac" WAVE
				if i := strings.LastIndex(rest, " "); i > 0 && !strings.HasSuffix(rest, `"`) {
					rest = rest[:i]
				}
				cue.File = unquote(rest)
			}
		case "TITLE":
			if cur != nil {
				cur.Title = unquote(rest)
			} else {
				cue.Title = unquote(rest)
			}
		case "PERFORMER":
			if cur != nil {
				cur.Performer = unquote(rest)
			} else {
				cue.Performer = unquote(rest)
			}
		case "TRACK":
			f := strings.Fields(rest)
			if len(f) >= 2 && !strings.EqualFold(f[1], "AUDIO") {
				cur = nil // data track
				continue
			}
			n, _ := strconv.Atoi(firstField(rest))
			cue.Tracks = append(cue.Tracks, CueTrack{Number: n, Start: -1})
			cur = &cue.Tracks[len(cue.Tracks)-1]
		case "INDEX":
			f := strings.Fields(rest)
			if cur == nil || len(f) < 2 {
				continue
			}
			t, ok := cueTime(f[1])
			if !ok {
				continue
			}
			// INDEX 01 is where the song starts; INDEX 00 (pregap) only if 01 is missing.
			if f[0] == "01" || cur.Start < 0 {
				cur.Start = t
			}
		}
	}
	tracks := cue.Tracks[:0]
	for _, t := range cue.Tracks {
		if t.Start >= 0 {
			tracks = append(tracks, t)
		}
	}
	cue.Tracks = tracks
	if len(cue.Tracks) == 0 {
		return nil, errors.New(i18n.L("file CUE tidak berisi daftar lagu", "the CUE file has no track list"))
	}
	return cue, nil
}

func firstField(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return ""
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return strings.TrimSpace(s)
}

// cueTime reads mm:ss:ff (75 frames per second).
func cueTime(s string) (float64, bool) {
	p := strings.Split(s, ":")
	if len(p) != 3 {
		return 0, false
	}
	m, e1 := strconv.Atoi(p[0])
	sec, e2 := strconv.Atoi(p[1])
	fr, e3 := strconv.Atoi(p[2])
	if e1 != nil || e2 != nil || e3 != nil || m < 0 || sec < 0 || fr < 0 {
		return 0, false
	}
	return float64(m*60+sec) + float64(fr)/75, true
}

// FindCue returns the cue sheet belonging to an audio file: "album.cue" / "album.flac.cue"
// next to it, or any cue sheet in the folder whose FILE line names it.
func FindCue(audio string) string {
	dir := filepath.Dir(audio)
	base := strings.TrimSuffix(filepath.Base(audio), filepath.Ext(audio))
	for _, name := range []string{base + ".cue", filepath.Base(audio) + ".cue"} {
		if p := filepath.Join(dir, name); fileExists(p) {
			return p
		}
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.cue"))
	for _, m := range matches {
		if c, err := ParseCue(m); err == nil && strings.EqualFold(filepath.Base(c.File), filepath.Base(audio)) {
			return m
		}
	}
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// TrackRange returns the start and end of track i ("" end = to the end of the file).
func (c *Cue) TrackRange(i int) (string, string) {
	start := strconv.FormatFloat(c.Tracks[i].Start, 'f', 3, 64)
	end := ""
	if i+1 < len(c.Tracks) {
		end = strconv.FormatFloat(c.Tracks[i+1].Start, 'f', 3, 64)
	}
	return start, end
}

// TrackTags are the song details of track i for the tag writer.
func (c *Cue) TrackTags(i int) *Tags {
	t := c.Tracks[i]
	artist := t.Performer
	if artist == "" {
		artist = c.Performer
	}
	year := c.Date
	if len(year) > 4 {
		year = year[:4]
	}
	return &Tags{
		Title:       t.Title,
		Artist:      artist,
		Album:       c.Title,
		AlbumArtist: c.Performer,
		Year:        year,
		Genre:       c.Genre,
		Track:       strconv.Itoa(t.Number) + "/" + strconv.Itoa(len(c.Tracks)),
	}
}
