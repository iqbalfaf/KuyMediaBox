package downloader

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct {
		in, source, typ, url string
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PL123", SourceYouTube, TypeVideo, "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
		{"youtu.be/dQw4w9WgXcQ?si=abc", SourceYouTube, TypeVideo, "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
		{"https://youtube.com/shorts/dQw4w9WgXcQ", SourceYouTube, TypeVideo, "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
		{"https://m.youtube.com/playlist?list=PLabc_DEF", SourceYouTube, TypePlaylist, "https://www.youtube.com/playlist?list=PLabc_DEF"},
		{"https://www.youtube.com/@dapursantai/videos", SourceYouTube, TypeChannel, "https://www.youtube.com/@dapursantai"},
		{"https://www.youtube.com/channel/UC123abc", SourceYouTube, TypeChannel, "https://www.youtube.com/channel/UC123abc"},
		{"https://www.youtube.com/c/NamaLama/featured", SourceYouTube, TypeChannel, "https://www.youtube.com/c/NamaLama"},
		{"https://open.spotify.com/intl-id/track/4uLU6hMCjMI75M1A2tKUQC?si=x", SourceSpotify, TypeTrack, "https://open.spotify.com/track/4uLU6hMCjMI75M1A2tKUQC"},
		{"https://open.spotify.com/album/1DFixLWuPkv3KT3TnV35m3", SourceSpotify, TypeAlbum, "https://open.spotify.com/album/1DFixLWuPkv3KT3TnV35m3"},
		{"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M", SourceSpotify, TypePlaylist, "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M"},
		{"https://open.spotify.com/artist/abc", SourceSpotify, TypeUnknown, "https://open.spotify.com/artist/abc"},
		{"https://vimeo.com/123", SourceOther, TypeVideo, "https://vimeo.com/123"},
		{"", "", TypeUnknown, ""},
	}
	for _, c := range cases {
		got := Detect(c.in)
		if got.Source != c.source || got.Type != c.typ || got.URL != c.url {
			t.Errorf("%q → %+v", c.in, got)
		}
	}
}

func TestSplitLinks(t *testing.T) {
	got := SplitLinks("https://youtu.be/dQw4w9WgXcQ\n  open.spotify.com/album/x  https://youtu.be/dQw4w9WgXcQ halo")
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}
