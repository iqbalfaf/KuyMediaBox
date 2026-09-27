package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/platform"
	"kuymediabox/internal/queue"
)

// System tray: with the setting on, closing the window only hides it, so subscriptions,
// watched folders, the clipboard watcher and running tasks keep going.

//go:embed build/windows/icon.ico
var trayIcon []byte

// trayArg starts the app hidden in the tray (used by "start with Windows").
const trayArg = "--tray"

const autostartName = "KuyMediaBox"

type trayState struct {
	mu      sync.Mutex
	running bool
	stop    chan struct{}
	hinted  bool        // the "still running in the tray" notice was shown
	quit    atomic.Bool // the user really wants to quit
	hidden  atomic.Bool // started with --tray and not shown yet
}

func startedInTray() bool { return slices.Contains(os.Args[1:], trayArg) }

// startTray shows the tray icon (once).
func (a *App) startTray() {
	t := &a.tray
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		return
	}
	t.running = true
	t.stop = make(chan struct{})
	stop := t.stop
	go func() {
		// The tray window and its message loop must stay on one OS thread.
		runtime.LockOSThread()
		systray.Run(func() { a.trayReady(stop) }, nil)
	}()
}

// stopTray removes the tray icon when the app shuts down. fyne's tray can't be started
// again in the same process, so this is only called on exit.
func (a *App) stopTray() {
	t := &a.tray
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.running {
		return
	}
	t.running = false
	close(t.stop)
	systray.Quit()
}

func (a *App) trayReady(stop chan struct{}) {
	systray.SetIcon(trayIcon)
	systray.SetTooltip("KuyMediaBox")
	systray.SetOnTapped(a.showWindow)
	open := systray.AddMenuItem("", "")
	status := systray.AddMenuItem("", "")
	status.Disable()
	systray.AddSeparator()
	quit := systray.AddMenuItem("", "")
	lang, last := "", "-"
	refresh := func() {
		if l := a.cfg.Get().Language; l != lang {
			lang = l
			open.SetTitle(i18n.L("Buka KuyMediaBox", "Open KuyMediaBox"))
			quit.SetTitle(i18n.L("Keluar", "Quit"))
			last = "-"
		}
		s := a.trayStatus()
		if s != last {
			last = s
			status.SetTitle(s)
			systray.SetTooltip("KuyMediaBox · " + s)
		}
	}
	refresh()
	go func() {
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-open.ClickedCh:
				a.showWindow()
			case <-quit.ClickedCh:
				a.quitApp()
			case <-tick.C:
				refresh()
			}
		}
	}()
}

// trayStatus summarises the queue ("2 tugas berjalan").
func (a *App) trayStatus() string {
	active := 0
	for _, t := range a.queue.List() {
		if t.Status == queue.StatusRunning || t.Status == queue.StatusQueued {
			active++
		}
	}
	if active == 0 {
		return i18n.L("Tidak ada tugas", "No tasks")
	}
	return fmt.Sprintf(i18n.L("%d tugas berjalan", "%d tasks running"), active)
}

func (a *App) showWindow() {
	if a.ctx == nil {
		return
	}
	a.tray.hidden.Store(false)
	wruntime.WindowShow(a.ctx)
	wruntime.WindowUnminimise(a.ctx)
	wruntime.WindowSetAlwaysOnTop(a.ctx, true)
	wruntime.WindowSetAlwaysOnTop(a.ctx, false)
}

// quitApp closes the app for real (the tray menu's Quit).
func (a *App) quitApp() {
	a.tray.quit.Store(true)
	if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}
}

// beforeClose hides the window instead of quitting while the tray setting is on.
func (a *App) beforeClose(ctx context.Context) bool {
	if a.tray.quit.Load() || !a.cfg.Get().Tray {
		return false
	}
	a.startTray()
	wruntime.WindowHide(ctx)
	a.tray.mu.Lock()
	first := !a.tray.hinted
	a.tray.hinted = true
	a.tray.mu.Unlock()
	if first && a.notifyOK {
		_ = wruntime.SendNotification(ctx, wruntime.NotificationOptions{
			ID:    fmt.Sprintf("kmb-tray-%d", time.Now().UnixNano()),
			Title: i18n.L("KuyMediaBox masih berjalan", "KuyMediaBox is still running"),
			Body:  i18n.L("Tugas, langganan, dan folder pantauan tetap jalan. Klik ikon di tray untuk membuka, klik kanan untuk keluar.", "Tasks, subscriptions and watched folders keep running. Click the tray icon to open it, right-click to quit."),
		})
	}
	return true
}

// QuitApp quits even when the tray setting is on (the app's own quit button).
func (a *App) QuitApp() { a.quitApp() }

// CloseWindow is the window's close button: to the tray when that is on, else quit.
func (a *App) CloseWindow() {
	if a.ctx != nil {
		wruntime.Quit(a.ctx) // goes through beforeClose
	}
}

func autostartCommand() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return `"` + exe + `" ` + trayArg
}

// GetAutostart reports whether the app starts (in the tray) when Windows starts.
func (a *App) GetAutostart() bool {
	return platform.AutostartCommand(autostartName) != ""
}

// SetAutostart turns starting with Windows on or off; it also turns the tray on.
func (a *App) SetAutostart(on bool) (bool, error) {
	cmd := autostartCommand()
	if on && cmd == "" {
		return false, fmt.Errorf("%s", i18n.L("Lokasi aplikasi tidak diketahui", "The app location is unknown"))
	}
	if err := platform.SetAutostart(autostartName, cmd, on); err != nil {
		return a.GetAutostart(), err
	}
	if on && !a.cfg.Get().Tray {
		s := a.cfg.Get()
		s.Tray = true
		if _, err := a.SaveSettings(s); err != nil {
			return true, err
		}
	}
	return a.GetAutostart(), nil
}

// refreshAutostart points an existing sign-in entry at the current exe (after the portable
// exe moved or was updated under a new name).
func (a *App) refreshAutostart() {
	cur := platform.AutostartCommand(autostartName)
	if cmd := autostartCommand(); cur != "" && cmd != "" && cur != cmd {
		_ = platform.SetAutostart(autostartName, cmd, true)
	}
}
