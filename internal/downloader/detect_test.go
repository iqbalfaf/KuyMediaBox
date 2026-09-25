package downloader

import (
	"strings"
	"testing"
)

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
		{"https://open.spotify.com/artist/abc", SourceSpotify, TypeArtist, "https://open.spotify.com/artist/abc"},
		{"https://vimeo.com/123", SourceOther, TypeVideo, "https://vimeo.com/123"},
		{"https://www.tiktok.com/@hihustleband/video/7677609057787612436?is_from_webapp=1", SourceTikTok, TypePost, "https://www.tiktok.com/@hihustleband/video/7677609057787612436"},
		{"tiktok.com/@hullcity/photo/7557376330036153622", SourceTikTok, TypePost, "https://www.tiktok.com/@hullcity/photo/7557376330036153622"},
		{"https://vt.tiktok.com/ZSabc123/", SourceTikTok, TypePost, "https://vt.tiktok.com/ZSabc123/"},
		{"https://www.tiktokv.com/share/video/7240568259186019630", SourceTikTok, TypePost, "https://www.tiktok.com/@_/video/7240568259186019630"},
		{"https://www.tiktok.com/@hullcity", SourceTikTok, TypeProfile, "https://www.tiktok.com/@hullcity"},
		{"https://www.tiktok.com/explore", SourceTikTok, TypeUnknown, "https://www.tiktok.com/explore"},
		{"https://www.instagram.com/p/DbDxIqNCjrI/?img_index=1", SourceInstagram, TypePost, "https://www.instagram.com/p/DbDxIqNCjrI/"},
		{"https://instagram.com/reels/CDg_6Y1pxWu", SourceInstagram, TypePost, "https://www.instagram.com/reel/CDg_6Y1pxWu/"},
		{"https://www.instagram.com/faizaltwelve/reel/CDg_6Y1pxWu/", SourceInstagram, TypePost, "https://www.instagram.com/reel/CDg_6Y1pxWu/"},
		{"https://www.instagram.com/stories/someone/123/", SourceInstagram, TypeUnknown, "https://www.instagram.com/stories/someone/123/"},
		{"https://www.facebook.com/share/v/1JUTPEWM3e/", SourceFacebook, TypePost, "https://www.facebook.com/share/v/1JUTPEWM3e/"},
		{"https://m.facebook.com/watch/?v=1081798797911783&_rdr", SourceFacebook, TypePost, "https://www.facebook.com/watch/?v=1081798797911783"},
		{"https://www.facebook.com/reel/1195289147628387?mibextid=x", SourceFacebook, TypePost, "https://www.facebook.com/reel/1195289147628387"},
		{"https://www.facebook.com/photo/?fbid=10152716011076729&set=a.10152716010956729", SourceFacebook, TypePost, "https://www.facebook.com/photo/?fbid=10152716011076729&set=a.10152716010956729"},
		{"https://fb.watch/abcDEF/", SourceFacebook, TypePost, "https://fb.watch/abcDEF/"},
		{"https://www.facebook.com/facebook", SourceFacebook, TypeUnknown, "https://www.facebook.com/facebook"},
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
	got = SplitLinks("lihat ini www.tiktok.com/@a/video/1 dan instagram.com/p/X1 juga fb.watch/abc")
	if len(got) != 3 {
		t.Fatalf("social without scheme: got %v", got)
	}
}

func TestDetectPhotoHint(t *testing.T) {
	for in, want := range map[string]bool{
		"https://www.tiktok.com/@a/photo/1":               true,
		"https://www.tiktok.com/@a/video/1":               false,
		"https://www.instagram.com/p/X1/":                 true,
		"https://www.instagram.com/reel/X1/":              false,
		"https://www.facebook.com/photo.php?fbid=1":       true,
		"https://www.facebook.com/share/p/abc/":           true,
		"https://www.facebook.com/share/v/abc/":           false,
		"https://www.facebook.com/somepage/videos/12345/": false,
		"https://www.facebook.com/somepage/photos/a.1/2/": true,
	} {
		if got := Detect(in); got.Photo != want || got.Type != TypePost {
			t.Errorf("%s → %+v, want photo=%v", in, got, want)
		}
	}
}

func TestTemplatesAndSections(t *testing.T) {
	if got := YtTemplate("{uploader} - {title} [{id}] 100%", 7); got != "%(uploader,channel|)s - %(title).150B [%(id)s] 100%%" {
		t.Fatalf("yt template %q", got)
	}
	if got := YtTemplate("{index}. {title} {unknown}", 3); got != "03. %(title).150B {unknown}" {
		t.Fatalf("index template %q", got)
	}
	e := Entry{Title: "Lagu", Artist: "Band", Album: "Album", Index: 4}
	if got := SpotifyFileName(e, true, 2, "{artist} - {title} ({album})"); got != "04 - Band - Lagu (Album)" {
		t.Fatalf("spotify template %q", got)
	}
	if got := SpotifyFileName(e, true, 2, "{index} {title} - {year}"); got != "04 Lagu" {
		t.Fatalf("empty token template %q", got)
	}
	for in, want := range map[[2]string]string{{"1:00", "2:30"}: "*60-150", {"90", ""}: "*90-inf", {"", ""}: "", {"2:00", "1:00"}: ""} {
		if got := sectionArg(in[0], in[1]); got != want {
			t.Errorf("section %v = %q, want %q", in, got, want)
		}
	}
	o := Options{SubLangs: "id, en; rm -rf", Subtitles: "embed", SponsorBlock: "remove"}
	o.Normalize(SourceTikTok)
	if o.SubLangs != "id" || o.SponsorBlock != "off" {
		t.Fatalf("normalize %+v", o)
	}
}

func TestYtArgsExtras(t *testing.T) {
	e := Env{YtDlp: "yt-dlp", CookiesBrowser: "firefox"}
	o := Options{Mode: "video", Quality: "1080", Container: "mp4", Subtitles: "embed", SubLangs: "id,en", SectionStart: "1:00", SectionEnd: "2:00", SponsorBlock: "remove"}
	args := strings.Join(e.ytArgs(ytJob{URL: "u", Dir: "d", Template: "t", Opts: o, PathFile: "p"}), " ")
	for _, want := range []string{"--cookies-from-browser firefox", "--embed-subs", "--sub-langs id,en", "--download-sections *60-120", "--sponsorblock-remove sponsor,selfpromo,interaction"} {
		if !strings.Contains(args, want) {
			t.Errorf("missing %q in %s", want, args)
		}
	}
	if strings.Contains(args, "--write-subs") {
		t.Error("embed mode must not keep .srt files")
	}
	e.CookiesFile = `C:\c.txt`
	if a := strings.Join(e.cookieArgs(), " "); a != `--cookies C:\c.txt` {
		t.Errorf("cookie file args %q", a)
	}
}

func TestRateLimitArg(t *testing.T) {
	e := Env{YtDlp: "yt-dlp", RateLimitKB: 512}
	args := strings.Join(e.ytArgs(ytJob{URL: "u", Dir: "d", Template: "t", Opts: Options{Mode: "audio", AudioFormat: "mp3"}, PathFile: "p"}), " ")
	if !strings.Contains(args, "--limit-rate 512K") {
		t.Fatalf("missing limit: %s", args)
	}
}
