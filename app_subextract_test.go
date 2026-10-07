//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kuymediabox/internal/config"
	"kuymediabox/internal/mediaconv"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/tools"
)

// TestSubExtractTask saves the subtitle tracks and fonts of an MKV through the queue.
func TestSubExtractTask(t *testing.T) {
	a := isolatedApp(t)
	dir := t.TempDir()
	srt := filepath.Join(dir, "id.srt")
	os.WriteFile(srt, []byte("1\n00:00:00,500 --> 00:00:02,000\nHalo dunia\n"), 0o644)
	ff := a.tools.Path(tools.FFmpeg)
	ass := filepath.Join(dir, "en.ass")
	if out, err := exec.Command(ff, "-hide_banner", "-loglevel", "error", "-i", srt, ass).CombinedOutput(); err != nil {
		t.Fatalf("make ass: %v %s", err, out)
	}
	in := filepath.Join(dir, "film.mkv")
	args := []string{"-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=3", "-i", srt, "-i", ass,
		"-map", "0", "-map", "1", "-map", "2", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:s", "copy", "-metadata:s:s:0", "language=ind"}
	font := filepath.Join(os.Getenv("WINDIR"), "Fonts", "arial.ttf")
	_, fontErr := os.Stat(font)
	if fontErr == nil {
		args = append(args, "-attach", font, "-metadata:s:t", "mimetype=application/x-truetype-font")
	}
	if out, err := exec.Command(ff, append(args, in)...).CombinedOutput(); err != nil {
		t.Fatalf("make mkv: %v %s", err, out)
	}
	s := a.cfg.Get()
	s.Outputs["video"] = config.Output{Mode: config.OutputSame}
	a.cfg.Set(s)

	run := func(o mediaconv.SubExtractOptions) queue.Info {
		t.Helper()
		refs, err := a.StartVideo([]JobItem{{ID: nextItemID(), Path: in}}, VideoJob{Mode: "subs", Subs: o})
		if err != nil {
			t.Fatal(err)
		}
		waitIdle(t, a, time.Minute)
		for _, i := range a.queue.List() {
			if i.ID == refs[0].TaskID {
				return i
			}
		}
		t.Fatal("task not found")
		return queue.Info{}
	}

	info := run(mediaconv.SubExtractOptions{Fonts: true})
	if info.Status != queue.StatusDone {
		t.Fatalf("%+v", info)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "film.ind.srt")); !strings.Contains(string(data), "Halo dunia") {
		t.Fatalf("film.ind.srt: %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "film.ass")); !strings.Contains(string(data), "[Script Info]") {
		t.Fatalf("film.ass: %q", data)
	}
	if fontErr == nil {
		if _, err := os.Stat(filepath.Join(dir, "film_fonts", "arial.ttf")); err != nil {
			t.Fatal(err)
		}
	}

	if info := run(mediaconv.SubExtractOptions{Langs: "ja"}); info.Status != queue.StatusSkipped {
		t.Fatalf("no Japanese track should skip: %+v", info)
	}

	setConflict := func(c string) {
		s := a.cfg.Get()
		s.Conflict = c
		a.cfg.Set(s)
	}
	fontDirs := func() []string {
		m, _ := filepath.Glob(filepath.Join(dir, "film_fonts*"))
		return m
	}

	// Run again with "skip": everything exists, so the task is skipped and no fonts are copied.
	setConflict(config.ConflictSkip)
	if info := run(mediaconv.SubExtractOptions{Fonts: true}); info.Status != queue.StatusSkipped {
		t.Fatalf("rerun with skip: %+v", info)
	}
	if fontErr == nil && len(fontDirs()) != 1 {
		t.Fatalf("fonts folders after skip: %v", fontDirs())
	}

	// SRT only: the fonts are useless without ASS and aren't saved.
	setConflict(config.ConflictRename)
	if info := run(mediaconv.SubExtractOptions{Format: "srt", Fonts: true}); info.Status != queue.StatusDone || strings.Contains(info.Message, "font") {
		t.Fatalf("srt with fonts: %+v", info)
	}
	if fontErr == nil && len(fontDirs()) != 1 {
		t.Fatalf("fonts folders after SRT: %v", fontDirs())
	}

	// Run again with "rename": new files and a numbered fonts folder.
	if info := run(mediaconv.SubExtractOptions{Fonts: true}); info.Status != queue.StatusDone {
		t.Fatalf("rerun with rename: %+v", info)
	}
	if _, err := os.Stat(filepath.Join(dir, "film (1).ass")); err != nil {
		t.Fatal(err)
	}
	if fontErr == nil {
		if _, err := os.Stat(filepath.Join(dir, "film_fonts (2)", "arial.ttf")); err != nil {
			t.Fatal(err)
		}
	}
}
