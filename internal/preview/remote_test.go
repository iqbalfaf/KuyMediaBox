package preview

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoteProxy(t *testing.T) {
	data := bytes.Repeat([]byte("01234567"), remoteChunk/4) // exactly 2 chunks
	var gotUA string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		if r.URL.Path == "/norange" {
			_, _ = w.Write(data[:100])
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	defer up.Close()
	h := Middleware(func() string { return "" }, http.NotFoundHandler())
	get := func(id, rng string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/kmb/remote?id="+id, nil)
		if rng != "" {
			req.Header.Set("Range", rng)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	id := AddRemote(up.URL+"/v.mp4", map[string]string{"User-Agent": "KMB-Test"})
	// The player asks for everything; it gets the first part only.
	rec := get(id, "bytes=0-")
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != remoteChunk || gotUA != "KMB-Test" {
		t.Fatalf("first part: %d, %d bytes, UA %q", rec.Code, rec.Body.Len(), gotUA)
	}
	if cr := rec.Header().Get("Content-Range"); cr != "bytes 0-4194303/8388608" || rec.Header().Get("Content-Type") != "video/mp4" {
		t.Errorf("headers: %v", rec.Header())
	}
	rec = get(id, "bytes=8388600-8388607")
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "01234567" {
		t.Errorf("small range: %d %q", rec.Code, rec.Body.String())
	}
	if rec := get("nope", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown id: %d", rec.Code)
	}
	if rec := get(AddRemote(up.URL+"/norange", nil), ""); rec.Code != http.StatusBadGateway {
		t.Errorf("a server without ranges must fail (frames are used instead): %d", rec.Code)
	}

	// Only the newest streams are kept.
	for range maxRemotes + 3 {
		AddRemote(up.URL, nil)
	}
	if remote(id) != nil || len(remotes.m) > maxRemotes {
		t.Errorf("old streams must be dropped (%d kept)", len(remotes.m))
	}
}

func TestRemoteFrame(t *testing.T) {
	ff := filepath.Join(os.Getenv("LOCALAPPDATA"), "KuyMediaBox", "bin", "ffmpeg.exe")
	if _, err := os.Stat(ff); err != nil {
		t.Skip("ffmpeg not installed")
	}
	video := filepath.Join(t.TempDir(), "v.mp4")
	if out, err := exec.Command(ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=10:duration=4",
		"-pix_fmt", "yuv420p", video).CombinedOutput(); err != nil {
		t.Fatalf("make video: %v %s", err, out)
	}
	var referer string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		referer = r.Header.Get("Referer")
		http.ServeFile(w, r, video)
	}))
	defer up.Close()
	id := AddRemote(up.URL+"/v.mp4", map[string]string{"User-Agent": "KMB", "Referer": "https://site/"})
	h := Middleware(func() string { return ff }, http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/kmb/rframe?id="+id+"&t=2&w=160", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "\xff\xd8") {
		t.Fatalf("frame: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if referer != "https://site/" {
		t.Errorf("the site's headers must reach the server, Referer = %q", referer)
	}
}
