package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Lyrics from LRCLIB (lrclib.net), a free, open lyrics database without a request limit.

// Lyrics of a song: time-synced (LRC) and/or plain text.
type Lyrics struct {
	Synced string `json:"syncedLyrics"`
	Plain  string `json:"plainLyrics"`
	// Instrumental songs have no lyrics.
	Instrumental bool    `json:"instrumental"`
	Duration     float64 `json:"duration"`
}

var lyricsClient = &http.Client{Timeout: 20 * time.Second}

const lrclib = "https://lrclib.net/api/"

func lrclibGet(ctx context.Context, path string, q url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lrclib+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "KuyMediaBox (https://github.com/iqbalfaf/KuyMediaBox)")
	resp, err := lyricsClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errNoLyrics
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("lrclib: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

var errNoLyrics = errors.New("no lyrics")

// FetchLyrics looks a song up: first exactly (artist, title, album, length), then by search,
// taking the closest length within a few seconds.
func FetchLyrics(ctx context.Context, artist, title, album string, duration float64) (Lyrics, error) {
	artist, title = strings.TrimSpace(artist), strings.TrimSpace(title)
	if artist == "" || title == "" {
		return Lyrics{}, errNoLyrics
	}
	q := url.Values{"artist_name": {artist}, "track_name": {title}}
	if album != "" {
		q.Set("album_name", album)
	}
	if duration > 0 {
		q.Set("duration", fmt.Sprint(math.Round(duration)))
	}
	var l Lyrics
	if err := lrclibGet(ctx, "get", q, &l); err == nil && (l.Synced != "" || l.Plain != "" || l.Instrumental) {
		return l, nil
	}
	var list []Lyrics
	if err := lrclibGet(ctx, "search", url.Values{"artist_name": {artist}, "track_name": {title}}, &list); err != nil {
		return Lyrics{}, err
	}
	best, bestDiff := -1, math.MaxFloat64
	for i, c := range list {
		if c.Synced == "" && c.Plain == "" {
			continue
		}
		diff := 0.0
		if duration > 0 && c.Duration > 0 {
			diff = math.Abs(c.Duration - duration)
			if diff > 4 {
				continue
			}
		}
		// Prefer synced lyrics when lengths are equally close.
		if c.Synced == "" {
			diff += 0.5
		}
		if diff < bestDiff {
			best, bestDiff = i, diff
		}
	}
	if best < 0 {
		return Lyrics{}, errNoLyrics
	}
	return list[best], nil
}

// WriteLRC saves the lyrics next to an audio file as "<name>.lrc" (synced, with the song
// details on top) or, without timing, as "<name>.txt". It returns the file written.
func WriteLRC(audio string, l Lyrics, artist, title, album string) (string, error) {
	base := strings.TrimSuffix(audio, filepath.Ext(audio))
	switch {
	case l.Synced != "":
		var b strings.Builder
		fmt.Fprintf(&b, "[ar:%s]\n[ti:%s]\n", artist, title)
		if album != "" {
			fmt.Fprintf(&b, "[al:%s]\n", album)
		}
		b.WriteString("[by:LRCLIB]\n")
		b.WriteString(strings.TrimSpace(l.Synced))
		b.WriteString("\n")
		return base + ".lrc", os.WriteFile(base+".lrc", []byte(b.String()), 0o644)
	case l.Plain != "":
		return base + ".txt", os.WriteFile(base+".txt", []byte(strings.TrimSpace(l.Plain)+"\n"), 0o644)
	}
	return "", errNoLyrics
}
