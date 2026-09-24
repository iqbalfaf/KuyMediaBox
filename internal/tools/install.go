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
	"kuymediabox/internal/i18n"
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
		return nil, fmt.Errorf(i18n.L("GitHub menjawab %s", "GitHub replied %s"), resp.Status)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// gallery-dl publishes its Windows builds on Codeberg (the GitHub repository has no assets).
const galleryDLReleases = "https://codeberg.org/api/v1/repos/mikf/gallery-dl/releases/latest"

// latestCodeberg reads the newest release of a Codeberg (Forgejo) repository.
func latestCodeberg(ctx context.Context, apiURL string) (*ghRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(i18n.L("Codeberg menjawab %s", "Codeberg replied %s"), resp.Status)
	}
	var rel ghRelease // Forgejo uses the same field names as GitHub
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// CheckUpdates asks GitHub/Codeberg for the newest yt-dlp, spotDL and gallery-dl versions.
func (m *Manager) CheckUpdates(ctx context.Context) {
	for id, repo := range map[string]string{YtDlp: "yt-dlp/yt-dlp", SpotDL: "spotDL/spotify-downloader", GalleryDL: ""} {
		var rel *ghRelease
		var err error
		if id == GalleryDL {
			rel, err = latestCodeberg(ctx, galleryDLReleases)
		} else {
			rel, err = latestRelease(ctx, repo)
		}
		if err != nil {
			continue
		}
		latest := strings.TrimPrefix(rel.TagName, "v")
		m.update(id, func(s *Status) {
			s.Latest = latest
			s.UpdateAvailable = s.Found && newer(latest, s.Version)
		})
	}
	// LibreOffice: only a copy we extracted ourselves is offered for updating.
	if latest, err := latestLibreOffice(ctx); err == nil {
		m.update(LibreOffice, func(s *Status) {
			s.Latest = latest
			s.UpdateAvailable = s.Found && s.Source == "downloaded" && newer(latest, s.Version)
		})
	}
}

// Install downloads (or re-downloads, for updates) a tool into the app tools folder.
func (m *Manager) Install(ctx context.Context, id string) error {
	m.mu.Lock()
	s := m.statuses[id]
	if s == nil {
		m.mu.Unlock()
		return errors.New(i18n.L("tool tidak dikenal", "unknown tool"))
	}
	if s.Busy {
		m.mu.Unlock()
		return errors.New(i18n.L("sedang diproses", "already in progress"))
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
		return errors.New(i18n.L("terpasang, tapi tidak bisa dijalankan", "installed, but it can't be run"))
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
			return fmt.Errorf(i18n.L("tidak bisa membaca rilis spotDL: %w", "can't read the spotDL release: %w"), err)
		}
		for _, a := range rel.Assets {
			if strings.HasSuffix(strings.ToLower(a.Name), "win32.exe") {
				return downloadFile(ctx, a.URL, filepath.Join(dir, "spotdl.exe"), progress)
			}
		}
		return errors.New(i18n.L("file spotDL untuk Windows tidak ditemukan di rilis terbaru", "spotDL for Windows not found in the latest release"))
	case LibreOffice:
		return installLibreOffice(ctx, progress)
	case GalleryDL:
		rel, err := latestCodeberg(ctx, galleryDLReleases)
		if err != nil {
			return fmt.Errorf(i18n.L("tidak bisa membaca rilis gallery-dl: %w", "can't read the gallery-dl release: %w"), err)
		}
		for _, a := range rel.Assets {
			if strings.EqualFold(a.Name, "gallery-dl.exe") {
				return downloadFile(ctx, a.URL, filepath.Join(dir, "gallery-dl.exe"), progress)
			}
		}
		return errors.New(i18n.L("file gallery-dl untuk Windows tidak ditemukan di rilis terbaru", "gallery-dl for Windows not found in the latest release"))
	}
	return errors.New(i18n.L("tool tidak dikenal", "unknown tool"))
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
		return fmt.Errorf(i18n.L("tidak bisa memasang file (sedang dipakai?): %w", "can't install the file (in use?): %w"), err)
	}
	return nil
}

// fetchAttempts is how often an interrupted download is resumed before giving up.
var fetchAttempts = 8

// fetch downloads url to dest. When the connection drops it resumes where it stopped
// (HTTP Range), which matters for big tools on slow or flaky connections.
func fetch(ctx context.Context, url, dest string, progress func(float64)) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	var (
		done    int64
		total   int64 = -1
		lastErr error
		last    = time.Now()
	)
	for attempt := 0; attempt < fetchAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				f.Close()
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			f.Close()
			return err
		}
		req.Header.Set("User-Agent", userAgent)
		if done > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", done))
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				f.Close()
				return ctx.Err()
			}
			lastErr = fmt.Errorf(i18n.L("gagal mengunduh, periksa koneksi internet: %w", "download failed, check your internet connection: %w"), err)
			continue
		}
		switch {
		case resp.StatusCode == http.StatusPartialContent && done > 0:
			if total < 0 && resp.ContentLength >= 0 {
				total = done + resp.ContentLength
			}
		case resp.StatusCode == http.StatusOK:
			// Fresh start (first attempt, or the server ignored the Range header).
			if done > 0 {
				if _, err := f.Seek(0, io.SeekStart); err != nil {
					resp.Body.Close()
					f.Close()
					return err
				}
				if err := f.Truncate(0); err != nil {
					resp.Body.Close()
					f.Close()
					return err
				}
				done = 0
			}
			total = resp.ContentLength
		default:
			resp.Body.Close()
			f.Close()
			return fmt.Errorf(i18n.L("gagal mengunduh (%s)", "download failed (%s)"), resp.Status)
		}
		buf := make([]byte, 256*1024)
		var rerr error
		for {
			var n int
			n, rerr = resp.Body.Read(buf)
			if n > 0 {
				if _, werr := f.Write(buf[:n]); werr != nil {
					resp.Body.Close()
					f.Close()
					return werr
				}
				done += int64(n)
				if total > 0 && time.Since(last) > 200*time.Millisecond {
					progress(float64(done) / float64(total) * 0.95)
					last = time.Now()
				}
			}
			if rerr != nil {
				break
			}
		}
		resp.Body.Close()
		if ctx.Err() != nil {
			f.Close()
			return ctx.Err()
		}
		if rerr == io.EOF && (total < 0 || done == total) {
			return f.Close()
		}
		if rerr == io.EOF {
			lastErr = errors.New(i18n.L("unduhan tidak lengkap", "incomplete download"))
		} else {
			lastErr = fmt.Errorf(i18n.L("unduhan terputus: %w", "download interrupted: %w"), rerr)
		}
	}
	f.Close()
	return lastErr
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
		return fmt.Errorf(i18n.L("arsip rusak: %w", "damaged archive: %w"), err)
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
			return fmt.Errorf(i18n.L("%s tidak ada di dalam arsip", "%s is missing from the archive"), base)
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
		return fmt.Errorf(i18n.L("tidak bisa memasang %s (sedang dipakai?): %w", "can't install %s (in use?): %w"), filepath.Base(dest), err)
	}
	return nil
}
