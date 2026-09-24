// Package updater checks GitHub Releases for a newer KuyMediaBox and installs it:
// portable copies replace their own .exe, installed copies run the new installer silently.
package updater

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Repo is the GitHub repository that publishes releases.
const Repo = "iqbalfaf/KuyMediaBox"

// Install modes.
const (
	ModePortable  = "portable"  // the exe folder is writable: swap the exe in place
	ModeInstaller = "installer" // installed under Program Files: run the setup silently
)

// Info describes the result of an update check.
type Info struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	Available   bool   `json:"available"`
	Notes       string `json:"notes"`
	URL         string `json:"url"`
	PublishedAt string `json:"publishedAt"`
	Mode        string `json:"mode"`
	AssetName   string `json:"assetName"`
	AssetSize   int64  `json:"assetSize"`

	assetURL string
	sumsURL  string
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt string    `json:"published_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	Assets      []ghAsset `json:"assets"`
}

var client = &http.Client{}

// APIBase can be overridden in tests.
var APIBase = "https://api.github.com"

// Check asks GitHub for the latest release and compares it with current.
func Check(ctx context.Context, current string) (Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, APIBase+"/repos/"+Repo+"/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "KuyMediaBox/"+current)
	resp, err := client.Do(req)
	if err != nil {
		return Info{Current: current}, errors.New("tidak bisa menghubungi GitHub, periksa koneksi internet")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Info{Current: current}, errors.New("belum ada rilis yang diterbitkan")
	}
	if resp.StatusCode != http.StatusOK {
		return Info{Current: current}, fmt.Errorf("GitHub menjawab %s, coba lagi nanti", resp.Status)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Info{Current: current}, fmt.Errorf("data rilis tidak valid: %w", err)
	}
	return fromRelease(rel, current, detectMode()), nil
}

func fromRelease(rel ghRelease, current, mode string) Info {
	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	info := Info{
		Current:     current,
		Latest:      latest,
		Notes:       cleanNotes(rel.Body),
		URL:         rel.HTMLURL,
		PublishedAt: rel.PublishedAt,
		Mode:        mode,
	}
	suffix := "-portable.exe"
	if mode == ModeInstaller {
		suffix = "-setup.exe"
	}
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		switch {
		case strings.HasSuffix(name, suffix) && strings.Contains(name, "windows"):
			info.AssetName, info.AssetSize, info.assetURL = a.Name, a.Size, a.URL
		case name == "sha256sums.txt":
			info.sumsURL = a.URL
		}
	}
	info.Available = !rel.Draft && !rel.Prerelease && info.assetURL != "" && Compare(latest, current) > 0
	return info
}

// cleanNotes keeps only the "what changed" part of a release body: the "## Yang baru"
// section written by the release workflow (or GitHub's "## What's Changed"), without the
// static download instructions.
func cleanNotes(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	section := func(title string) (string, bool) {
		i := strings.Index(body, title)
		if i < 0 {
			return "", false
		}
		rest := body[i+len(title):]
		if j := strings.Index(rest, "\n## "); j >= 0 {
			rest = rest[:j]
		}
		return strings.TrimSpace(rest), true
	}
	if s, ok := section("## Yang baru"); ok {
		body = s
	} else if s, ok := section("## What's Changed"); ok {
		body = s
	} else {
		// Older releases: drop the "## Unduh" block (up to the SHA256SUMS line).
		var kept []string
		skipping := false
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "## Unduh") {
				skipping = true
				continue
			}
			if skipping {
				if strings.HasPrefix(line, "## ") {
					skipping = false
				} else {
					if strings.Contains(line, "SHA256SUMS") {
						skipping = false
					}
					continue
				}
			}
			kept = append(kept, line)
		}
		body = strings.Join(kept, "\n")
	}
	body = strings.TrimSpace(body)
	if r := []rune(body); len(r) > 4000 {
		body = string(r[:4000]) + "…"
	}
	return body
}

var reNum = regexp.MustCompile(`\d+`)

// Compare returns 1 if a > b, -1 if a < b and 0 if equal (numeric, dot separated).
func Compare(a, b string) int {
	pa := reNum.FindAllString(strings.SplitN(a, "-", 2)[0], -1)
	pb := reNum.FindAllString(strings.SplitN(b, "-", 2)[0], -1)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

func exePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return exe, nil
}

// detectMode decides how an update can be applied to this copy of the app.
func detectMode() string {
	exe, err := exePath()
	if err != nil {
		return ModeInstaller
	}
	dir := filepath.Dir(exe)
	f, err := os.CreateTemp(dir, ".kmb-write-test-*")
	if err != nil {
		return ModeInstaller
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return ModePortable
}

// Download fetches the release asset for info.Mode into dir and verifies its SHA-256.
func Download(ctx context.Context, info Info, dir string, progress func(float64)) (string, error) {
	if info.assetURL == "" {
		return "", errors.New("file update untuk Windows tidak ditemukan di rilis ini")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	want, err := expectedHash(ctx, info)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(dir, info.AssetName)
	tmp := dest + ".download"
	defer os.Remove(tmp)
	if err := fetch(ctx, info.assetURL, tmp, progress); err != nil {
		return "", err
	}
	got, err := fileHash(tmp)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(got, want) {
		return "", errors.New("file update rusak (checksum tidak cocok), coba lagi")
	}
	os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func expectedHash(ctx context.Context, info Info) (string, error) {
	if info.sumsURL == "" {
		return "", errors.New("rilis tidak menyertakan SHA256SUMS.txt, update dibatalkan demi keamanan")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, info.sumsURL, nil)
	req.Header.Set("User-Agent", "KuyMediaBox")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal mengambil checksum: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gagal mengambil checksum (%s)", resp.Status)
	}
	return parseSums(resp.Body, info.AssetName)
}

func parseSums(r io.Reader, name string) (string, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), string(rune(0xFEFF))))
		f := strings.Fields(line)
		if len(f) >= 2 && strings.EqualFold(strings.TrimPrefix(f[len(f)-1], "*"), name) && len(f[0]) == 64 {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("checksum untuk %s tidak ditemukan", name)
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fetch(ctx context.Context, url, dest string, progress func(float64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "KuyMediaBox")
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("gagal mengunduh update, periksa koneksi internet: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gagal mengunduh update (%s)", resp.Status)
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
			if total > 0 && progress != nil && time.Since(last) > 150*time.Millisecond {
				progress(float64(done) / float64(total))
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
		return errors.New("unduhan update tidak lengkap")
	}
	if progress != nil {
		progress(1)
	}
	return f.Close()
}

// Cleanup removes leftovers of a previous portable update (KuyMediaBox.exe.old).
func Cleanup() {
	exe, err := exePath()
	if err != nil {
		return
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(exe), "*.exe.old"))
	for _, m := range matches {
		_ = os.Remove(m)
	}
}
