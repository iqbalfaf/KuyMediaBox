package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/i18n"
)

// Recipes are the presets and current settings of the converter pages, resolved into the
// jobs the backend runs. The pages keep them in the WebView's storage, so the frontend
// mirrors them here for workflows and the command line.

// Recipe is one preset of a module ("current" = the page's settings).
type Recipe struct {
	Kind    string          `json:"kind"` // image | video | audio | subtitle
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Builtin bool            `json:"builtin"`
	Job     json.RawMessage `json:"job"`
}

var recipeMu sync.Mutex

func recipePath() string { return filepath.Join(appdir.DataDir(), "presets.json") }

// SyncRecipes stores the frontend's presets for workflows and the command line.
func (a *App) SyncRecipes(list []Recipe) error {
	b, err := json.MarshalIndent(list, "", " ")
	if err != nil {
		return err
	}
	recipeMu.Lock()
	defer recipeMu.Unlock()
	tmp := recipePath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, recipePath())
}

func loadRecipes() ([]Recipe, error) {
	recipeMu.Lock()
	b, err := os.ReadFile(recipePath())
	recipeMu.Unlock()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list []Recipe
	err = json.Unmarshal(bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF}), &list)
	return list, err
}

// findRecipe picks a preset by name: exact (ignoring case), then a unique prefix, then a
// unique part of the name. kind "" searches every module; name "" is the page settings.
func findRecipe(list []Recipe, kind, name string) (Recipe, error) {
	var pool []Recipe
	for _, r := range list {
		if kind == "" || r.Kind == kind {
			pool = append(pool, r)
		}
	}
	if len(pool) == 0 {
		return Recipe{}, errors.New(i18n.L("Belum ada preset. Buka KuyMediaBox sekali supaya preset tersedia.", "No presets yet. Open KuyMediaBox once so the presets are available."))
	}
	want := strings.ToLower(strings.TrimSpace(name))
	if want == "" {
		for _, r := range pool {
			if r.ID == "current" {
				return r, nil
			}
		}
	}
	for _, match := range []func(string) bool{
		func(n string) bool { return n == want },
		func(n string) bool { return strings.HasPrefix(n, want) },
		func(n string) bool { return strings.Contains(n, want) },
	} {
		var hits []Recipe
		for _, r := range pool {
			if r.ID != "current" && match(strings.ToLower(r.Name)) {
				hits = append(hits, r)
			}
		}
		if len(hits) == 1 {
			return hits[0], nil
		}
		if len(hits) > 1 {
			names := make([]string, len(hits))
			for i, h := range hits {
				names[i] = fmt.Sprintf("%s (%s)", h.Name, h.Kind)
			}
			sort.Strings(names)
			return Recipe{}, fmt.Errorf(i18n.L("Nama preset \"%s\" cocok dengan beberapa: %s", "Preset name \"%s\" matches several: %s"), name, strings.Join(names, "; "))
		}
	}
	return Recipe{}, fmt.Errorf(i18n.L("Preset \"%s\" tidak ditemukan", "Preset \"%s\" not found"), name)
}
