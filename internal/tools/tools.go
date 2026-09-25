// Package tools finds, installs and updates the external programs KuyMediaBox relies on:
// FFmpeg/ffprobe, yt-dlp, a JavaScript runtime for yt-dlp (Deno or Node.js) and spotDL.
package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/config"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
)

// Tool IDs.
const (
	FFmpeg      = "ffmpeg"
	FFprobe     = "ffprobe"
	YtDlp       = "ytdlp"
	JSRuntime   = "jsruntime"
	SpotDL      = "spotdl"
	GalleryDL   = "gallerydl"
	LibreOffice = "libreoffice"
	VeraPDF     = "verapdf"
)

// Status is shown on the Settings page.
type Status struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Found           bool    `json:"found"`
	Path            string  `json:"path"`
	Version         string  `json:"version"`
	Source          string  `json:"source"`  // custom | downloaded | bundled | system
	Runtime         string  `json:"runtime"` // deno | node (JS runtime only)
	Latest          string  `json:"latest"`
	UpdateAvailable bool    `json:"updateAvailable"`
	Busy            bool    `json:"busy"`
	Progress        float64 `json:"progress"`
	Error           string  `json:"error"`
	Required        string  `json:"required"` // what needs it, for the UI
	Optional        bool    `json:"optional"` // nice to have; missing is not a warning
}

type meta struct {
	name     string
	desc     [2]string // Indonesian, English
	required [2]string
	exes     []string // candidate executable names in priority order
}

var metas = map[string]meta{
	FFmpeg:      {"FFmpeg", [2]string{"Mesin konversi video, audio & gambar", "Video, audio & image conversion engine"}, [2]string{"Video, Audio, Download", "Video, Audio, Download"}, []string{"ffmpeg.exe"}},
	YtDlp:       {"yt-dlp", [2]string{"Download video & audio dari YouTube, TikTok, Instagram & Facebook", "Downloads video & audio from YouTube, TikTok, Instagram & Facebook"}, [2]string{"semua Download", "all downloads"}, []string{"yt-dlp.exe"}},
	JSRuntime:   {"JS runtime", [2]string{"Dibutuhkan yt-dlp untuk YouTube", "Needed by yt-dlp for YouTube"}, [2]string{"Download YouTube", "YouTube downloads"}, []string{"deno.exe", "node.exe"}},
	SpotDL:      {"spotDL", [2]string{"Membaca playlist, album & lagu Spotify", "Reads Spotify playlists, albums & tracks"}, [2]string{"Download Spotify", "Spotify downloads"}, []string{"spotdl.exe"}},
	GalleryDL:   {"gallery-dl", [2]string{"Membaca foto dari post TikTok & Facebook", "Reads pictures from TikTok & Facebook posts"}, [2]string{"foto TikTok & Facebook", "TikTok & Facebook pictures"}, []string{"gallery-dl.exe"}},
	LibreOffice: {"LibreOffice", [2]string{"Opsional: konversi Word, Excel & PowerPoint ↔ PDF bila Microsoft Office tidak terpasang (±375 MB)", "Optional: Word, Excel & PowerPoint ↔ PDF when Microsoft Office isn't installed (±375 MB)"}, [2]string{"konversi dokumen Office di menu PDF", "Office document conversion in the PDF tools"}, []string{"soffice.com"}},
	VeraPDF:     {"veraPDF", [2]string{"Opsional: validasi resmi PDF/A (Java dipasang otomatis bila belum ada, ±130 MB)", "Optional: official PDF/A validation (Java is installed too when missing, ±130 MB)"}, [2]string{"Validasi PDF/A di menu PDF", "PDF/A validation in the PDF tools"}, []string{"verapdf.bat"}},
}

// Order is the display order.
var Order = []string{FFmpeg, YtDlp, JSRuntime, SpotDL, GalleryDL, LibreOffice, VeraPDF}

// optional tools are not reported as missing.
var optional = map[string]bool{LibreOffice: true, VeraPDF: true}

