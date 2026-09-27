//go:build integration

package downloader

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchLyricsLive(t *testing.T) {
	l, err := FetchLyrics(context.Background(), "Coldplay", "Yellow", "Parachutes", 267)
	if err != nil || (l.Synced == "" && l.Plain == "") {
		t.Fatalf("lyrics: %v %+v", err, l)
	}
	dir := t.TempDir()
	audio := filepath.Join(dir, "Coldplay - Yellow.mp3")
	p, err := WriteLRC(audio, l, "Coldplay", "Yellow", "Parachutes")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), "Look at the stars") {
		t.Errorf("%s: %q", p, string(data)[:80])
	}
	if l.Synced != "" && (!strings.HasSuffix(p, ".lrc") || !strings.Contains(string(data), "[ar:Coldplay]")) {
		t.Errorf("synced lyrics must be an .lrc with tags: %s", p)
	}
	if _, err := FetchLyrics(context.Background(), "Nobody Zzqx", "No Such Song Qqzx", "", 100); err == nil {
		t.Error("unknown song must fail")
	}
}
