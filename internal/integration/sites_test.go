//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	"kuymediabox/internal/downloader"
	"kuymediabox/internal/queue"
)

// Public links on the sites with their own tab. Sites change: a link that disappears only
// fails its own subtest.
func TestMoreSitesAnalyze(t *testing.T) {
	m := setup(t)
	e := env(m)
	for _, c := range []struct {
		name, url string
		min       int
	}{
		{"soundcloud-track", "https://soundcloud.com/forss/flickermood", 1},
		{"soundcloud-set", "https://soundcloud.com/the-concept-band/sets/the-royal-concept-ep", 2},
		{"bilibili-video", "https://www.bilibili.com/video/BV13x41117TL", 1},
		{"reddit-video", "https://www.reddit.com/r/videos/comments/6rrwyj/that_small_heart_attack/", 1},
		{"reddit-picture", "https://www.reddit.com/r/lavaporn/comments/8cqhub/", 1},
		{"reddit-gallery", "https://www.reddit.com/gallery/hrrh23", 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			link := downloader.Detect(c.url)
			var col *downloader.Collection
			var err error
			if downloader.IsSocial(link.Source) {
				col, err = downloader.AnalyzeSocial(context.Background(), e, link)
			} else {
				col, err = downloader.AnalyzeYouTube(context.Background(), e, link)
			}
			if err != nil {
				t.Fatalf("%s %s: %v", link.Source, link.Type, err)
			}
			kinds := map[string]int{}
			for _, en := range col.Entries {
				kinds[en.Kind]++
			}
			t.Logf("%s/%s | %q by %q | %d entries %v", col.Source, col.Type, col.Title, col.Subtitle, len(col.Entries), kinds)
			if len(col.Entries) < c.min {
				t.Errorf("%d entries, want at least %d", len(col.Entries), c.min)
			}
		})
	}
}

func TestSoundCloudDownload(t *testing.T) {
	m := setup(t)
	e := env(m)
	link := downloader.Detect("https://soundcloud.com/forss/flickermood")
	col, err := downloader.AnalyzeYouTube(context.Background(), e, link)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	o := downloader.Options{Mode: "audio", AudioFormat: "mp3"}
	o.Normalize(col.Source)
	out, err := downloader.DownloadYouTube(context.Background(), e, col.Entries[0], dir, "", false, o, nopReporter{})
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(out)
	if err != nil || st.Size() < 100_000 {
		t.Fatalf("result %s: %v", out, err)
	}
	t.Logf("%s (%d KB)", out, st.Size()/1024)
}

type nopReporter struct{}

func (nopReporter) Progress(float64)        {}
func (nopReporter) Message(string)          {}
func (nopReporter) SetOutput(string, int64) {}

var _ queue.Reporter = nopReporter{}
