package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/config"
	"kuymediabox/internal/downloader"
	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/naming"
	"kuymediabox/internal/platform"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/tools"
)

// App is bound to the frontend; its exported methods are callable from JavaScript.
type App struct {
	ctx   context.Context
	cfg   *config.Store
	tools *tools.Manager
	queue *queue.Manager
	namer *naming.Namer

	encMu      sync.Mutex
	encPath    string
	encoderSet map[string]bool

	colMu       sync.Mutex
	collections map[string]*downloader.Collection
	colSeq      int

	notifyOK bool
	upd      updateState
}

// NewApp creates the application state.
func NewApp() *App {
	a := &App{cfg: config.Load(), namer: naming.NewNamer(), collections: map[string]*downloader.Collection{}}
	i18n.Set(a.cfg.Get().Language)
	a.tools = tools.New(a.cfg, func(list []tools.Status) { a.emit("tools:changed", list) })
	a.queue = queue.New(func(info queue.Info) { a.emit("task:update", info) }, a.onBatchDone)
	return a
}

func (a *App) emit(name string, data any) {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, name, data)
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	cleanTemp()
	if err := wruntime.InitializeNotifications(ctx); err == nil {
		a.notifyOK = true
	}
	go func() {
		a.tools.Detect(context.Background())
		a.tools.CheckUpdates(context.Background())
	}()
	go a.autoCheckUpdate()
}

func (a *App) shutdown(ctx context.Context) {
	a.queue.Shutdown()
	// Give killed processes a moment so temp files are released before cleanup.
	time.Sleep(300 * time.Millisecond)
	cleanTemp()
}

func cleanTemp() {
	dir := appdir.TempDir()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}

// kindTitle is the notification title for a finished batch.
func kindTitle(kind string) string {
	switch kind {
	case queue.KindImage:
		return i18n.L("Konversi gambar selesai", "Image conversion finished")
	case queue.KindVideo:
		return i18n.L("Konversi video selesai", "Video conversion finished")
	case queue.KindAudio:
		return i18n.L("Konversi audio selesai", "Audio conversion finished")
	case queue.KindPDF:
		return i18n.L("Alat PDF selesai", "PDF tools finished")
	}
	return i18n.L("Download selesai", "Download finished")
}

func (a *App) onBatchDone(kind string, done, failed, skipped, canceled int) {
	a.emit("batch:done", map[string]any{"kind": kind, "done": done, "failed": failed, "skipped": skipped, "canceled": canceled})
	if !a.notifyOK || !a.cfg.Get().Notify || done+failed == 0 {
		return
	}
	var parts []string
	if done > 0 {
		parts = append(parts, fmt.Sprintf(i18n.L("%d berhasil", "%d succeeded"), done))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf(i18n.L("%d gagal", "%d failed"), failed))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf(i18n.L("%d dilewati", "%d skipped"), skipped))
	}
	_ = wruntime.SendNotification(a.ctx, wruntime.NotificationOptions{
		ID:    fmt.Sprintf("kmb-%d", time.Now().UnixNano()),
		Title: kindTitle(kind),
		Body:  strings.Join(parts, " · "),
	})
}

// ---- Settings -------------------------------------------------------------------------

// GetSettings returns the saved settings.
func (a *App) GetSettings() config.Settings { return a.cfg.Get() }

// SaveSettings stores new settings.
func (a *App) SaveSettings(s config.Settings) (config.Settings, error) {
	cur := a.cfg.Get()
	s.ToolPaths = cur.ToolPaths // tool paths are managed separately
	saved, err := a.cfg.Set(s)
	if err == nil {
		i18n.Set(saved.Language)
		if saved.Language != cur.Language {
			a.emit("tools:changed", a.tools.List()) // descriptions are translated
		}
	}
	return saved, err
}

// GetDefaultDirs returns the default result folder of every module.
func (a *App) GetDefaultDirs() map[string]string {
	out := map[string]string{}
	for _, k := range config.OutputKinds {
		out[k] = appdir.DefaultOutputDir(k)
	}
	return out
}

// outputSpec turns a module's saved result-folder setting into a concrete spec.
func (a *App) outputSpec(kind string) naming.OutputSpec {
	o := a.cfg.Get().Outputs[kind]
	switch o.Mode {
	case config.OutputSubfolder, config.OutputSame:
		return naming.OutputSpec{Mode: o.Mode}
	case config.OutputCustom:
		return naming.OutputSpec{Mode: config.OutputCustom, Dir: o.Dir}
	default:
		return naming.OutputSpec{Mode: config.OutputCustom, Dir: appdir.DefaultOutputDir(kind)}
	}
}

// OutputFolder returns the fixed result folder of a module, or "" when it is dynamic
// (next to each source file).
func (a *App) OutputFolder(kind string) string {
	spec := a.outputSpec(kind)
	if spec.Mode == config.OutputCustom {
		return spec.Dir
	}
	return ""
}

