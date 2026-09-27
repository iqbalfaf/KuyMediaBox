package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/config"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/naming"
	"kuymediabox/internal/queue"
)

// Workflows: every file goes through the steps of a workflow one after another; the result
// of a step is the input of the next ("download → MP3 → even volume"). In-between results
// live in a temp folder removed when the file is done. The last step saves where its module
// normally saves; "same folder" outputs go next to the original file.

type flowRun struct {
	id      string
	wf      config.Workflow
	orig    string   // the original file (the downloaded file for download workflows)
	step    int      // step being run; -1 while the download is still running
	dir     string   // temp folder for in-between results
	pending int      // tasks of the current step still running
	next    []string // results of the current step
	failed  int      // tasks of the current step that failed
}

type flowState struct {
	hook  func(FlowResult) // command line: told about every finished file
	mu    sync.Mutex
	seq   int
	tasks map[string]*flowRun    // task id → run
	early map[string]earlyFinish // tasks that finished before their run was registered
	// moving counts runs between two steps: their last task is gone, the next step's tasks
	// aren't queued yet. The command line must not think everything is done meanwhile.
	moving int
}

// flowsBusy reports whether a workflow still has work queued or about to be queued.
func (a *App) flowsBusy() bool {
	a.flows.mu.Lock()
	defer a.flows.mu.Unlock()
	return len(a.flows.tasks) > 0 || a.flows.moving > 0
}

type earlyFinish struct {
	info queue.Info
	at   time.Time
}

// FlowResult is sent to the frontend when a file has gone through a workflow.
type FlowResult struct {
	Workflow string `json:"workflow"`
	Input    string `json:"input"`
	Output   string `json:"output"` // "" when a step failed
	Error    string `json:"error"`
}

func (a *App) workflow(id string) (config.Workflow, error) {
	for _, w := range a.cfg.Get().Workflows {
		if w.ID == id || strings.EqualFold(w.Name, id) {
			if len(w.Steps) == 0 {
				return w, fmt.Errorf(i18n.L("Alur kerja \"%s\" belum punya langkah", "Workflow \"%s\" has no steps"), w.Name)
			}
			return w, nil
		}
	}
	return config.Workflow{}, fmt.Errorf(i18n.L("Alur kerja \"%s\" tidak ditemukan", "Workflow \"%s\" not found"), id)
}

func (a *App) newRun(wf config.Workflow, orig string) *flowRun {
	a.flows.mu.Lock()
	a.flows.seq++
	id := fmt.Sprintf("f%d-%d", time.Now().UnixMilli(), a.flows.seq)
	a.flows.mu.Unlock()
	return &flowRun{id: id, wf: wf, orig: orig, dir: filepath.Join(appdir.TempDir(), "flow", id)}
}

// RunWorkflow sends files through a workflow; returns how many were started.
func (a *App) RunWorkflow(id string, paths []string) (int, error) {
	wf, err := a.workflow(id)
	if err != nil {
		return 0, err
	}
	started := 0
	var firstErr error
	for _, p := range paths {
		run := a.newRun(wf, p)
		if err := a.startStep(run, []string{p}); err != nil {
			a.finishRun(run, "", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		started++
	}
	if started == 0 && firstErr != nil {
		return 0, firstErr
	}
	return started, nil
}

// flowAfterDownload runs a workflow on each file of these download tasks once it is saved.
func (a *App) flowAfterDownload(id string, taskIDs []string) error {
	wf, err := a.workflow(id)
	if err != nil {
		return err
	}
	for _, t := range taskIDs {
		run := a.newRun(wf, "")
		run.step = -1
		a.track(run, []string{t})
	}
	return nil
}

// startStep queues the current step of a run for inputs.
func (a *App) startStep(run *flowRun, inputs []string) error {
	step := run.wf.Steps[run.step]
	final := run.step == len(run.wf.Steps)-1
	outDir, plain := "", false
	if final {
		outDir = a.finalDir(step.Kind, run.orig)
	} else {
		outDir, plain = filepath.Join(run.dir, strconv.Itoa(run.step)), true
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}
	}
	ids, err := a.startFlowStep(step, inputs, outDir, plain)
	if err != nil {
		return fmt.Errorf("%s (%s %d): %w", run.wf.Name, i18n.L("langkah", "step"), run.step+1, err)
	}
	a.track(run, ids)
	return nil
}

// finalDir is where the last step saves: next to the original file when its module saves
// "in the same folder" (or a subfolder of it), else "" for the module's own folder.
func (a *App) finalDir(kind, orig string) string {
	spec := a.outputSpec(kind)
	if orig == "" || (spec.Mode != config.OutputSame && spec.Mode != config.OutputSubfolder) {
		return ""
	}
	dir, err := naming.OutputDir(orig, spec)
	if err != nil {
		return ""
	}
	return dir
}

// startFlowStep starts one module for inputs; returns the task ids.
func (a *App) startFlowStep(step config.FlowStep, inputs []string, outDir string, plain bool) ([]string, error) {
	items := make([]JobItem, len(inputs))
	for i, p := range inputs {
		items[i] = JobItem{ID: nextItemID(), Path: p, OutDir: outDir, Plain: plain}
	}
	job := step.Job
	if len(job) == 0 {
		job = json.RawMessage("{}")
	}
	var refs []JobRef
	var err error
	switch step.Kind {
	case queue.KindImage:
		var o imageconv.Options
		if err = json.Unmarshal(job, &o); err == nil {
			refs, err = a.StartImage(items, o)
		}
	case queue.KindVideo:
		var j VideoJob
		if err = json.Unmarshal(job, &j); err == nil {
			if j.Mode == "merge" || j.Mode == "" {
				j.Mode = "video"
			}
			refs, err = a.StartVideo(items, j)
		}
	case queue.KindAudio:
		var j AudioJob
		if err = json.Unmarshal(job, &j); err == nil {
			j.Mode = "convert"
			refs, err = a.StartAudio(items, j)
		}
	case queue.KindSubtitle:
		var j SubtitleJob
		if err = json.Unmarshal(job, &j); err == nil {
			refs, err = a.StartSubtitle(items, j)
		}
	case kindPDF:
		if step.Tool == "" || step.Tool == "digisign" {
			return nil, errors.New(i18n.L("Alat PDF ini tidak bisa dipakai di alur kerja", "This PDF tool can't be used in a workflow"))
		}
		var o PdfOptions
		if err = json.Unmarshal(job, &o); err == nil {
			jobs := make([]PdfJob, len(items))
			for i, it := range items {
				jobs[i] = PdfJob{ID: it.ID, Path: it.Path, OutDir: outDir}
			}
			refs, err = a.StartPdf(step.Tool, jobs, o, "")
		}
	default:
		return nil, fmt.Errorf("%s: %s", i18n.L("Langkah tidak dikenal", "Unknown step"), step.Kind)
	}
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(refs))
	for i, r := range refs {
		ids[i] = r.TaskID
	}
	return ids, nil
}

