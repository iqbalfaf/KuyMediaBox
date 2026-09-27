package mediaconv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCue(t *testing.T) {
	dir := t.TempDir()
	// Windows-1252 "é" (0xE9), a pregap on track 2 and a data track at the end.
	sheet := "REM GENRE Pop\r\nREM DATE 1999\r\nPERFORMER \"Band\"\r\nTITLE \"Album Caf\xe9\"\r\nFILE \"Band - Album.flac\" WAVE\r\n" +
		"  TRACK 01 AUDIO\r\n    TITLE \"One\"\r\n    INDEX 01 00:00:00\r\n" +
		"  TRACK 02 AUDIO\r\n    TITLE \"Two\"\r\n    PERFORMER \"Guest\"\r\n    INDEX 00 03:10:00\r\n    INDEX 01 03:12:37\r\n" +
		"  TRACK 03 MODE1/2352\r\n    INDEX 01 09:00:00\r\n"
	cuePath := filepath.Join(dir, "whatever.cue")
	if err := os.WriteFile(cuePath, []byte(sheet), 0o644); err != nil {
		t.Fatal(err)
	}
	audio := filepath.Join(dir, "Band - Album.flac")
	_ = os.WriteFile(audio, nil, 0o644)
	if got := FindCue(audio); got != cuePath {
		t.Fatalf("FindCue = %q", got)
	}
	c, err := ParseCue(cuePath)
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "Album Café" || c.Performer != "Band" || c.Genre != "Pop" || c.File != "Band - Album.flac" || len(c.Tracks) != 2 {
		t.Fatalf("cue %+v", c)
	}
	if s := c.Tracks[1].Start; s < 192.49 || s > 192.5 {
		t.Errorf("track 2 starts at %v, want 192.493 (INDEX 01)", s)
	}
	start, end := c.TrackRange(0)
	if start != "0.000" || !strings.HasPrefix(end, "192.49") {
		t.Errorf("range %s-%s", start, end)
	}
	if _, end := c.TrackRange(1); end != "" {
		t.Errorf("last track must run to the end, got %q", end)
	}
	tags := c.TrackTags(1)
	if tags.Artist != "Guest" || tags.AlbumArtist != "Band" || tags.Track != "2/2" || tags.Year != "1999" {
		t.Errorf("tags %+v", tags)
	}
}
