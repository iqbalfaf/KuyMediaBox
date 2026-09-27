package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kuymediabox/internal/config"
	"kuymediabox/internal/i18n"
)

// Settings backup: the global settings plus the pages' own settings and presets (kept in
// the WebView's storage, so the frontend hands them over) in one JSON file, e.g. to move to a
// new PC.

const backupApp = "KuyMediaBox"

type backupFile struct {
	App      string            `json:"app"`
	Version  string            `json:"version"`
	Exported string            `json:"exported"`
	Settings config.Settings   `json:"settings"`
	UI       map[string]string `json:"ui"` // page settings & presets: storage key → JSON
}

// ImportResult is what the frontend restores after an import.
type ImportResult struct {
	UI        map[string]string `json:"ui"`
	Presets   int               `json:"presets"`
	Workflows int               `json:"workflows"`
	Version   string            `json:"version"`
}

// ExportSettings saves the settings and ui (page settings & presets) to a file the user
// chooses; returns its path ("" when cancelled).
func (a *App) ExportSettings(ui map[string]string) (string, error) {
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:           i18n.L("Simpan cadangan pengaturan", "Save a settings backup"),
		DefaultFilename: "KuyMediaBox-" + i18n.L("pengaturan", "settings") + "-" + time.Now().Format("2006-01-02") + ".json",
		Filters:         []wruntime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		path += ".json"
	}
	return path, writeBackup(path, a.cfg.Get(), ui)
}

func writeBackup(path string, s config.Settings, ui map[string]string) error {
	s.ToolPaths = map[string]string{} // paths of this PC only
	clean := map[string]string{}
	for k, v := range ui {
		if strings.HasPrefix(k, "kmb.") && json.Valid([]byte(v)) {
			clean[k] = v
		}
	}
	b, err := json.MarshalIndent(backupFile{App: backupApp, Version: Version, Exported: time.Now().Format(time.RFC3339), Settings: s, UI: clean}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// ImportSettings reads a backup the user chooses and applies its settings; the frontend
// restores ui. Returns nil when cancelled.
func (a *App) ImportSettings() (*ImportResult, error) {
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:   i18n.L("Buka cadangan pengaturan", "Open a settings backup"),
		Filters: []wruntime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	return a.importBackup(path)
}

func (a *App) importBackup(path string) (*ImportResult, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f backupFile
	if err := json.Unmarshal(bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF}), &f); err != nil || f.App != backupApp {
		return nil, errors.New(i18n.L("File ini bukan cadangan pengaturan KuyMediaBox", "This file isn't a KuyMediaBox settings backup"))
	}
	// Fields the backup doesn't have keep their current values (older versions).
	s := a.cfg.Get()
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	var rawSettings map[string]json.RawMessage
	_ = json.Unmarshal(raw["settings"], &rawSettings)
	cur, _ := json.Marshal(s)
	var merged map[string]json.RawMessage
	_ = json.Unmarshal(cur, &merged)
	for k, v := range rawSettings {
		if k != "toolPaths" {
			merged[k] = v
		}
	}
	mb, _ := json.Marshal(merged)
	var next config.Settings
	if err := json.Unmarshal(mb, &next); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.L("Isi cadangan rusak", "The backup is damaged"), err)
	}
	if _, err := a.SaveSettings(next); err != nil {
		return nil, err
	}
	res := &ImportResult{UI: map[string]string{}, Workflows: len(next.Workflows), Version: f.Version}
	for k, v := range f.UI {
		if !strings.HasPrefix(k, "kmb.") || !json.Valid([]byte(v)) {
			continue
		}
		res.UI[k] = v
		if strings.HasPrefix(k, "kmb.presets.") {
			var p struct {
				List []json.RawMessage `json:"list"`
			}
			if json.Unmarshal([]byte(v), &p) == nil {
				res.Presets += len(p.List)
			}
		}
	}
	return res, nil
}
