package downloader

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kuymediabox/internal/queue"
)

func TestDetectX(t *testing.T) {
	cases := []struct{ in, typ, url, id string }{
		{"https://x.com/SpaceX/status/1732824684683784516", TypePost, "https://x.com/SpaceX/status/1732824684683784516", "1732824684683784516"},
		{"https://twitter.com/UltimaShadowX/status/1577719286659006464/video/2?s=20", TypePost, "https://x.com/UltimaShadowX/status/1577719286659006464", "1577719286659006464"},
		{"mobile.twitter.com/nasa/status/123/photo/1", TypePost, "https://x.com/nasa/status/123", "123"},
		{"https://fxtwitter.com/nasa/status/456", TypePost, "https://x.com/nasa/status/456", "456"},
		{"https://x.com/i/web/status/789", TypePost, "https://x.com/i/status/789", "789"},
		{"https://x.com/i/status/790", TypePost, "https://x.com/i/status/790", "790"},
		{"https://x.com/NASA", TypeProfile, "https://x.com/NASA/media", "NASA"},
		{"https://twitter.com/NASA/media/", TypeProfile, "https://x.com/NASA/media", "NASA"},
		{"https://x.com/home", TypeUnknown, "https://x.com/home", ""},
		{"https://x.com/NASA/likes", TypeUnknown, "https://x.com/NASA/likes", ""},
	}
	for _, c := range cases {
		l := Detect(c.in)
		if l.Source != SourceX || l.Type != c.typ || l.URL != c.url || l.ID != c.id {
			t.Errorf("%s → %+v", c.in, l)
		}
	}
	if Detect("https://dropbox.com/s/abc").Source == SourceX || Detect("https://netflix.com/title/1").Source == SourceX {
		t.Error("other sites must not be taken for X")
	}
	if !IsSocial(SourceX) || len(SplitLinks("lihat x.com/nasa/status/1 dan twitter.com/a/status/2 dropbox.com/x")) != 2 {
		t.Error("x links must be picked up from pasted text")
	}
}