// Manager keeps the current status of every tool.
type Manager struct {
	mu       sync.Mutex
	cfg      *config.Store
	statuses map[string]*Status
	ffprobe  string
	onChange func([]Status)
}

// New creates a manager; onChange receives the full list after every change.
func New(cfg *config.Store, onChange func([]Status)) *Manager {
	m := &Manager{cfg: cfg, statuses: map[string]*Status{}, onChange: onChange}
	for _, id := range Order {
		md := metas[id]
		m.statuses[id] = &Status{ID: id, Name: md.name}
	}
	return m
}

// List returns a snapshot of all statuses in display order.
func (m *Manager) List() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Status, 0, len(Order))
	for _, id := range Order {
		st := *m.statuses[id]
		md := metas[id]
		// Resolved on every call so a language switch shows up without a restart.
		st.Description = i18n.L(md.desc[0], md.desc[1])
		st.Required = i18n.L(md.required[0], md.required[1])
		st.Optional = optional[id]
		out = append(out, st)
	}
	return out
}

func (m *Manager) changed() {
	if m.onChange != nil {
		m.onChange(m.List())
	}
}

func (m *Manager) update(id string, fn func(s *Status)) {
	m.mu.Lock()
	if s := m.statuses[id]; s != nil {
		fn(s)
	}
	m.mu.Unlock()
	m.changed()
}

// Path returns the executable for a tool, or "" when missing.
func (m *Manager) Path(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id == FFprobe {
		return m.ffprobe
	}
	if s := m.statuses[id]; s != nil && s.Found {
		return s.Path
	}
	return ""
}

// JSRuntimeKind returns "deno" or "node" for the detected runtime.
func (m *Manager) JSRuntimeKind() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statuses[JSRuntime].Runtime
}

// candidateDirs lists folders to search, highest priority first.
func candidateDirs() []struct{ dir, source string } {
	return []struct{ dir, source string }{
		{appdir.ToolsDir(), "downloaded"},
		{filepath.Join(appdir.ExeDir(), "bin"), "bundled"},
		{appdir.ExeDir(), "bundled"},
	}
}

// libreDir is where the app extracts its own LibreOffice.
func libreDir() string { return filepath.Join(appdir.ToolsDir(), "libreoffice") }

func libreCandidates() []struct{ path, source string } {
	var out []struct{ path, source string }
	add := func(p, source string) { out = append(out, struct{ path, source string }{p, source}) }
	own := libreDir()
	add(filepath.Join(own, "program", "soffice.com"), "downloaded")
	add(filepath.Join(own, "LibreOffice", "program", "soffice.com"), "downloaded")
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432"} {
		if base := os.Getenv(env); base != "" {
			add(filepath.Join(base, "LibreOffice", "program", "soffice.com"), "system")
		}
	}
	return out
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// locate finds a tool: custom path → app tools folder → bundled bin → PATH.
func (m *Manager) locate(id string) (path, source string) {
	if custom := m.cfg.Get().ToolPaths[id]; custom != "" && fileExists(custom) {
		return custom, "custom"
	}
	if id == VeraPDF {
		for _, p := range veraCandidates() {
			if fileExists(p.path) {
				return p.path, p.source
			}
		}
	}
	if id == LibreOffice {
		// Our own extracted copy first, then a normal installation.
		for _, p := range libreCandidates() {
			if fileExists(p.path) {
				return p.path, p.source
			}
		}
	}
	for _, exe := range metas[id].exes {
		for _, c := range candidateDirs() {
			p := filepath.Join(c.dir, exe)
			if fileExists(p) {
				return p, c.source
			}
		}
		if p, err := exec.LookPath(strings.TrimSuffix(exe, ".exe")); err == nil {
			if abs, err := filepath.Abs(p); err == nil {
				p = abs
			}
			return p, "system"
		}
	}
	return "", ""
}

// Detect locates every tool and reads its version (in parallel).
func (m *Manager) Detect(ctx context.Context) []Status {
	var wg sync.WaitGroup
	for _, id := range Order {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			m.detectOne(ctx, id)
		}(id)
	}
	wg.Wait()
	m.changed()
	return m.List()
}

