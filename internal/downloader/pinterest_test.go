package downloader

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDetectPinterest(t *testing.T) {
	cases := []struct{ in, typ, url, id string }{
		{"https://www.pinterest.com/pin/1125968651834556/", TypePost, "https://www.pinterest.com/pin/1125968651834556/", "1125968651834556"},
		{"https://id.pinterest.com/pin/311944711712319941/sent/?invite_code=x", TypePost, "https://www.pinterest.com/pin/311944711712319941/", "311944711712319941"},
		{"pinterest.co.uk/pin/some-title--123456789/", TypePost, "https://www.pinterest.com/pin/123456789/", "123456789"},
		{"https://pin.it/abcde12", TypePost, "https://pin.it/abcde12", ""},
		{"https://www.pinterest.com/ccl1313/4-the-love-of-fur/", TypeBoard, "https://www.pinterest.com/ccl1313/4-the-love-of-fur/", "ccl1313/4-the-love-of-fur"},
		{"https://www.pinterest.com/ccl1313/", TypeProfile, "https://www.pinterest.com/ccl1313/pins/", "ccl1313"},
		{"https://www.pinterest.com/ccl1313/_created/", TypeProfile, "https://www.pinterest.com/ccl1313/_created/", "ccl1313"},
		{"https://www.pinterest.com/search/pins/?q=kucing%20lucu&rs=typed", TypeSearch, "https://www.pinterest.com/search/pins/?q=kucing+lucu", "kucing lucu"},
		{"https://www.pinterest.com/ideas/", TypeUnknown, "https://www.pinterest.com/ideas/", ""},
	}
	for _, c := range cases {
		l := Detect(c.in)
		if l.Source != SourcePinterest || l.Type != c.typ || l.URL != c.url || l.ID != c.id {
			t.Errorf("%s → %+v", c.in, l)
		}
	}
	if !IsSocial(SourcePinterest) || len(SplitLinks("lihat pin.it/abc12 dan www.pinterest.com/pin/1/")) != 2 {
		t.Error("pinterest links must be picked up from pasted text")
	}
}

func TestPinCollection(t *testing.T) {
	data, err := os.ReadFile("testdata/gallery-pinterest-search.json")
	if err != nil {
		t.Fatal(err)
	}
	files, err := parseGalleryJSON(string(data))
	if err != nil || len(files) != 3 {
		t.Fatalf("parse: %v %d", err, len(files))
	}
	link := Detect("https://www.pinterest.com/search/pins/?q=kucing")
	col, err := pinCollection(link, files)
	if err != nil {
		t.Fatal(err)
	}
	if col.Type != TypeSearch || col.Title != "kucing" || len(col.Entries) != 3 {
		t.Fatalf("collection %+v", col)
	}
	img, vid := col.Entries[0], col.Entries[2]
	if img.Kind != KindImage || !strings.Contains(img.URL, "pinimg.com") || img.ext != "webp" || img.Thumbnail == "" {
		t.Fatalf("image entry %+v", img)
	}
	if vid.Kind != KindVideo || !strings.HasPrefix(vid.URL, "https://www.pinterest.com/pin/") || vid.archiveKey == "" || vid.Duration < 10 || vid.Duration > 30 {
		t.Fatalf("video entry %+v", vid)
	}
	name := SocialFileName(col, col.Entries[1], 2)
	if !strings.Contains(name, "[311944711712319941]") {
		t.Fatalf("file name %q", name)
	}
	if got := PostFolderName(col); got != "Pinterest cari kucing" {
		t.Fatalf("folder %q", got)
	}
	if sourceReferer(img.URL) != "https://www.pinterest.com/" {
		t.Fatal("referer")
	}
}

// Live check: KMB_LIVE=1 go test -run TestPinterestLive ./internal/downloader
func TestPinterestLive(t *testing.T) {
	if os.Getenv("KMB_LIVE") == "" {
		t.Skip("set KMB_LIVE=1")
	}
	env := Env{YtDlp: "yt-dlp", GalleryDL: os.Getenv("LOCALAPPDATA") + `\KuyMediaBox\bin\gallery-dl.exe`, TempDir: t.TempDir()}
	for _, raw := range []string{
		"https://www.pinterest.com/pin/311944711712319941/",
		"https://www.pinterest.com/pin/1125968651834556/",
		"https://www.pinterest.com/ccl1313/4-the-love-of-fur/",
		"https://www.pinterest.com/ccl1313/_created/",
		"https://www.pinterest.com/search/pins/?q=kucing%20lucu",
	} {
		start := time.Now()
		col, err := AnalyzeSocial(context.Background(), env, Detect(raw))
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		t.Logf("%s → %q (%s) %d items, first %s, %s", raw, col.Title, col.Subtitle, len(col.Entries), col.Entries[0].Kind, time.Since(start).Round(time.Millisecond))
	}
}

func TestPinEdgeCases(t *testing.T) {
	vid := func(url string) galleryFile {
		return galleryFile{URL: url, Meta: map[string]any{"id": "AbC-12_xyz", "extension": "mp4"}}
	}
	img := galleryFile{URL: "https://i.pinimg.com/originals/a.jpg", Meta: map[string]any{"id": "AbC-12_xyz", "extension": "jpg"}}
	// A pin with two videos: each keeps its own stream (the pin page would give yt-dlp one video).
	files := []galleryFile{vid("ytdl:https://v1.pinimg.com/1.m3u8"), vid("ytdl:https://v1.pinimg.com/2.m3u8"), img, img}
	col, err := pinCollection(Detect("https://www.pinterest.com/pin/AbC-12_xyz/"), files)
	if err != nil {
		t.Fatal(err)
	}
	if len(col.Entries) != 3 || col.Entries[0].URL != "https://v1.pinimg.com/1.m3u8" || col.Entries[1].URL != "https://v1.pinimg.com/2.m3u8" || col.Entries[0].archiveKey != "" {
		t.Fatalf("entries %+v", col.Entries)
	}
	if col.Entries[0].ID == col.Entries[1].ID || col.Entries[1].ID == col.Entries[2].ID || col.postID != "AbC-12_xyz" {
		t.Fatalf("ids %q %q %q post %q", col.Entries[0].ID, col.Entries[1].ID, col.Entries[2].ID, col.postID)
	}
	// One video: downloaded from the pin page, remembered in the archive.
	col, _ = pinCollection(Detect("https://www.pinterest.com/pin/AbC-12_xyz/"), files[:1])
	if e := col.Entries[0]; e.URL != "https://www.pinterest.com/pin/AbC-12_xyz/" || e.archiveKey != "pinterest AbC-12_xyz" {
		t.Fatalf("single video %+v", e)
	}

	cases := map[string]string{
		`NotFoundError: Requested pin could not be found`: "tidak ditemukan",
		`AuthRequired: authenticated cookies needed`:      "Login lewat cookies browser",
		"": "Tidak ada foto",
	}
	for in, want := range cases {
		if err := pinError(in, Env{}); !strings.Contains(err.Error(), want) {
			t.Errorf("%q → %v", in, err)
		}
	}
}