func TestXCollection(t *testing.T) {
	data, err := os.ReadFile("testdata/gallery-x.json")
	if err != nil {
		t.Fatal(err)
	}
	files, err := parseGalleryJSON(string(data))
	if err != nil || len(files) != 5 {
		t.Fatalf("parse: %v %d", err, len(files))
	}
	// 19-digit IDs must survive JSON decoding exactly.
	if id := metaString(files[1].Meta, "tweet_id"); id != "1577719286659006464" {
		t.Fatalf("tweet id %q", id)
	}

	// A profile lists the photo post and the four videos of the other post.
	link := Detect("https://x.com/UltimaShadowX")
	col, err := xPostsCollection(link, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	if col.Type != TypeProfile || col.Title != "@UltimaShadowX" || len(col.Entries) != 5 {
		t.Fatalf("collection %+v", col)
	}
	img := col.Entries[0]
	if img.Kind != KindImage || img.ext != "jpg" || !strings.Contains(img.URL, "name=orig") || !strings.Contains(img.Thumbnail, "name=small") ||
		img.Title != "Big Wedeene River, Canada" || img.ID != "604341487988576256" {
		t.Fatalf("image entry %+v", img)
	}
	for i, e := range col.Entries[1:] {
		if e.Kind != KindVideo || e.URL != "https://x.com/UltimaShadowX/status/1577719286659006464" || e.item != i+1 ||
			!strings.HasPrefix(e.archiveKey, "twitter 15777192362") || e.Duration < 2 || e.Duration > 5 {
			t.Fatalf("video entry %d %+v", i, e)
		}
	}
	if got := PostFolderName(col); got != "X @UltimaShadowX" {
		t.Fatalf("folder %q", got)
	}
	if name := SocialFileName(col, col.Entries[2], 2); name != "UltimaShadowX - Test [1577719286659006464-2]" {
		t.Fatalf("file name %q", name)
	}

	// A post with several videos: numbered items, previews from yt-dlp.
	post := Detect("https://twitter.com/UltimaShadowX/status/1577719286659006464")
	col, err = xPostsCollection(post, files[1:], []ytInfo{{ID: "1577719236201414657", Thumbnail: "https://pbs.twimg.com/thumb.jpg"}})
	if err != nil {
		t.Fatal(err)
	}
	if col.Type != TypePost || col.Title != "Test" || col.Entries[1].Title != "Video 2" || col.Entries[1].Thumbnail == "" || col.Subtitle != "UltimaShadowX" {
		t.Fatalf("post %+v", col)
	}
	// One video is still picked by number: yt-dlp also lists the videos of a quoted post.
	col, _ = xPostsCollection(post, files[1:2], nil)
	if len(col.Entries) != 1 || col.Entries[0].item != 1 || col.Entries[0].Title != "Test" {
		t.Fatalf("single video %+v", col.Entries)
	}
	// The same file twice is listed once; the same ID gets a suffix (entry IDs key the UI list).
	dup := []galleryFile{files[0], files[0], {URL: files[0].URL + "&x=2", Meta: files[0].Meta}}
	col, _ = xPostsCollection(link, dup, nil)
	if len(col.Entries) != 2 || col.Entries[0].ID == col.Entries[1].ID {
		t.Fatalf("duplicates %+v", col.Entries)
	}
	// A GIF link has no media ID: yt-dlp's list gives it by position.
	gif := galleryFile{URL: "https://video.twimg.com/tweet_video/AbC.mp4", Meta: map[string]any{"tweet_id": "5", "type": "animated_gif", "extension": "mp4"}}
	col, _ = xPostsCollection(Detect("https://x.com/a/status/5"), []galleryFile{gif}, []ytInfo{{ID: "77", Thumbnail: "t.jpg"}})
	if e := col.Entries[0]; e.Kind != KindVideo || e.archiveKey != "twitter 77" || e.Thumbnail != "t.jpg" {
		t.Fatalf("gif %+v", e)
	}
	if sourceReferer(img.URL) != "https://x.com/" {
		t.Fatal("referer")
	}
	if _, err := xPostsCollection(post, nil, nil); err == nil {
		t.Fatal("a post without media must fail")
	}
}

func TestXError(t *testing.T) {
	cases := map[string]string{
		`[[-1, {"error": "AuthRequired", "message": "authenticated cookies needed to access this timeline"}]]`: "Login lewat cookies browser",
		`KeyError: 'result'`: "tidak ditemukan",
		"":                   "tidak berisi",
	}
	for in, want := range cases {
		if err := xError(in, Env{}); !strings.Contains(err.Error(), want) {
			t.Errorf("%q → %v", in, err)
		}
	}
	if err := xError("AuthRequired", Env{CookiesBrowser: "firefox"}); !strings.Contains(err.Error(), "kedaluwarsa") {
		t.Errorf("with cookies → %v", err)
	}
	// Without gallery-dl, a photo post read by yt-dlp asks for gallery-dl.
	if err := xYtError(queue.Fail("Gagal", "ERROR: [twitter] 1: No video could be found in this tweet"), Env{}); !strings.Contains(err.Error(), "gallery-dl") {
		t.Errorf("photo post without gallery-dl → %v", err)
	}
}

// Live check: KMB_LIVE=1 go test -run TestXLive ./internal/downloader
func TestXLive(t *testing.T) {
	if os.Getenv("KMB_LIVE") == "" {
		t.Skip("set KMB_LIVE=1")
	}
	bin := filepath.Join(os.Getenv("LOCALAPPDATA"), "KuyMediaBox", "bin")
	env := Env{YtDlp: filepath.Join(bin, "yt-dlp.exe"), FFmpeg: filepath.Join(bin, "ffmpeg.exe"), GalleryDL: filepath.Join(bin, "gallery-dl.exe"), TempDir: t.TempDir()}
	cols := map[string]*Collection{}
	for _, raw := range []string{
		"https://x.com/supernaturepics/status/604341487988576256",
		"https://twitter.com/UltimaShadowX/status/1577719286659006464",
		"https://x.com/SpaceX/status/1732824684683784516",
	} {
		start := time.Now()
		col, err := AnalyzeSocial(context.Background(), env, Detect(raw))
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		cols[raw] = col
		t.Logf("%s → %q (%s) %d items, first %s thumb=%v, %s", raw, col.Title, col.Subtitle, len(col.Entries), col.Entries[0].Kind, col.Entries[0].Thumbnail != "", time.Since(start).Round(time.Millisecond))
	}
	if _, err := AnalyzeSocial(context.Background(), env, Detect("https://x.com/NASA")); err == nil || !strings.Contains(err.Error(), "login") {
		t.Fatalf("profile without login: %v", err)
	}
	probe := func(p string) string {
		out, _ := exec.Command(filepath.Join(bin, "ffprobe.exe"), "-v", "error", "-show_entries", "stream=codec_name", "-of", "csv=p=0", p).Output()
		return strings.Join(strings.Fields(string(out)), ",")
	}
	dir := t.TempDir()
	photo := cols["https://x.com/supernaturepics/status/604341487988576256"]
	out, err := DownloadSocial(context.Background(), env, photo.Entries[0], dir, "photo", Options{Mode: "video", ImageFormat: "original"}, nopReporter{})
	if err != nil || !strings.HasSuffix(out, ".jpg") {
		t.Fatalf("photo: %v %s", err, out)
	}
	multi := cols["https://twitter.com/UltimaShadowX/status/1577719286659006464"]
	for _, mode := range []string{"video", "audio"} {
		o := Options{Mode: mode}
		o.Normalize(SourceX)
		out, err := DownloadSocial(context.Background(), env, multi.Entries[1], dir, "video2-"+mode, o, nopReporter{})
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		dur, _ := exec.Command(filepath.Join(bin, "ffprobe.exe"), "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", out).Output()
		t.Logf("%s → %s [%s] %ss", mode, filepath.Base(out), probe(out), strings.TrimSpace(string(dur)))
		if mode == "audio" && !strings.HasSuffix(out, ".mp3") || mode == "video" && !strings.HasSuffix(out, ".mp4") {
			t.Fatalf("%s: %s", mode, out)
		}
		// The second video of the post is 2.1 s long (the others are 3.7-4.8 s).
		if d := strings.TrimSpace(string(dur)); !strings.HasPrefix(d, "2.") {
			t.Fatalf("%s: picked the wrong video (%s s)", mode, d)
		}
	}
}
