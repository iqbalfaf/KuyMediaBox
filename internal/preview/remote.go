package preview

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Online videos in the trim dialog: the stream is proxied (/kmb/remote) so the player can
// send the site's headers, and frames are cut from it with FFmpeg (/kmb/rframe) when the
// player can't play it.

// Remote is an online stream registered for previews.
type Remote struct {
	URL     string
	Headers map[string]string

	added time.Time
	busy  sync.Mutex   // one FFmpeg per stream
	seq   atomic.Int64 // newest frame request
	last  []byte       // last frame made, for requests that were overtaken
}

var remotes = struct {
	sync.Mutex
	m map[string]*Remote
}{m: map[string]*Remote{}}

const maxRemotes = 16

// AddRemote registers a stream and returns its id.
func AddRemote(url string, headers map[string]string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	remotes.Lock()
	defer remotes.Unlock()
	for len(remotes.m) >= maxRemotes {
		oldest := ""
		for k, r := range remotes.m {
			if oldest == "" || r.added.Before(remotes.m[oldest].added) {
				oldest = k
			}
		}
		delete(remotes.m, oldest)
	}
	remotes.m[id] = &Remote{URL: url, Headers: headers, added: time.Now()}
	return id
}

func remote(id string) *Remote {
	remotes.Lock()
	defer remotes.Unlock()
	return remotes.m[id]
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

// The webview gets every response in one piece, so the player is answered in parts it
// then asks for one after the other.
const remoteChunk = 4 << 20

var reRange = regexp.MustCompile(`^bytes=(\d*)-(\d*)$`)

func serveRemote(w http.ResponseWriter, r *http.Request) {
	rm := remote(r.URL.Query().Get("id"))
	if rm == nil {
		http.NotFound(w, r)
		return
	}
	var from, to int64 = 0, -1
	if m := reRange.FindStringSubmatch(strings.TrimSpace(r.Header.Get("Range"))); m != nil {
		if m[1] == "" {
			http.Error(w, "suffix ranges not supported", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		from, _ = strconv.ParseInt(m[1], 10, 64)
		if m[2] != "" {
			to, _ = strconv.ParseInt(m[2], 10, 64)
		}
	}
	if to < 0 || to-from+1 > remoteChunk {
		to = from + remoteChunk - 1
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, rm.URL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	for k, v := range rm.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", from, to))
	resp, err := httpClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		// No range support (or an error): the player can't seek in it; the dialog falls
		// back to frames.
		http.Error(w, resp.Status, http.StatusBadGateway)
		return
	}
	for _, h := range []string{"Content-Type", "Content-Range", "Content-Length", "Last-Modified"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusPartialContent)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, remoteChunk))
}

// ffmpegHeaders formats headers for ffmpeg's -headers option.
func ffmpegHeaders(h map[string]string) string {
	var b strings.Builder
	for k, v := range h {
		if strings.EqualFold(k, "User-Agent") {
			continue // given with -user_agent
		}
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}
	return b.String()
}

func serveRemoteFrame(w http.ResponseWriter, r *http.Request, ff string) {
	if ff == "" {
		http.Error(w, "ffmpeg missing", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	rm := remote(q.Get("id"))
	if rm == nil {
		http.NotFound(w, r)
		return
	}
	my := rm.seq.Add(1)
	rm.busy.Lock()
	defer rm.busy.Unlock()
	// A request overtaken by a newer one while it waited gets the last frame made instead
	// of running FFmpeg for a position the user has already left.
	out := rm.last
	if my == rm.seq.Load() || out == nil {
		width := max(64, min(int(num(q.Get("w"), 640)), 1280))
		t := strconv.FormatFloat(num(q.Get("t"), 0), 'f', 3, 64)
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
		if ua := rm.Headers["User-Agent"]; ua != "" {
			args = append(args, "-user_agent", ua)
		}
		if h := ffmpegHeaders(rm.Headers); h != "" {
			args = append(args, "-headers", h)
		}
		args = append(args, "-ss", t, "-i", rm.URL, "-frames:v", "1", "-vf", fmt.Sprintf("scale=%d:-2", width),
			"-f", "image2pipe", "-c:v", "mjpeg", "-q:v", "5", "-")
		frame, err := bytesOut(ctx, ff, args...)
		if err == nil && len(frame) > 0 {
			out, rm.last = frame, frame
		}
	}
	if len(out) == 0 {
		http.Error(w, "no frame", http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(out)
}
