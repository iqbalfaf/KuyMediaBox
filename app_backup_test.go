package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kuymediabox/internal/config"
)

func TestBackupRoundTrip(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(root, "roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	a := NewApp()
	s := a.cfg.Get()
	s.Suffix = "_hemat"
	s.ToolPaths = map[string]string{"ffmpeg": `C:\ffmpeg\ffmpeg.exe`}
	s.Workflows = []config.Workflow{{ID: "w1", Name: "Lagu", Steps: []config.FlowStep{{Kind: "audio", Job: json.RawMessage(`{}`)}}}}
	if _, err := a.cfg.Set(s); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "backup.json")
	ui := map[string]string{"kmb.video": `{"mode":"audio"}`, "kmb.presets.video": `{"list":[{"id":"p1"},{"id":"p2"}]}`, "other": `{}`, "kmb.bad": `{nope`}
	if err := writeBackup(path, a.cfg.Get(), ui); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if json.Valid(b) == false || containsStr(string(b), `ffmpeg.exe`) || containsStr(string(b), `"other"`) {
		t.Fatalf("backup content: %s", b)
	}

	// Another PC: different settings and its own tool paths.
	s = a.cfg.Get()
	s.Suffix = "_x"
	s.Workflows = nil
	s.ToolPaths = map[string]string{"ffmpeg": `D:\tools\ffmpeg.exe`}
	a.cfg.Set(s)
	// Importing brings back the backup's settings and workflows but keeps this PC's tool paths.
	res, err := a.importBackup(path)
	if err != nil {
		t.Fatal(err)
	}
	got := a.cfg.Get()
	if got.Suffix != "_hemat" || len(got.Workflows) != 1 || got.ToolPaths["ffmpeg"] != `D:\tools\ffmpeg.exe` {
		t.Fatalf("imported %+v", got)
	}
	if res.Presets != 2 || len(res.UI) != 2 || res.UI["kmb.video"] == "" || res.Workflows != 1 {
		t.Fatalf("result %+v", res)
	}

	// A backup saved again in Notepad (UTF-8 with BOM) still restores the settings.
	a.cfg.Set(s)
	os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, b...), 0o644)
	if _, err := a.importBackup(path); err != nil || a.cfg.Get().Suffix != "_hemat" {
		t.Fatalf("BOM backup: err %v suffix %q", err, a.cfg.Get().Suffix)
	}

	os.WriteFile(path, []byte(`{"app":"Lain"}`), 0o644)
	if _, err := a.importBackup(path); err == nil {
		t.Fatal("foreign file accepted")
	}
}

func containsStr(s, sub string) bool { return strings.Contains(s, sub) }
