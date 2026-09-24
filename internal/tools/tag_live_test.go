package tools

import (
	"context"
	"os"
	"testing"
)

func TestLatestTagLive(t *testing.T) {
	if os.Getenv("KMB_LIVE") == "" {
		t.Skip()
	}
	for _, repo := range []string{"yt-dlp/yt-dlp", "spotDL/spotify-downloader"} {
		tag, err := latestTag(context.Background(), repo)
		if err != nil || tag == "" {
			t.Fatalf("%s: %q %v", repo, tag, err)
		}
		t.Logf("%s → %s", repo, tag)
	}
}