// onSecondInstance brings the running window to the front when the app is started again.
func (a *App) onSecondInstance(_ options.SecondInstanceData) {
	if a.ctx == nil {
		return
	}
	wruntime.WindowUnminimise(a.ctx)
	wruntime.WindowShow(a.ctx)
	wruntime.WindowSetAlwaysOnTop(a.ctx, true)
	wruntime.WindowSetAlwaysOnTop(a.ctx, false)
}

// PickDirectory lets the user choose a folder; returns "" when cancelled.
func (a *App) PickDirectory(title, start string) (string, error) {
	opts := wruntime.OpenDialogOptions{Title: title, CanCreateDirectories: true}
	if st, err := os.Stat(start); err == nil && st.IsDir() {
		opts.DefaultDirectory = start
	}
	return wruntime.OpenDirectoryDialog(a.ctx, opts)
}

// OpenFolder shows a folder in Explorer (creating it if needed).
func (a *App) OpenFolder(dir string) error {
	if dir == "" {
		return nil
	}
	_ = os.MkdirAll(dir, 0o755)
	return platform.OpenFolder(dir)
}

// RevealFile opens Explorer with the file selected.
func (a *App) RevealFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return a.OpenFolder(filepath.Dir(path))
	}
	return platform.RevealFile(path)
}

// ---- Tools ----------------------------------------------------------------------------

// GetTools returns the status of every external tool.
func (a *App) GetTools() []tools.Status { return a.tools.List() }

// RecheckTools locates tools again and checks for updates.
func (a *App) RecheckTools() []tools.Status {
	a.tools.Detect(context.Background())
	a.resetEncoders()
	go a.tools.CheckUpdates(context.Background())
	return a.tools.List()
}

// InstallTool downloads or updates a tool; progress arrives through "tools:changed".
func (a *App) InstallTool(id string) error {
	err := a.tools.Install(context.Background(), id)
	a.resetEncoders()
	return err
}

// PickToolPath lets the user point to an existing executable.
func (a *App) PickToolPath(id string) error {
	p, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:   i18n.L("Pilih file program", "Choose the program file"),
		Filters: []wruntime.FileFilter{{DisplayName: i18n.L("Program (*.exe)", "Programs (*.exe)"), Pattern: "*.exe"}},
	})
	if err != nil || p == "" {
		return err
	}
	err = a.tools.SetCustomPath(context.Background(), id, p)
	a.resetEncoders()
	return err
}

// ResetToolPath forgets a custom tool path.
func (a *App) ResetToolPath(id string) error {
	err := a.tools.SetCustomPath(context.Background(), id, "")
	a.resetEncoders()
	return err
}

// Capabilities tells the UI which encoders are available.
type Capabilities struct {
	FFmpeg   bool            `json:"ffmpeg"`
	Encoders map[string]bool `json:"encoders"`
}

// GetCapabilities lists usable video encoders (h264, h265, vp9, av1).
func (a *App) GetCapabilities() Capabilities {
	set := a.encoders()
	caps := Capabilities{FFmpeg: a.tools.Path(tools.FFmpeg) != "", Encoders: map[string]bool{}}
	for codec, names := range map[string][]string{
		"h264": {"libx264"}, "h265": {"libx265"}, "vp9": {"libvpx-vp9"}, "av1": {"libsvtav1", "libaom-av1"},
	} {
		for _, n := range names {
			if set[n] {
				caps.Encoders[codec] = true
			}
		}
	}
	return caps
}

func (a *App) encoders() map[string]bool {
	path := a.tools.Path(tools.FFmpeg)
	a.encMu.Lock()
	defer a.encMu.Unlock()
	if path == "" {
		return map[string]bool{}
	}
	if a.encoderSet != nil && a.encPath == path {
		return a.encoderSet
	}
	set, err := ffmpeg.Encoders(context.Background(), path)
	if err != nil {
		return map[string]bool{}
	}
	a.encPath, a.encoderSet = path, set
	return set
}

func (a *App) resetEncoders() {
	a.encMu.Lock()
	a.encoderSet = nil
	a.encMu.Unlock()
}

// ---- Tasks ----------------------------------------------------------------------------

// ListTasks returns every known task.
func (a *App) ListTasks() []queue.Info { return a.queue.List() }

// CancelTask stops one task.
func (a *App) CancelTask(id string) { a.queue.Cancel(id) }

// CancelKind stops all tasks of a kind (image, video, audio, download).
func (a *App) CancelKind(kind string) { a.queue.CancelKind(kind) }

// ForgetTasks drops finished tasks from memory.
func (a *App) ForgetTasks(ids []string) { a.queue.Forget(ids) }
