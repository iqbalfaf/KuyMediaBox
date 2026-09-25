// Package config stores user settings as JSON in %APPDATA%\KuyMediaBox\settings.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"kuymediabox/internal/appdir"
)

// Output modes for a module's results.
const (
	OutputDefault   = "default"   // the module's default folder (e.g. Videos\KuyMediaBox)
	OutputSubfolder = "subfolder" // dynamic: <source folder>\converted
	OutputSame      = "same"      // dynamic: next to the source file
	OutputCustom    = "custom"    // a folder the user picked
)

// Conflict policies when the output name already exists.
const (
	ConflictRename    = "rename"
	ConflictSkip      = "skip"
	ConflictOverwrite = "overwrite"
)

// OutputKinds are the modules that have their own result folder.
var OutputKinds = []string{"image", "video", "audio", "download", "pdf"}

// Output is where one module saves its results.
type Output struct {
	Mode string `json:"mode"`
	Dir  string `json:"dir"` // used when Mode == custom
}

// UI languages.
const (
	LangID = "id"
	LangEN = "en"
)

// UI themes.
const (
	ThemeDark   = "dark"
	ThemeLight  = "light"
	ThemeSystem = "system"
)

// DefaultParallel is how many tasks of each kind run at the same time by default.
var DefaultParallel = map[string]int{"image": 3, "video": 1, "audio": 2, "download": 2, "pdf": 2}

// MaxParallel caps the parallel tasks per kind.
const MaxParallel = 8

// CookieBrowsers are the browsers yt-dlp and gallery-dl can read cookies from.
var CookieBrowsers = []string{"chrome", "edge", "firefox", "brave", "opera", "vivaldi", "chromium"}

// WatchRule makes the app convert new files that appear in a folder.
type WatchRule struct {
	ID      string          `json:"id"`
	Dir     string          `json:"dir"`
	Kind    string          `json:"kind"`    // image | video | audio
	Options json.RawMessage `json:"options"` // the module's settings when the rule was saved
	Enabled bool            `json:"enabled"`
}

// Settings are the user's global preferences.
type Settings struct {
	Outputs            map[string]Output `json:"outputs"`
	DownloadSubfolders bool              `json:"downloadSubfolders"`
	Suffix             string            `json:"suffix"`
	Conflict           string            `json:"conflict"`
	Notify             bool              `json:"notify"`
	SkipDownloaded     bool              `json:"skipDownloaded"`
	AutoUpdate         bool              `json:"autoUpdate"`
	Language           string            `json:"language"` // id | en
	Theme              string            `json:"theme"`    // dark | light | system
	Parallel           map[string]int    `json:"parallel"` // tasks at once per kind
	CookiesBrowser     string            `json:"cookiesBrowser"`
	CookiesFile        string            `json:"cookiesFile"`
	NameTemplate       string            `json:"nameTemplate"`    // YouTube & other sites, "" = built-in
	SpotifyTemplate    string            `json:"spotifyTemplate"` // Spotify, "" = built-in
	SpotifyLogin       bool              `json:"spotifyLogin"`    // spotDL --user-auth (private playlists)
	ClipboardWatch     bool              `json:"clipboardWatch"`
	DownloadLimitKB    int               `json:"downloadLimitKB"` // total download speed limit, 0 = none
	Watch              []WatchRule       `json:"watch"`
	ToolPaths          map[string]string `json:"toolPaths"`
}

// Defaults returns fresh default settings.
func Defaults() Settings {
	s := Settings{
		Outputs:            map[string]Output{},
		DownloadSubfolders: true,
		Suffix:             "_converted",
		Conflict:           ConflictRename,
		Notify:             true,
		SkipDownloaded:     true,
		AutoUpdate:         true,
		Language:           LangID,
		Theme:              ThemeDark,
		Parallel:           map[string]int{},
		Watch:              []WatchRule{},
		ToolPaths:          map[string]string{},
	}
	for k, v := range DefaultParallel {
		s.Parallel[k] = v
	}
	for _, k := range OutputKinds {
		s.Outputs[k] = Output{Mode: OutputDefault}
	}
	return s
}

