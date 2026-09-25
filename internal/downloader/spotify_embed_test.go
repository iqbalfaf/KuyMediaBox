package downloader

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestParseEmbedTrack(t *testing.T) {
	page, err := os.ReadFile("testdata/spotify-embed-track.html")
	if err != nil {
		t.Fatal(err)
	}
	e, err := parseEmbed(page)
	if err != nil {
		t.Fatal(err)
	}
	songs := embedSongs(Link{Source: SourceSpotify, Type: TypeTrack, ID: "7Ea2ts7yPshPjSBHozNV7b"}, e)
	if len(songs) != 1 {
		t.Fatalf("%d songs", len(songs))
	}
	s := songs[0]
	if s.Name != "RAIN COME" || s.artistLine() != "HI HUSTLE" || s.Duration != 204 || s.CoverURL == "" ||
		s.URL != "https://open.spotify.com/track/7Ea2ts7yPshPjSBHozNV7b" || s.Year != "2025" {
		t.Fatalf("song %+v", s)
	}
}

// Live check of the Spotify link reader: KMB_LIVE=1 go test -run TestAnalyzeSpotifyLive ./internal/downloader
func TestAnalyzeSpotifyLive(t *testing.T) {
	if os.Getenv("KMB_LIVE") == "" {
		t.Skip("set KMB_LIVE=1")
	}
	for _, raw := range []string{
		"https://open.spotify.com/intl-id/track/7Ea2ts7yPshPjSBHozNV7b?si=d8ac1b69c71e4436",
		"https://open.spotify.com/album/1DFixLWuPkv3KT3TnV35m3",
		"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M",
	} {
		start := time.Now()
		col, err := AnalyzeSpotify(context.Background(), Env{}, Detect(raw))
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		t.Logf("%s → %q (%s) %d entries in %s", raw, col.Title, col.Subtitle, len(col.Entries), time.Since(start).Round(time.Millisecond))
		if len(col.Entries) == 0 || time.Since(start) > 15*time.Second {
			t.Fatalf("too slow or empty")
		}
	}
}
