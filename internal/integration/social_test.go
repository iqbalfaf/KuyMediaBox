//go:build integration

package integration

import (
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kuymediabox/internal/downloader"
	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/tools"
)

// Public posts (no login). Social sites change often: a post that disappears only fails its own subtest.
var socialCases = []struct {
	name, url string
	wantKinds []string // kinds expected among the entries
}{
	{"tiktok-video", "https://www.tiktok.com/@hihustleband/video/7677609057787612436", []string{downloader.KindVideo}},
	{"tiktok-photo", "https://www.tiktok.com/@hullcity/photo/7557376330036153622", []string{downloader.KindImage, downloader.KindAudio}},
	{"tiktok-photo-as-video-link", "https://www.tiktok.com/@hullcity/video/7557376330036153622", []string{downloader.KindImage}},
	{"instagram-video-post", "https://www.instagram.com/p/DbDxIqNCjrI/", []string{downloader.KindVideo}},
	{"instagram-photo", "https://www.instagram.com/p/BqvsDleB3lV/", []string{downloader.KindImage}},
	{"instagram-carousel", "https://www.instagram.com/p/BoHk1haB5tM/", []string{downloader.KindImage}},
	{"facebook-share-video", "https://www.facebook.com/share/v/1JUTPEWM3e/", []string{downloader.KindVideo}},
	{"facebook-photo", "https://www.facebook.com/photo.php?fbid=10165113568399554&set=t.100064860875397&type=3", []string{downloader.KindImage}},
}

func TestSocialAnalyze(t *testing.T) {
	m := setup(t)
	if m.Path(tools.GalleryDL) == "" {
		t.Fatal("gallery-dl not installed")
	}
	e := env(m)
	for _, c := range socialCases {
		t.Run(c.name, func(t *testing.T) {
			link := downloader.Detect(c.url)
			col, err := downloader.AnalyzeSocial(context.Background(), e, link)
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			kinds := map[string]int{}
			for _, en := range col.Entries {
				kinds[en.Kind]++
			}
			t.Logf("%s | %q by %q | %d entries %v", col.Type, col.Title, col.Subtitle, len(col.Entries), kinds)
			for _, k := range c.wantKinds {
				if kinds[k] == 0 {
					t.Errorf("no %s entry", k)
				}
			}
			if col.Title == "" || strings.Contains(col.Title, "views ·") {
				t.Errorf("bad title %q", col.Title)
			}
		})
	}
}

func TestSocialDownload(t *testing.T) {
	m := setup(t)
	e := env(m)
	dir := t.TempDir()
	ctx := context.Background()
	get := func(t *testing.T, url string) *downloader.Collection {
		col, err := downloader.AnalyzeSocial(ctx, e, downloader.Detect(url))
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		return col
	}
	download := func(t *testing.T, col *downloader.Collection, en downloader.Entry, o downloader.Options) string {
		o.Normalize(col.Source)
		name := downloader.SocialFileName(col, en, 2)
		r := &rep{t: t}
		out, err := downloader.DownloadSocial(ctx, e, en, dir, name, o, r)
		if err != nil {
			t.Fatalf("download %s: %v", en.Title, err)
		}
		st, serr := os.Stat(out)
		if serr != nil || st.Size() == 0 {
			t.Fatalf("output %s missing", out)
		}
		t.Logf("→ %s (%d KB)", filepath.Base(out), st.Size()/1024)
		return out
	}

	t.Run("tiktok-video-mp4", func(t *testing.T) {
		col := get(t, socialCases[0].url)
		out := download(t, col, col.Entries[0], downloader.Options{Mode: "video", Quality: "720", Container: "mp4"})
		info, err := ffmpeg.Probe(ctx, m.Path(tools.FFprobe), out)
		if err != nil || !info.HasVideo || !info.HasAudio {
			t.Fatalf("probe %+v %v", info, err)
		}
	})
	t.Run("tiktok-video-as-mp3", func(t *testing.T) {
		col := get(t, socialCases[0].url)
		out := download(t, col, col.Entries[0], downloader.Options{Mode: "audio", AudioFormat: "mp3"})
		if !strings.HasSuffix(out, ".mp3") {
			t.Fatalf("got %s", out)
		}
	})
	t.Run("tiktok-photo-jpg-and-sound", func(t *testing.T) {
		col := get(t, socialCases[1].url)
		for _, en := range col.Entries {
			out := download(t, col, en, downloader.Options{ImageFormat: "jpg", AudioFormat: "mp3"})
			if en.Kind == downloader.KindImage {
				f, _ := os.Open(out)
				_, format, err := image.DecodeConfig(f)
				f.Close()
				if err != nil || format != "jpeg" || !strings.HasSuffix(out, ".jpg") {
					t.Fatalf("picture %s: %s %v", out, format, err)
				}
			}
		}
	})
	t.Run("instagram-carousel-pictures", func(t *testing.T) {
		col := get(t, socialCases[5].url)
		if len(col.Entries) < 2 {
			t.Fatalf("carousel has %d entries", len(col.Entries))
		}
		for _, en := range col.Entries[:2] {
			download(t, col, en, downloader.Options{})
		}
	})
	t.Run("instagram-video", func(t *testing.T) {
		col := get(t, socialCases[3].url)
		download(t, col, col.Entries[0], downloader.Options{Mode: "video", Quality: "best", Container: "mp4"})
	})
	t.Run("facebook-video", func(t *testing.T) {
		col := get(t, socialCases[6].url)
		download(t, col, col.Entries[0], downloader.Options{Mode: "video", Quality: "720", Container: "mp4"})
	})
	t.Run("facebook-photo-skip-existing", func(t *testing.T) {
		col := get(t, socialCases[7].url)
		o := downloader.Options{SkipExisting: true}
		download(t, col, col.Entries[0], o)
		o.Normalize(col.Source)
		_, err := downloader.DownloadSocial(ctx, e, col.Entries[0], dir, downloader.SocialFileName(col, col.Entries[0], 2), o, &rep{t: t})
		if !errors.Is(err, queue.ErrSkipped) {
			t.Fatalf("second download should be skipped, got %v", err)
		}
	})
}

func TestSocialErrors(t *testing.T) {
	m := setup(t)
	e := env(m)
	for _, url := range []string{
		"https://www.instagram.com/stories/instagram/123/",
		"https://www.tiktok.com/@kmb_nobody_zz/video/1",
	} {
		_, err := downloader.AnalyzeSocial(context.Background(), e, downloader.Detect(url))
		var ue *queue.UserError
		if !errors.As(err, &ue) || ue.Message == "" {
			t.Errorf("%s: want a clear user error, got %v", url, err)
			continue
		}
		t.Logf("%s → %s", url, ue.Message)
	}
}
