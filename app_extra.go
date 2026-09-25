package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/config"
	"kuymediabox/internal/history"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/platform"
	"kuymediabox/internal/queue"
)

// ---- History (G-11) --------------------------------------------------------------------

// recordFinished stores a finished task in the history and remembers its output (so a
// watched folder never picks up the app's own results).
func (a *App) recordFinished(info queue.Info) {
	if info.Output != "" {
		a.producedMu.Lock()
		a.produced[strings.ToLower(filepath.Clean(info.Output))] = true
		a.producedMu.Unlock()
	}
	e := history.Entry{
		ID: info.ID + "-" + time.Now().Format("150405.000"), Time: info.Finished, Started: info.Started, Kind: info.Kind,
		Title: info.Title, Input: info.Input, Output: info.Output, InSize: info.InSize, OutSize: info.OutSize,
		Status: info.Status, Message: info.Message, Detail: info.Detail,
	}
	if err := a.hist.Add(e); err == nil {
		a.emit("history:add", e)
	}
}

// GetHistory returns the task history, newest first.
func (a *App) GetHistory() []history.Entry {
	list := a.hist.List(0)
	if list == nil {
		list = []history.Entry{}
	}
	return list
}

// RemoveHistory deletes history entries.
func (a *App) RemoveHistory(ids []string) error { return a.hist.Remove(ids) }

// ClearHistory deletes the whole history.
func (a *App) ClearHistory() error { return a.hist.Clear() }

// PathExists tells the UI whether a result is still on disk.
func (a *App) PathExists(paths []string) map[string]bool {
	out := map[string]bool{}
	for _, p := range paths {
		if p == "" {
			continue
		}
		_, err := os.Stat(p)
		out[p] = err == nil
	}
	return out
}

// ---- Parallel tasks (G-10) -------------------------------------------------------------

func (a *App) applyParallel(s config.Settings) {
	for kind, n := range s.Parallel {
		a.queue.SetLimit(kind, n)
	}
}

// ---- After the queue (G-14) ------------------------------------------------------------

// After-queue actions.
const (
	afterNone     = "none"
	afterSleep    = "sleep"
	afterShutdown = "shutdown"
)

// afterDelay is how long the user can still cancel before the computer sleeps or shuts down.
const afterDelay = 60 * time.Second

type afterState struct {
	mu     sync.Mutex
	action string
	timer  *time.Timer
	due    time.Time
}

// AfterQueue is what happens when every task has finished.
type AfterQueue struct {
	Action  string `json:"action"`  // none | sleep | shutdown
	Pending bool   `json:"pending"` // counting down
	Seconds int    `json:"seconds"` // left in the countdown
}

func (a *App) afterInfo() AfterQueue {
	a.after.mu.Lock()
	defer a.after.mu.Unlock()
	info := AfterQueue{Action: a.after.action}
	if info.Action == "" {
		info.Action = afterNone
	}
	if a.after.timer != nil {
		info.Pending = true
		info.Seconds = max(0, int(time.Until(a.after.due).Seconds()+0.5))
	}
	return info
}

// GetAfterQueue returns the chosen action (it is not saved: every session starts with "none").
func (a *App) GetAfterQueue() AfterQueue { return a.afterInfo() }

// SetAfterQueue chooses what happens when the queue is done.
func (a *App) SetAfterQueue(action string) AfterQueue {
	switch action {
	case afterSleep, afterShutdown:
	default:
		action = afterNone
	}
	a.after.mu.Lock()
	a.after.action = action
	if action == afterNone && a.after.timer != nil {
		a.after.timer.Stop()
		a.after.timer = nil
	}
	a.after.mu.Unlock()
	info := a.afterInfo()
	a.emit("after:changed", info)
	return info
}

// CancelAfterQueue stops a running countdown and resets the action.
func (a *App) CancelAfterQueue() AfterQueue { return a.SetAfterQueue(afterNone) }

// maybeAfterQueue starts the countdown when nothing is left in the queue.
func (a *App) maybeAfterQueue() {
	if a.queue.Busy() {
		return
	}
	a.after.mu.Lock()
	action := a.after.action
	if action == "" || action == afterNone || a.after.timer != nil {
		a.after.mu.Unlock()
		return
	}
	a.after.due = time.Now().Add(afterDelay)
	a.after.timer = time.AfterFunc(afterDelay, func() {
		a.after.mu.Lock()
		act := a.after.action
		a.after.action = afterNone
		a.after.timer = nil
		a.after.mu.Unlock()
		a.emit("after:changed", a.afterInfo())
		if a.queue.Busy() { // new work arrived during the countdown
			return
		}
		switch act {
		case afterSleep:
			_ = platform.Sleep()
		case afterShutdown:
			_ = platform.Shutdown()
		}
	})
	a.after.mu.Unlock()
	a.emit("after:changed", a.afterInfo())
}

