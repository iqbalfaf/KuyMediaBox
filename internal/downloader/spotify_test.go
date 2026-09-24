package downloader

import "testing"

func TestSortSongs(t *testing.T) {
	album := []spotifySong{
		{Name: "b", DiscNumber: 1, TrackNumber: 2},
		{Name: "d", DiscNumber: 2, TrackNumber: 1},
		{Name: "a", DiscNumber: 1, TrackNumber: 1},
		{Name: "c", DiscNumber: 1, TrackNumber: 3},
	}
	sortSongs(album, true)
	got := ""
	for _, s := range album {
		got += s.Name
	}
	if got != "abcd" {
		t.Fatalf("album order %s", got)
	}

	list := []spotifySong{{Name: "y", ListPosition: 2}, {Name: "x", ListPosition: 1}}
	sortSongs(list, false)
	if list[0].Name != "x" {
		t.Fatalf("playlist order %v", list)
	}
}

func TestSpotifyFileName(t *testing.T) {
	e := Entry{Title: "Lagu: Satu?", Artist: "Band A, Band B", Index: 3}
	if got := SpotifyFileName(e, true, 2); got != "03 - Band A, Band B - Lagu_ Satu_" {
		t.Fatalf("got %q", got)
	}
	if got := SpotifyFileName(e, false, 2); got != "Band A, Band B - Lagu_ Satu_" {
		t.Fatalf("got %q", got)
	}
}