// Normalize fixes invalid or empty values.
func (s *Settings) Normalize() {
	d := Defaults()
	if s.Outputs == nil {
		s.Outputs = map[string]Output{}
	}
	for _, k := range OutputKinds {
		o := s.Outputs[k]
		o.Dir = strings.TrimSpace(o.Dir)
		switch o.Mode {
		case OutputDefault:
		case OutputSubfolder, OutputSame:
			if k == "download" { // downloads have no source file to sit next to
				o.Mode = OutputDefault
			}
		case OutputCustom:
			if o.Dir == "" {
				o.Mode = OutputDefault
			}
		default:
			o.Mode = OutputDefault
		}
		if o.Mode != OutputCustom {
			o.Dir = ""
		}
		s.Outputs[k] = o
	}
	for k := range s.Outputs {
		if !isKind(k) {
			delete(s.Outputs, k)
		}
	}
	switch s.Conflict {
	case ConflictRename, ConflictSkip, ConflictOverwrite:
	default:
		s.Conflict = d.Conflict
	}
	s.Suffix = SanitizeSuffix(s.Suffix)
	if s.Language != LangEN {
		s.Language = LangID
	}
	switch s.Theme {
	case ThemeLight, ThemeSystem:
	default:
		s.Theme = ThemeDark
	}
	par := map[string]int{}
	for k, v := range DefaultParallel {
		n := s.Parallel[k]
		if n < 1 {
			n = v
		}
		par[k] = min(n, MaxParallel)
	}
	s.Parallel = par
	if !slices.Contains(CookieBrowsers, s.CookiesBrowser) {
		s.CookiesBrowser = ""
	}
	s.CookiesFile = strings.TrimSpace(s.CookiesFile)
	s.DownloadLimitKB = max(0, min(s.DownloadLimitKB, 1_000_000))
	s.NameTemplate = cleanTemplate(s.NameTemplate)
	s.SpotifyTemplate = cleanTemplate(s.SpotifyTemplate)
	rules := []WatchRule{}
	for _, r := range s.Watch {
		r.Dir = strings.TrimSpace(r.Dir)
		if r.Dir == "" || (r.Kind != "image" && r.Kind != "video" && r.Kind != "audio") || r.ID == "" {
			continue
		}
		if len(r.Options) == 0 || !json.Valid(r.Options) {
			r.Options = json.RawMessage("{}")
		}
		rules = append(rules, r)
	}
	s.Watch = rules
	if s.ToolPaths == nil {
		s.ToolPaths = map[string]string{}
	}
}