// ---- "Send to" menu & opening files (G-15) ---------------------------------------------

const sendToName = "KuyMediaBox.lnk"

// GetSendTo reports whether KuyMediaBox is in Explorer's "Send to" menu.
func (a *App) GetSendTo() bool {
	dir := platform.SendToDir()
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, sendToName))
	return err == nil
}

// SetSendTo adds or removes KuyMediaBox in Explorer's "Send to" menu.
func (a *App) SetSendTo(on bool) (bool, error) {
	dir := platform.SendToDir()
	if dir == "" {
		return false, errors.New(i18n.L("Hanya tersedia di Windows", "Only available on Windows"))
	}
	lnk := filepath.Join(dir, sendToName)
	if !on {
		if err := os.Remove(lnk); err != nil && !os.IsNotExist(err) {
			return true, err
		}
		return false, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	if err := platform.CreateShortcut(lnk, exe, i18n.L("Buka file di KuyMediaBox", "Open files in KuyMediaBox")); err != nil {
		return false, err
	}
	return true, nil
}

// OpenRequest tells the UI which page receives files opened from Explorer.
type OpenRequest struct {
	Page  string   `json:"page"` // image | video | audio | pdf
	Paths []string `json:"paths"`
}

// launchArgs keeps files passed on the command line until the UI asks for them.
type launchArgs struct {
	mu    sync.Mutex
	paths []string
}

func existingPaths(args []string) []string {
	var out []string
	for _, p := range args {
		if strings.HasPrefix(p, "-") {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// TakeLaunchFiles returns (once) the files the app was started with.
func (a *App) TakeLaunchFiles() *OpenRequest {
	a.launch.mu.Lock()
	paths := a.launch.paths
	a.launch.paths = nil
	a.launch.mu.Unlock()
	if len(paths) == 0 {
		return nil
	}
	req := a.RoutePaths(paths)
	return &req
}

// RoutePaths picks the page that fits most of the files (folders are scanned).
func (a *App) RoutePaths(paths []string) OpenRequest {
	best, bestN := "", 0
	for _, kind := range []string{queue.KindImage, queue.KindVideo, kindPDF, queue.KindAudio} {
		n := len(collectLimited(kind, paths, 2000))
		if kind == queue.KindAudio {
			// Audio also accepts video files: only count real audio for the choice.
			n = 0
			for _, p := range collectLimited(kind, paths, 2000) {
				if audioExts[strings.ToLower(filepath.Ext(p))] {
					n++
				}
			}
		}
		if n > bestN {
			best, bestN = kind, n
		}
	}
	return OpenRequest{Page: best, Paths: paths}
}

func collectLimited(kind string, paths []string, limit int) []string {
	out := collect(kind, paths)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// openFromExplorer handles files sent to an already running window.
func (a *App) openFromExplorer(args []string) {
	paths := existingPaths(args)
	if len(paths) == 0 || a.ctx == nil {
		return
	}
	a.emit("files:open", a.RoutePaths(paths))
}

// ---- Watched folders (G-16) ------------------------------------------------------------

type watchFile struct {
	size  int64
	mod   time.Time
	since time.Time // when size/mod last changed
}

type watcher struct {
	mu    sync.Mutex
	rules []config.WatchRule
	seen  map[string]map[string]*watchFile // rule id → path → state
	done  map[string]bool                  // paths already queued
	stop  chan struct{}
}

// stableFor is how long a new file must stay unchanged before it is processed (copies in progress).
const stableFor = 3 * time.Second

func (a *App) startWatcher() {
	a.watch = &watcher{seen: map[string]map[string]*watchFile{}, done: map[string]bool{}, stop: make(chan struct{})}
	a.updateWatch(a.cfg.Get().Watch)
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-a.watch.stop:
				return
			case <-t.C:
				a.pollWatch()
			}
		}
	}()
}

func (a *App) stopWatcher() {
	if a.watch != nil {
		close(a.watch.stop)
	}
}

// updateWatch installs new rules. Files already in a newly watched folder are left alone:
// only files that appear afterwards are converted.
func (a *App) updateWatch(rules []config.WatchRule) {
	w := a.watch
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	keep := map[string]bool{}
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		key := r.ID + "|" + strings.ToLower(filepath.Clean(r.Dir)) + "|" + r.Kind
		keep[key] = true
		if _, ok := w.seen[key]; ok {
			continue
		}
		base := map[string]*watchFile{}
		for _, p := range a.watchList(r) {
			base[p] = nil // existing: never processed
		}
		w.seen[key] = base
	}
	for key := range w.seen {
		if !keep[key] {
			delete(w.seen, key)
		}
	}
	w.rules = nil
	for _, r := range rules {
		if r.Enabled {
			w.rules = append(w.rules, r)
		}
	}
}

// watchList lists the convertible files directly inside a watched folder.
func (a *App) watchList(r config.WatchRule) []string {
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		return nil
	}
	suffix := strings.ToLower(a.cfg.Get().Suffix)
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		base := strings.TrimSuffix(low, filepath.Ext(low))
		if strings.HasPrefix(name, ".") || strings.Contains(low, ".kmb-part") || (suffix != "" && strings.HasSuffix(base, suffix)) {
			continue
		}
		if !acceptWatch(r.Kind, filepath.Ext(name)) {
			continue
		}
		out = append(out, filepath.Join(r.Dir, name))
	}
	return out
}

