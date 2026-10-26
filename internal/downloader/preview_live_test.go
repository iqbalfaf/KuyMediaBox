package downloader

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// KMB_LIVE=1: a preview stream of "Me at the zoo" can be read with the headers yt-dlp gave.
func TestPreviewStreamLive(t *testing.T) {
	if os.Getenv("KMB_LIVE") == "" {
		t.Skip()
	}
	bin := filepath.Join(os.Getenv("LOCALAPPDATA"), "KuyMediaBox", "bin")
	node, _ := exec.LookPath("node")
	env := Env{YtDlp: filepath.Join(bin, "yt-dlp.exe"), JSKind: "node", JSPath: node}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s, err := PreviewStream(ctx, env, "https://www.youtube.com/watch?v=jNQXAC9IVRw")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("protocol %s ext %s video %v audio %v playable %v", s.Protocol, s.Ext, s.Video, s.Audio, s.Playable())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", "bytes=0-1023")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("range request: %s", resp.Status)
	}
}
