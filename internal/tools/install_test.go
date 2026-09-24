package tools

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// flakyServer serves data but drops the connection after cut bytes on the first
// `drops` requests; it honours "Range: bytes=N-" like a real CDN.
func flakyServer(t *testing.T, data []byte, cut int, drops int32, ranges bool) (*httptest.Server, *int32) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		start := 0
		if rg := r.Header.Get("Range"); ranges && strings.HasPrefix(rg, "bytes=") {
			start, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rg, "bytes="), "-"))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(data)-1, len(data)))
			w.Header().Set("Content-Length", strconv.Itoa(len(data)-start))
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.WriteHeader(http.StatusOK)
		}
		body := data[start:]
		if n <= drops && len(body) > cut {
			w.Write(body[:cut])
			w.(http.Flusher).Flush()
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("no hijacker")
			}
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestFetchResumesAfterDrop(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789abcdef"), 64*1024) // 1 MiB
	for _, ranges := range []bool{true, false} {
		srv, calls := flakyServer(t, data, 300*1024, 2, ranges)
		dest := filepath.Join(t.TempDir(), "tool.exe")
		if err := fetch(context.Background(), srv.URL, dest, func(float64) {}); err != nil {
			t.Fatalf("ranges=%v: %v", ranges, err)
		}
		got, _ := os.ReadFile(dest)
		if !bytes.Equal(got, data) {
			t.Fatalf("ranges=%v: got %d bytes, want %d identical bytes", ranges, len(got), len(data))
		}
		if atomic.LoadInt32(calls) != 3 {
			t.Fatalf("ranges=%v: %d requests, want 3", ranges, *calls)
		}
	}
}

func TestFetchGivesUp(t *testing.T) {
	old := fetchAttempts
	fetchAttempts = 2
	defer func() { fetchAttempts = old }()
	data := bytes.Repeat([]byte("x"), 200*1024)
	srv, _ := flakyServer(t, data, 1000, 100, true)
	err := fetch(context.Background(), srv.URL, filepath.Join(t.TempDir(), "x"), func(float64) {})
	if err == nil {
		t.Fatal("expected an error after repeated drops")
	}
}