func (m *Manager) detectOne(ctx context.Context, id string) {
	path, source := m.locate(id)
	st := Status{Found: path != "", Path: path, Source: source}
	if path != "" {
		if id == JSRuntime {
			st.Runtime = "node"
			if strings.EqualFold(filepath.Base(path), "deno.exe") || strings.EqualFold(filepath.Base(path), "deno") {
				st.Runtime = "deno"
			}
		}
		v, err := readVersion(ctx, id, path)
		if err != nil {
			st.Found = false
			st.Error = i18n.L("Tidak bisa dijalankan: ", "Can't be run: ") + firstLine(err.Error())
		} else {
			st.Version = v
		}
	}
	var probe string
	if id == FFmpeg && st.Found {
		cand := filepath.Join(filepath.Dir(path), "ffprobe.exe")
		if fileExists(cand) {
			probe = cand
		} else if p, err := exec.LookPath("ffprobe"); err == nil {
			probe = p
		}
		if probe == "" {
			st.Found = false
			st.Error = i18n.L("ffprobe.exe tidak ditemukan di samping ffmpeg.exe", "ffprobe.exe not found next to ffmpeg.exe")
		}
	}
	m.mu.Lock()
	s := m.statuses[id]
	s.Found, s.Path, s.Source, s.Version, s.Runtime, s.Error = st.Found, st.Path, st.Source, st.Version, st.Runtime, st.Error
	if s.Latest != "" {
		s.UpdateAvailable = s.Found && newer(s.Latest, s.Version)
	}
	if id == FFmpeg {
		m.ffprobe = probe
	}
	m.mu.Unlock()
}

var reVersion = regexp.MustCompile(`\d+(\.\d+)+`)

func readVersion(ctx context.Context, id, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	switch id {
	case FFmpeg:
		out, err := proc.Output(ctx, path, "-hide_banner", "-version")
		if err != nil {
			return "", err
		}
		// "ffmpeg version 7.1-essentials_build-www.gyan.dev Copyright ..."
		f := strings.Fields(firstLine(out))
		if len(f) >= 3 {
			v := f[2]
			if m := reVersion.FindString(v); m != "" && strings.HasPrefix(v, m) {
				return m, nil
			}
			if len(v) > 18 {
				v = v[:18]
			}
			return v, nil
		}
		return "?", nil
	case YtDlp:
		out, err := proc.Output(ctx, path, "--version")
		return firstLine(out), err
	case JSRuntime:
		out, err := proc.Output(ctx, path, "--version")
		if err != nil {
			return "", err
		}
		return reVersion.FindString(firstLine(out)), nil
	case LibreOffice:
		// "LibreOffice 26.8.0.2 <hash>"
		out, err := proc.Output(ctx, path, "--version")
		if err != nil {
			return "", err
		}
		if v := reVersion.FindString(out); v != "" {
			return v, nil
		}
		return firstLine(out), nil
	case VeraPDF:
		return veraVersion(path)
	case SpotDL, GalleryDL:
		out, err := proc.Output(ctx, path, "--version")
		if err != nil {
			return "", err
		}
		if v := reVersion.FindString(out); v != "" {
			return v, nil
		}
		return firstLine(out), nil
	}
	return "", nil
}

// SetCustomPath stores a user-chosen executable (empty clears it) and re-detects the tool.
func (m *Manager) SetCustomPath(ctx context.Context, id, path string) error {
	if err := m.cfg.SetToolPath(id, path); err != nil {
		return err
	}
	m.detectOne(ctx, id)
	m.changed()
	return nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// newer reports whether version a is greater than b (numeric, dot separated, "v" ignored).
func newer(a, b string) bool {
	pa := parts(a)
	pb := parts(b)
	if len(pa) == 0 || len(pb) == 0 {
		return false
	}
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func parts(v string) []int {
	v = reVersion.FindString(v)
	if v == "" {
		return nil
	}
	var out []int
	for _, p := range strings.Split(v, ".") {
		n := 0
		for _, c := range p {
			n = n*10 + int(c-'0')
		}
		out = append(out, n)
	}
	return out
}
