package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/updater"
)

// Version is the app version. Release builds set it with -ldflags "-X main.Version=x.y.z".
var Version = "0.1.4"

type updateState struct {
	mu         sync.Mutex
	last       *updater.Info
	installing bool
}

// GetVersion returns the running version.
func (a *App) GetVersion() string { return Version }

// CheckUpdate asks GitHub Releases whether a newer version exists.
func (a *App) CheckUpdate() (updater.Info, error) {
	info, err := updater.Check(context.Background(), Version)
	if err != nil {
		return info, err
	}
	a.upd.mu.Lock()
	a.upd.last = &info
	a.upd.mu.Unlock()
	return info, nil
}

// autoCheckUpdate runs shortly after startup when enabled in the settings.
func (a *App) autoCheckUpdate() {
	updater.Cleanup()
	_ = os.RemoveAll(filepath.Join(appdir.DataDir(), "update")) // installer left from a finished update
	if !a.cfg.Get().AutoUpdate {
		return
	}
	time.Sleep(4 * time.Second)
	info, err := a.CheckUpdate()
	if err == nil && info.Available {
		a.emit("update:available", info)
	}
}

// InstallUpdate downloads the newest release, verifies it and restarts into it.
// Progress arrives as "update:progress" (0..1). Running tasks are cancelled.
func (a *App) InstallUpdate() error {
	a.upd.mu.Lock()
	if a.upd.installing {
		a.upd.mu.Unlock()
		return errors.New(i18n.L("update sedang berjalan", "an update is already running"))
	}
	a.upd.installing = true
	a.upd.mu.Unlock()
	defer func() {
		a.upd.mu.Lock()
		a.upd.installing = false
		a.upd.mu.Unlock()
	}()

	info, err := updater.Check(context.Background(), Version)
	if err != nil {
		return err
	}
	if !info.Available {
		return errors.New(i18n.L("aplikasi sudah versi terbaru", "the app is already up to date"))
	}

	// Not under TempDir: that folder is wiped on shutdown, before the installer runs.
	dir := filepath.Join(appdir.DataDir(), "update")
	if info.Mode == updater.ModePortable {
		// The new exe must sit on the same drive as the current one so it can be swapped in.
		if exe, err := os.Executable(); err == nil {
			dir = filepath.Dir(exe)
		}
	}
	a.emit("update:progress", map[string]any{"stage": "download", "progress": 0})
	path, err := updater.Download(context.Background(), info, dir, func(p float64) {
		a.emit("update:progress", map[string]any{"stage": "download", "progress": p})
	})
	if err != nil {
		return err
	}

	a.emit("update:progress", map[string]any{"stage": "install", "progress": 1})
	for _, k := range []string{queue.KindImage, queue.KindVideo, queue.KindAudio, queue.KindDownload} {
		a.queue.CancelKind(k)
	}
	if err := updater.Apply(path, info.Mode); err != nil {
		_ = os.Remove(path)
		return err
	}
	go func() {
		time.Sleep(700 * time.Millisecond)
		wruntime.Quit(a.ctx)
	}()
	return nil
}
