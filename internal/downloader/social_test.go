package downloader

import (
	"errors"
	"os"
	"strings"
	"testing"

	"kuymediabox/internal/queue"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGalleryTikTokPhoto(t *testing.T) {
	out := readFixture(t, "gallery-tiktok-photo.json")
	files, err := parseGalleryJSON(out)
	if err != nil || len(files) != 2 {
		t.Fatalf("parse: %v, %d files", err, len(files))
	}
	link := Detect("https://www.tiktok.com/@hullcity/photo/7557376330036153622")
	col, err := galleryCollection(link, files, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(col.Entries) != 1 || col.Entries[0].Kind != KindImage || col.Entries[0].ext != "jpeg" {
		t.Fatalf("entries %+v", col.Entries)
	}
	if !col.hasSound {
		t.Fatal("mp3 of the slideshow not noticed")
	}
	if col.uploader != "hullcity" || col.Title != "#hcafc #thetigers #dave #goat" || col.postID != "7557376330036153622" {
		t.Fatalf("meta: uploader=%q title=%q id=%q", col.uploader, col.Title, col.postID)
	}
	name := SocialFileName(col, col.Entries[0], 2)
	if name != "hullcity - #hcafc #thetigers #dave #goat [7557376330036153622]" {
		t.Fatalf("file name %q", name)
	}
}

func TestGalleryFacebookPhoto(t *testing.T) {
	out := readFixture(t, "gallery-facebook-photo.json")
	files, err := parseGalleryJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	col, err := galleryCollection(Detect("https://www.facebook.com/photo.php?fbid=10165113568399554"), files, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(col.Entries) != 1 || col.Entries[0].ext != "jpg" || col.uploader == "" {
		t.Fatalf("collection %+v", col)
	}
}

func TestGalleryLoginError(t *testing.T) {
	_, err := parseGalleryJSON(readFixture(t, "gallery-instagram-login.json"))
	if err == nil {
		t.Fatal("expected an error message")
	}
	var ue *queue.UserError
	if !errors.As(friendlySocialError(err.Error()), &ue) || !strings.Contains(ue.Message, "login") {
		t.Fatalf("got %v", friendlySocialError(err.Error()))
	}
}

func TestFriendlySocialError(t *testing.T) {
	cases := map[string]string{
		"ERROR: [TikTok] 674: Your IP address is blocked from accessing this post":                       "TikTok",
		"ERROR: [Instagram] x: Requested content is not available, rate-limit reached or login required": "login",
		"[gallery-dl][error] Unsupported URL 'https://www.facebook.com/x'":                               "tidak didukung",
		"ERROR: [facebook] 1: HTTP Error 404: Not Found":                                                 "dihapus",
	}
	for in, want := range cases {
		var ue *queue.UserError
		if err := friendlySocialError(in); !errors.As(err, &ue) || !strings.Contains(ue.Message, want) {
			t.Errorf("%q → %v (want %q)", in, err, want)
		}
	}
}

func TestCleanCaption(t *testing.T) {
	cases := []struct{ source, title, desc, uploader, want string }{
		{SourceFacebook, "10K views · 679 reactions | Drain You..♡ | HEART SHAPED BOX", "", "HEART SHAPED BOX", "Drain You..♡"},
		{SourceInstagram, "Video by faizaltwelve", "Latihan hari ini\n#futsal", "Muhammed Faisal", "Latihan hari ini #futsal"},
		{SourceTikTok, "The Sigit - Save Me  #thesigit", "", "hihustleband", "The Sigit - Save Me #thesigit"},
		{SourceInstagram, "Photo by x", "", "x", "Photo by x"},
	}
	for _, c := range cases {
		if got := cleanCaption(c.source, c.title, c.desc, c.uploader); got != c.want {
			t.Errorf("cleanCaption(%q) = %q, want %q", c.title, got, c.want)
		}
	}
	if got := cleanCaption(SourceTikTok, strings.Repeat("a", 300), "", ""); len([]rune(got)) != 121 {
		t.Errorf("long caption not shortened: %d", len([]rune(got)))
	}
}

func TestMediaKind(t *testing.T) {
	if mediaKind(ytInfo{}) != KindImage {
		t.Error("no formats must be a picture")
	}
	if mediaKind(ytInfo{Formats: []ytFmt{{VCodec: "none", ACodec: "mp3"}}}) != KindAudio {
		t.Error("audio-only must be sound")
	}
	if mediaKind(ytInfo{Formats: []ytFmt{{VCodec: "none", ACodec: "mp4a"}, {VCodec: "h264", ACodec: "none"}}}) != KindVideo {
		t.Error("video formats must be a video")
	}
}

func TestExtFromURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://p16.tiktokcdn.com/obj/abc~tplv-photomode-image.jpeg?x=1": "jpeg",
		"https://scontent.cdninstagram.com/v/t51/123_n.jpg?stp=dst":       "jpg",
		"https://scontent.fbcdn.net/v/t39/468_n.png?_nc=1":                "png",
		"https://example.com/image":                                       "jpg",
	} {
		if got := extFromURL(in); got != want {
			t.Errorf("extFromURL(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestSocialNames(t *testing.T) {
	col := &Collection{Source: SourceInstagram, Type: TypePost, Title: "Liburan: pantai/laut?", postID: "BoHk1haB5tM", uploader: "j9ryl"}
	col.Entries = []Entry{{Kind: KindImage, Index: 1}, {Kind: KindVideo, Index: 2}}
	if got := SocialFileName(col, col.Entries[1], 2); got != "j9ryl [BoHk1haB5tM] 02" {
		t.Errorf("file name %q", got)
	}
	if got := PostFolderName(col); got != "j9ryl - Liburan_ pantai_laut_ [BoHk1haB5tM]" {
		t.Errorf("folder %q", got)
	}
	tt := &Collection{Source: SourceTikTok, Type: TypePost, Title: "x", postID: "1", uploader: "u"}
	tt.Entries = []Entry{{Kind: KindImage, Index: 1}, {Kind: KindAudio, Index: 2}}
	if got := SocialFileName(tt, tt.Entries[1], 2); !strings.HasSuffix(got, "[1] musik") {
		t.Errorf("sound name %q", got)
	}
}

func TestOptionsImageFormat(t *testing.T) {
	o := Options{ImageFormat: "png"}
	o.Normalize(SourceInstagram)
	if o.ImageFormat != "original" {
		t.Fatalf("got %q", o.ImageFormat)
	}
	o = Options{ImageFormat: "jpg"}
	o.Normalize(SourceTikTok)
	if o.ImageFormat != "jpg" {
		t.Fatalf("got %q", o.ImageFormat)
	}
}
