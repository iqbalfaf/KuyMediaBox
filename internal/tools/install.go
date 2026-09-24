package tools

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kuymediabox/internal/appdir"
)

var httpClient = &http.Client{Timeout: 0} // downloads can be long; contexts cancel them

const userAgent = "KuyMediaBox/0.1 (+https://github.com)"

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func latestRelease(ctx context.Context, repo string) (*ghRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub menjawab %s", resp.Status)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// CheckUpdates asks GitHub for the newest yt-dlp and spotDL versions.
func (m *Manager) CheckUpdates(ctx context.Context) {
	for id, repo := range map[string]string{YtDlp: "yt-dlp/yt-dlp", SpotDL: "spotDL/spotify-downloader"} {
		rel, err := latestRelease(ctx, repo)
		if err != nil {
			continue
		}
		latest := strings.TrimPrefix(rel.TagName, "v")
		m.update(id, func(s *Status) {
			s.Latest = latest
			s.UpdateAvailable = s.Found && newer(latest, s.Version)
		})
	}
}

// Install downloads (or re-downloads, for updates) a tool into the app tools folder.
func (m *Manager) Install(ctx context.Context, id string) error {
	m.mu.Lock()
	s := m.statuses[id]
	if s == nil {
		m.mu.Unlock()
		return errors.New("tool tidak dikenal")
	}
	if s.Busy {
		m.mu.Unlock()
		return errors.New("sedang diproses")
	}
	s.Busy, s.Progress, s.Error = true, 0, ""
	m.mu.Unlock()
	m.changed()

	err := m.install(ctx, id)

	m.update(id, func(s *Status) {
		s.Busy = false
		s.Progress = 0
		if err != nil {
			s.Error = err.Error()
		}
	})
	if err != nil {
		return err
	}
	m.detectOne(ctx, id)
	m.changed()
	m.mu.Lock()
	found := m.statuses[id].Found
	m.mu.Unlock()
	if !found {
		return errors.New("terpasang, tapi tidak bisa dijalankan")
	}
	return nil
}

func (m *Manager) install(ctx context.Context, id string) error {
	dir := appdir.ToolsDir()
	progress := func(p float64) { m.update(id, func(s *Status) { s.Progress = p }) }
	switch id {
	case FFmpeg:
		sources := []string{
			"https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip",
			"https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip",
		}
		var lastErr error
		for _, url := range sources {
			lastErr = downloadZip(ctx, url, dir, map[string]string{"ffmpeg.exe": "ffmpeg.exe", "ffprobe.exe": "ffprobe.exe"}, progress)
			if lastErr == nil || ctx.Err() != nil {
				return lastErr
			}
		}
		return lastErr
	case YtDlp:
		return downloadFile(ctx, "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe", filepath.Join(dir, "yt-dlp.exe"), progress)
	case JSRuntime:
		return downloadZip(ctx, "https://github.com/denoland/deno/releases/latest/download/deno-x86_64-pc-windows-msvc.zip", dir, map[string]string{"deno.exe": "deno.exe"}, progress)
	case SpotDL:
		rel, err := latestRelease(ctx, "spotDL/spotify-downloader")
		if err != nil {
			return fmt.Errorf("tidak bisa membaca rilis spotDL: %w", err)
		}
		for _, a := range rel.Assets {
			if strings.HasSuffix(strings.ToLower(a.Name), "win32.exe") {
				return downloadFile(ctx, a.URL, filepath.Join(dir, "spotdl.exe"), progress)
			}
		}
		return errors.New("file spotDL untuk Windows tidak ditemukan di rilis terbaru")
	}
	return errors.New("tool tidak dikenal")
}

// downloadFile saves url to dest atomically, reporting 0..1 progress.
func downloadFile(ctx context.Context, url, dest string, progress func(float64)) error {
	tmp := dest + ".download"
	if err := fetch(ctx, url, tmp, progress); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("tidak bisa memasang file (sedang dipakai?): %w", err)
	}
	return nil
}

func fetch(ctx context.Context, url, dest string, progress func(float64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("gagal mengunduh, periksa koneksi internet: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gagal mengunduh (%s)", resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 256*1024)
	last := time.Now()
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
			done += int64(n)
			if total > 0 && time.Since(last) > 200*time.Millisecond {
				progress(float64(done) / float64(total) * 0.95)
				last = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("unduhan terputus: %w", rerr)
		}
	}
	if total > 0 && done != total {
		f.Close()
		return errors.New("unduhan tidak lengkap")
	}
	return f.Close()
}

// downloadZip fetches a zip and extracts the named files (matched by base name) into dir.
func downloadZip(ctx context.Context, url, dir string, want map[string]string, progress func(float64)) error {
	tmp, err := os.CreateTemp(appdir.TempDir(), "tool-*.zip")
	if err != nil {
		return err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)
	if err := fetch(ctx, url, name, progress); err != nil {
		return err
	}
	zr, err := zip.OpenReader(name)
	if err != nil {
		return fmt.Errorf("arsip rusak: %w", err)
	}
	defer zr.Close()
	found := map[string]bool{}
	for _, f := range zr.File {
		base := strings.ToLower(filepath.Base(f.Name))
		target, ok := want[base]
		if !ok || found[base] || f.FileInfo().IsDir() {
			continue
		}
		if err := extract(f, filepath.Join(dir, target)); err != nil {
			return err
		}
		found[base] = true
	}
	for base := range want {
		if !found[base] {
			return fmt.Errorf("%s tidak ada di dalam arsip", base)
		}
	}
	progress(1)
	return nil
}

func extract(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	tmp := dest + ".download"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	_ = os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("tidak bisa memasang %s (sedang dipakai?): %w", filepath.Base(dest), err)
	}
	return nil
}