// track waits for tasks of a run's current step.
func (a *App) track(run *flowRun, ids []string) {
	a.flows.mu.Lock()
	if a.flows.tasks == nil {
		a.flows.tasks = map[string]*flowRun{}
	}
	run.pending += len(ids)
	var done []queue.Info
	for _, id := range ids {
		a.flows.tasks[id] = run
		if e, ok := a.flows.early[id]; ok {
			delete(a.flows.early, id)
			done = append(done, e.info)
		}
	}
	a.flows.mu.Unlock()
	for _, info := range done {
		a.onFlowTask(info)
	}
}

// onFlowTask moves a run on when a task of it finishes (queue.OnFinish).
func (a *App) onFlowTask(info queue.Info) {
	a.flows.mu.Lock()
	run := a.flows.tasks[info.ID]
	if run == nil {
		// Maybe a task whose run registers in a moment; keep it briefly.
		if a.flows.early == nil {
			a.flows.early = map[string]earlyFinish{}
		}
		now := time.Now()
		for id, e := range a.flows.early {
			if now.Sub(e.at) > 30*time.Second {
				delete(a.flows.early, id)
			}
		}
		a.flows.early[info.ID] = earlyFinish{info, now}
		a.flows.mu.Unlock()
		return
	}
	delete(a.flows.tasks, info.ID)
	ok := info.Status == queue.StatusDone || (info.Status == queue.StatusSkipped && info.Output != "" && exists(info.Output))
	if ok {
		run.next = append(run.next, outputsOf(info.Output)...)
	} else {
		run.failed++
	}
	run.pending--
	if run.pending > 0 {
		a.flows.mu.Unlock()
		return
	}
	next := run.next
	run.next = nil
	failed := run.failed
	run.failed = 0
	last := run.step == len(run.wf.Steps)-1
	advancing := len(next) > 0 && !last
	if advancing {
		a.flows.moving++
	}
	a.flows.mu.Unlock()
	if advancing {
		defer func() {
			a.flows.mu.Lock()
			a.flows.moving--
			a.flows.mu.Unlock()
		}()
	}

	switch {
	case len(next) == 0:
		msg := info.Message
		if msg == "" {
			msg = i18n.L("Langkah gagal", "The step failed")
		}
		a.finishRun(run, "", errors.New(msg))
	case last:
		var err error
		if failed > 0 {
			err = fmt.Errorf(i18n.L("%d file gagal di langkah terakhir", "%d files failed in the last step"), failed)
		}
		a.finishRun(run, next[0], err)
	default:
		if run.step < 0 {
			run.orig = next[0] // downloaded file
		}
		run.step++
		if err := a.startStep(run, next); err != nil {
			a.finishRun(run, "", err)
		}
	}
}

func (a *App) finishRun(run *flowRun, output string, err error) {
	_ = os.RemoveAll(run.dir)
	res := FlowResult{Workflow: run.wf.Name, Input: run.orig, Output: output}
	if err != nil {
		res.Error = err.Error()
	}
	a.emit("flow:done", res)
	if a.flows.hook != nil {
		a.flows.hook(res)
	}
}

// inFlowTemp reports whether a path is an in-between workflow result (not kept, so not
// worth a history entry).
func inFlowTemp(p string) bool {
	root := strings.ToLower(filepath.Clean(filepath.Join(appdir.TempDir(), "flow"))) + string(filepath.Separator)
	return strings.HasPrefix(strings.ToLower(filepath.Clean(p)), root)
}

// outputsOf lists a task's results: the file, or the files of a result folder.
func outputsOf(p string) []string {
	st, err := os.Stat(p)
	if err != nil {
		return nil
	}
	if !st.IsDir() {
		return []string{p}
	}
	entries, _ := os.ReadDir(p)
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, filepath.Join(p, e.Name()))
		}
	}
	return out
}