// cleanTemplate trims a file name template and drops characters Windows forbids (the path
// separators too: a template names a file, not a folder).
func cleanTemplate(t string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(t) {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimRight(b.String(), ". ")
	if len([]rune(out)) > 120 {
		out = string([]rune(out)[:120])
	}
	return out
}

func isKind(k string) bool {
	for _, x := range OutputKinds {
		if x == k {
			return true
		}
	}
	return false
}

// SanitizeSuffix removes characters Windows does not allow in file names.
func SanitizeSuffix(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimRight(b.String(), ". ")
	if len([]rune(out)) > 40 {
		out = string([]rune(out)[:40])
	}
	return out
}

// Store loads and saves settings safely across goroutines.
type Store struct {
	mu   sync.RWMutex
	path string
	s    Settings
}

// Load reads settings from disk, falling back to defaults.
func Load() *Store {
	return LoadFrom(filepath.Join(appdir.ConfigDir(), "settings.json"))
}

// LoadFrom reads settings from a specific file (used by tests).
func LoadFrom(path string) *Store {
	st := &Store{path: path, s: Defaults()}
	if data, err := os.ReadFile(st.path); err == nil {
		st.s = parse(data)
	}
	st.s.Normalize()
	return st
}

func parse(data []byte) Settings {
	s := Defaults()
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return s
	}
	var parsed Settings
	if json.Unmarshal(data, &parsed) != nil {
		return s
	}
	// Keep defaults for anything the file does not mention (older versions).
	if _, ok := raw["outputs"]; ok {
		for k, v := range parsed.Outputs {
			s.Outputs[k] = v
		}
	}
	// Older builds stored a single download folder.
	var legacy struct {
		DownloadDir string `json:"downloadDir"`
	}
	if _, ok := raw["outputs"]; !ok && json.Unmarshal(data, &legacy) == nil && legacy.DownloadDir != "" &&
		!strings.EqualFold(filepath.Clean(legacy.DownloadDir), filepath.Clean(appdir.DefaultOutputDir("download"))) {
		s.Outputs["download"] = Output{Mode: OutputCustom, Dir: legacy.DownloadDir}
	}
	if _, ok := raw["downloadSubfolders"]; ok {
		s.DownloadSubfolders = parsed.DownloadSubfolders
	}
	if _, ok := raw["suffix"]; ok {
		s.Suffix = parsed.Suffix
	}
	if _, ok := raw["conflict"]; ok {
		s.Conflict = parsed.Conflict
	}
	if _, ok := raw["notify"]; ok {
		s.Notify = parsed.Notify
	}
	if _, ok := raw["skipDownloaded"]; ok {
		s.SkipDownloaded = parsed.SkipDownloaded
	}
	if _, ok := raw["autoUpdate"]; ok {
		s.AutoUpdate = parsed.AutoUpdate
	}
	if _, ok := raw["language"]; ok {
		s.Language = parsed.Language
	}
	if _, ok := raw["theme"]; ok {
		s.Theme = parsed.Theme
	}
	for k, v := range parsed.Parallel {
		s.Parallel[k] = v
	}
	s.CookiesBrowser, s.CookiesFile = parsed.CookiesBrowser, parsed.CookiesFile
	s.NameTemplate, s.SpotifyTemplate = parsed.NameTemplate, parsed.SpotifyTemplate
	s.SpotifyLogin, s.ClipboardWatch = parsed.SpotifyLogin, parsed.ClipboardWatch
	s.DownloadLimitKB = parsed.DownloadLimitKB
	if parsed.Watch != nil {
		s.Watch = parsed.Watch
	}
	if parsed.ToolPaths != nil {
		s.ToolPaths = parsed.ToolPaths
	}
	return s
}

// Get returns a copy of the current settings.
func (st *Store) Get() Settings {
	st.mu.RLock()
	defer st.mu.RUnlock()
	c := st.s
	c.ToolPaths = map[string]string{}
	for k, v := range st.s.ToolPaths {
		c.ToolPaths[k] = v
	}
	c.Outputs = map[string]Output{}
	for k, v := range st.s.Outputs {
		c.Outputs[k] = v
	}
	c.Parallel = map[string]int{}
	for k, v := range st.s.Parallel {
		c.Parallel[k] = v
	}
	c.Watch = append([]WatchRule{}, st.s.Watch...)
	return c
}

// Set replaces the settings and writes them to disk.
func (st *Store) Set(s Settings) (Settings, error) {
	s.Normalize()
	st.mu.Lock()
	st.s = s
	st.mu.Unlock()
	return st.Get(), st.save()
}

// SetToolPath stores (or clears) a custom path for a tool.
func (st *Store) SetToolPath(id, path string) error {
	st.mu.Lock()
	if path == "" {
		delete(st.s.ToolPaths, id)
	} else {
		st.s.ToolPaths[id] = path
	}
	st.mu.Unlock()
	return st.save()
}

func (st *Store) save() error {
	st.mu.RLock()
	data, err := json.MarshalIndent(st.s, "", "  ")
	st.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(st.path), 0o755); err != nil {
		return err
	}
	tmp := st.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, st.path)
}
