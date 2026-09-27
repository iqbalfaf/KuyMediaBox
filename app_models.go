package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"kuymediabox/internal/bgremove"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/tools"
	"kuymediabox/internal/whisper"
)

// AI models (speech recognition, background removal) are downloaded on demand, one file
// each, into the tools folder.

// ModelStatus is one model on the Subtitle or Image page.
type ModelStatus struct {
	Kind      string  `json:"kind"` // whisper | bgremove
	ID        string  `json:"id"`
	SizeMB    int     `json:"sizeMB"`
	Installed bool    `json:"installed"`
	Busy      bool    `json:"busy"`
	Progress  float64 `json:"progress"`
	Error     string  `json:"error"`
}

type modelDef struct {
	id, file, url string
	sizeMB        int
}

func modelDefs(kind string) []modelDef {
	var out []modelDef
	switch kind {
	case "whisper":
		for _, m := range whisper.Models {
			out = append(out, modelDef{m.ID, m.File, whisper.ModelURL(m), m.SizeMB})
		}
	case "bgremove":
		for _, m := range bgremove.Models {
			out = append(out, modelDef{m.ID, m.File, bgremove.ModelURL(m), m.SizeMB})
		}
	}
	return out
}

func findModel(kind, id string) (modelDef, bool) {
	for _, m := range modelDefs(kind) {
		if m.id == id {
			return m, true
		}
	}
	return modelDef{}, false
}

// modelPath is the file of a model, or "" when it is unknown.
func modelPath(kind, id string) string {
	m, ok := findModel(kind, id)
	if !ok {
		return ""
	}
	return filepath.Join(tools.ModelDir(kind), m.file)
}

func modelInstalled(kind string, m modelDef) bool {
	st, err := os.Stat(filepath.Join(tools.ModelDir(kind), m.file))
	return err == nil && st.Size() > int64(m.sizeMB)*1000*1000*85/100
}

type modelJobs struct {
	mu   sync.Mutex
	jobs map[string]*ModelStatus // kind/id → download in progress (or its last error)
	stop map[string]context.CancelFunc
}

var models = modelJobs{jobs: map[string]*ModelStatus{}, stop: map[string]context.CancelFunc{}}

// ListModels returns the models of a kind with their download state.
func (a *App) ListModels(kind string) []ModelStatus {
	models.mu.Lock()
	defer models.mu.Unlock()
	var out []ModelStatus
	for _, m := range modelDefs(kind) {
		st := ModelStatus{Kind: kind, ID: m.id, SizeMB: m.sizeMB, Installed: modelInstalled(kind, m)}
		if j := models.jobs[kind+"/"+m.id]; j != nil {
			st.Busy, st.Progress, st.Error = j.Busy, j.Progress, j.Error
		}
		out = append(out, st)
	}
	return out
}

func (a *App) modelsChanged(kind string) { a.emit("models:changed", a.ListModels(kind)) }

// InstallModel downloads a model; progress arrives through "models:changed".
func (a *App) InstallModel(kind, id string) error {
	m, ok := findModel(kind, id)
	if !ok {
		return errors.New(i18n.L("model tidak dikenal", "unknown model"))
	}
	key := kind + "/" + id
	models.mu.Lock()
	if j := models.jobs[key]; j != nil && j.Busy {
		models.mu.Unlock()
		return nil
	}
	job := &ModelStatus{Kind: kind, ID: id, Busy: true}
	models.jobs[key] = job
	ctx, cancel := context.WithCancel(context.Background())
	models.stop[key] = cancel
	models.mu.Unlock()
	a.modelsChanged(kind)

	err := tools.Download(ctx, m.url, filepath.Join(tools.ModelDir(kind), m.file), func(p float64) {
		models.mu.Lock()
		job.Progress = p
		models.mu.Unlock()
		a.modelsChanged(kind)
	})
	canceled := ctx.Err() != nil // read before cancel() below, which always sets it
	models.mu.Lock()
	job.Busy, job.Progress = false, 0
	delete(models.stop, key)
	if err != nil && !canceled {
		job.Error = err.Error()
	} else {
		delete(models.jobs, key)
	}
	models.mu.Unlock()
	cancel()
	a.modelsChanged(kind)
	if canceled {
		return nil
	}
	return err
}

// CancelModel stops a model download.
func (a *App) CancelModel(kind, id string) {
	models.mu.Lock()
	if stop := models.stop[kind+"/"+id]; stop != nil {
		stop()
	}
	models.mu.Unlock()
}

// DeleteModel removes a downloaded model file.
func (a *App) DeleteModel(kind, id string) error {
	p := modelPath(kind, id)
	if p == "" {
		return errors.New(i18n.L("model tidak dikenal", "unknown model"))
	}
	if kind == "bgremove" {
		a.resetRemover()
	}
	err := os.Remove(p)
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	a.modelsChanged(kind)
	return err
}
