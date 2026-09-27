//go:build integration

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"kuymediabox/internal/config"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/tools"
)

// isolatedApp is an App whose settings, data and tools live in a temp folder; FFmpeg is
// linked in from the real tools folder (or KMB_TOOLS).
func isolatedApp(t *testing.T) *App {
	t.Helper()
	bin := os.Getenv("KMB_TOOLS")
	if bin == "" {
		bin = filepath.Join(os.Getenv("LOCALAPPDATA"), "KuyMediaBox", "bin")
	}
	root := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(root, "roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	dst := filepath.Join(root, "local", "KuyMediaBox", "bin")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, exe := range []string{"ffmpeg.exe", "ffprobe.exe"} {
		if err := os.Link(filepath.Join(bin, exe), filepath.Join(dst, exe)); err != nil {
			t.Skipf("no %s in %s: %v", exe, bin, err)
		}
	}
	a := NewApp()
	a.tools.Detect(context.Background())
	a.markToolsDetected()
	if a.tools.Path(tools.FFmpeg) == "" {
		t.Skip("ffmpeg not found")
	}
	return a
}

// waitIdle waits until the queue and every workflow are finished.
func waitIdle(t *testing.T, a *App, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		busy := false
		for _, i := range a.queue.List() {
			if i.Status == queue.StatusQueued || i.Status == queue.StatusRunning {
				busy = true
			}
		}
		a.flows.mu.Lock()
		busy = busy || len(a.flows.tasks) > 0
		a.flows.mu.Unlock()
		if !busy {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("still busy")
}

func TestWorkflowVideoToMP3ToEvenM4A(t *testing.T) {
	a := isolatedApp(t)
	in := filepath.Join(t.TempDir(), "klip rapat.mp4")
	gen := exec.Command(a.tools.Path(tools.FFmpeg), "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=3",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3", "-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", in)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("make clip: %v %s", err, out)
	}

	s := a.cfg.Get()
	s.Outputs["audio"] = config.Output{Mode: config.OutputSame}
	s.Workflows = []config.Workflow{{ID: "w1", Name: "Rapat → suara rata", Steps: []config.FlowStep{
		{Kind: "video", Job: json.RawMessage(`{"mode":"audio","audio":{"format":"mp3","bitrate":128}}`)},
		{Kind: "audio", Job: json.RawMessage(`{"mode":"convert","options":{"format":"m4a","bitrate":128,"normalize":true,"loudness":-16}}`)},
	}}}
	if _, err := a.cfg.Set(s); err != nil {
		t.Fatal(err)
	}

	n, err := a.RunWorkflow("rapat → suara rata", []string{in}) // by name, any case
	if err != nil || n != 1 {
		t.Fatalf("run: %d %v", n, err)
	}
	waitIdle(t, a, 2*time.Minute)

	want := filepath.Join(filepath.Dir(in), "klip rapat"+s.Suffix+".m4a")
	if _, err := os.Stat(want); err != nil {
		var got []string
		for _, i := range a.queue.List() {
			got = append(got, i.Kind+" "+i.Status+" "+i.Output+" "+i.Message+" "+i.Detail)
		}
		t.Fatalf("final result missing: %v\n%v", err, got)
	}
	// The in-between MP3 lived in the temp folder and is gone.
	if entries, _ := os.ReadDir(filepath.Join(os.Getenv("LOCALAPPDATA"), "KuyMediaBox", "tmp", "flow")); len(entries) != 0 {
		t.Fatalf("in-between results left: %v", entries)
	}
	// History keeps the final step only.
	var kinds []string
	for _, e := range a.hist.List(0) {
		kinds = append(kinds, e.Kind)
	}
	if len(kinds) != 1 || kinds[0] != "audio" {
		t.Fatalf("history %v", kinds)
	}

	// A failing first step ends the run without leaving anything behind.
	if _, err := a.RunWorkflow("w1", []string{filepath.Join(filepath.Dir(in), "nope.mp4")}); err == nil {
		waitIdle(t, a, time.Minute)
	}
	a.flows.mu.Lock()
	left := len(a.flows.tasks)
	a.flows.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d tasks still tracked", left)
	}
}