func acceptWatch(kind, ext string) bool {
	ext = strings.ToLower(ext)
	switch kind {
	case queue.KindImage:
		return imageconv.InputExts[ext]
	case queue.KindVideo:
		return videoExts[ext]
	case queue.KindAudio:
		return audioExts[ext]
	}
	return false
}

func (a *App) pollWatch() {
	w := a.watch
	w.mu.Lock()
	rules := append([]config.WatchRule(nil), w.rules...)
	w.mu.Unlock()
	now := time.Now()
	for _, r := range rules {
		key := r.ID + "|" + strings.ToLower(filepath.Clean(r.Dir)) + "|" + r.Kind
		var ready []string
		for _, p := range a.watchList(r) {
			st, err := os.Stat(p)
			if err != nil {
				continue
			}
			lp := strings.ToLower(filepath.Clean(p))
			a.producedMu.Lock()
			own := a.produced[lp]
			a.producedMu.Unlock()
			w.mu.Lock()
			seen := w.seen[key]
			if seen == nil || own || w.done[lp] {
				w.mu.Unlock()
				continue
			}
			f, known := seen[p]
			switch {
			case known && f == nil: // there before the rule started
			case !known:
				seen[p] = &watchFile{size: st.Size(), mod: st.ModTime(), since: now}
			case f.size != st.Size() || !f.mod.Equal(st.ModTime()):
				f.size, f.mod, f.since = st.Size(), st.ModTime(), now
			case now.Sub(f.since) >= stableFor && canOpen(p):
				w.done[lp] = true
				delete(seen, p)
				ready = append(ready, p)
			}
			w.mu.Unlock()
		}
		if len(ready) > 0 {
			if err := a.startWatched(r, ready); err != nil {
				a.emit("watch:error", map[string]string{"dir": r.Dir, "error": err.Error()})
			} else {
				a.emit("watch:queued", map[string]any{"dir": r.Dir, "kind": r.Kind, "count": len(ready)})
			}
		}
	}
}

// canOpen reports whether a file can be read (not locked by the program still writing it).
func canOpen(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// DataFolder returns the app data folder (tools, caches, history).
func (a *App) DataFolder() string { return appdir.DataDir() }

// OpenFileDefault opens a file with its default program.
func (a *App) OpenFileDefault(path string) error {
	if _, err := os.Stat(path); err != nil {
		return errors.New(i18n.L("File tidak ditemukan", "File not found"))
	}
	return platform.OpenFolder(path) // Explorer opens a file with its default program
}

// PickFile lets the user choose one file; returns "" when cancelled.
func (a *App) PickFile(title, filterName, pattern string) (string, error) {
	opts := wruntime.OpenDialogOptions{Title: title}
	if pattern != "" {
		opts.Filters = []wruntime.FileFilter{{DisplayName: filterName, Pattern: pattern}, {DisplayName: i18n.L("Semua file", "All files"), Pattern: "*.*"}}
	}
	return wruntime.OpenFileDialog(a.ctx, opts)
}
