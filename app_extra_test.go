package main

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kuymediabox/internal/config"
	"kuymediabox/internal/history"
	"kuymediabox/internal/naming"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/tools"
)

func writePNG(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 40, 30))); err != nil {
		t.Fatal(err)
	}
}

// A watched folder converts files that appear after the rule starts, never the ones already
// there and never the app's own results.
func TestWatchedFolder(t *testing.T) {
	dir := t.TempDir()
	watched := filepath.Join(dir, "in")
	outDir := filepath.Join(dir, "out")
	_ = os.MkdirAll(watched, 0o755)
	writePNG(t, filepath.Join(watched, "old.png"))

	cfg := config.LoadFrom(filepath.Join(dir, "settings.json"))
	s := cfg.Get()
	s.Outputs["image"] = config.Output{Mode: config.OutputCustom, Dir: outDir}
	opts, _ := json.Marshal(map[string]any{"format": "jpg", "quality": 80})
	s.Watch = []config.WatchRule{{ID: "w1", Dir: watched, Kind: "image", Options: opts, Enabled: true}}
	if _, err := cfg.Set(s); err != nil {
		t.Fatal(err)
	}
	done := make(chan queue.Info, 4)
	a := &App{cfg: cfg, namer: naming.NewNamer(), produced: map[string]bool{}, hist: history.Open(filepath.Join(dir, "h.jsonl"))}
	a.tools = tools.New(cfg, nil)
	a.queue = queue.New(nil, nil)
	a.queue.OnFinish = func(i queue.Info) { a.recordFinished(i); done <- i }
	a.watch = &watcher{seen: map[string]map[string]*watchFile{}, done: map[string]bool{}, stop: make(chan struct{})}
	a.updateWatch(cfg.Get().Watch)

	writePNG(t, filepath.Join(watched, "new.png"))
	deadline := time.Now().Add(15 * time.Second)
	var info queue.Info
	for got := false; !got && time.Now().Before(deadline); {
		a.pollWatch()
		select {
		case info = <-done:
			got = true
		case <-time.After(500 * time.Millisecond):
		}
	}
	if info.Status != queue.StatusDone || filepath.Base(info.Output) != "new_converted.jpg" {
		t.Fatalf("watched conversion: %+v", info)
	}
	// Nothing else is picked up: the old file and the result stay untouched.
	for i := 0; i < 3; i++ {
		a.pollWatch()
		time.Sleep(200 * time.Millisecond)
	}
	select {
	case extra := <-done:
		t.Fatalf("unexpected task %+v", extra)
	default:
	}
	if list := a.hist.List(0); len(list) != 1 || list[0].Input == "" || list[0].InSize == 0 {
		t.Fatalf("history %+v", list)
	}
}

func TestSettingsNormalizeNewFields(t *testing.T) {
	s := config.Settings{
		Parallel:       map[string]int{"video": 40, "image": 0},
		Theme:          "neon",
		CookiesBrowser: "netscape",
		NameTemplate:   ` {uploader}/{title}?* `,
		Watch:          []config.WatchRule{{ID: "a", Dir: "", Kind: "image"}, {ID: "b", Dir: `C:\x`, Kind: "pdf"}, {ID: "c", Dir: `C:\y`, Kind: "audio"}},
	}
	s.Normalize()
	if s.Parallel["video"] != config.MaxParallel || s.Parallel["image"] != 3 || s.Parallel["download"] != 2 {
		t.Fatalf("parallel %v", s.Parallel)
	}
	if s.Theme != config.ThemeDark || s.CookiesBrowser != "" || s.NameTemplate != "{uploader}{title}" {
		t.Fatalf("normalize %+v", s)
	}
	if len(s.Watch) != 1 || s.Watch[0].ID != "c" || string(s.Watch[0].Options) != "{}" {
		t.Fatalf("watch rules %+v", s.Watch)
	}
}
