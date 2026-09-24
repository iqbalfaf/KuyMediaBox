// Package queue runs conversion and download tasks with a per-kind concurrency limit,
// reports progress through a callback and supports cancelling single tasks or whole kinds.
package queue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"kuymediabox/internal/i18n"
)

// Task kinds.
const (
	KindImage    = "image"
	KindVideo    = "video"
	KindAudio    = "audio"
	KindDownload = "download"
	KindPDF      = "pdf"
)

// Task statuses.
const (
	StatusQueued   = "queued"
	StatusRunning  = "running"
	StatusDone     = "done"
	StatusFailed   = "failed"
	StatusCanceled = "canceled"
	StatusSkipped  = "skipped"
)

// ErrSkipped marks a task that ended without work (file already exists, already downloaded).
var ErrSkipped = errors.New("skipped")

// SkipError carries a human message for a skipped task.
type SkipError struct{ Reason string }

func (e *SkipError) Error() string        { return e.Reason }
func (e *SkipError) Is(target error) bool { return target == ErrSkipped }

// Skip returns an error that marks the task as skipped with reason.
func Skip(reason string) error { return &SkipError{Reason: reason} }

// UserError is a failure with a friendly message plus the raw tool output as detail.
type UserError struct {
	Message string
	Detail  string
}

func (e *UserError) Error() string { return e.Message }

// Fail builds a UserError.
func Fail(message, detail string) error { return &UserError{Message: message, Detail: detail} }

// Info is the public, JSON-friendly view of a task.
type Info struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Title    string  `json:"title"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"` // 0..1, or -1 when unknown
	Message  string  `json:"message"`
	Detail   string  `json:"detail"`
	Output   string  `json:"output"`
	OutSize  int64   `json:"outSize"`
	Started  int64   `json:"started"`  // unix ms
	Finished int64   `json:"finished"` // unix ms
	// Seq grows with every published update of the task. Events can reach the UI out of order
	// (they are sent after the lock is released), so the UI keeps the snapshot with the highest Seq.
	Seq uint64 `json:"seq"`
}

// Reporter lets a running task publish progress.
type Reporter interface {
	Progress(p float64)
	Message(msg string)
	SetOutput(path string, size int64)
}

// RunFunc does the work of a task.
type RunFunc func(ctx context.Context, r Reporter) error

type task struct {
	mu      sync.Mutex
	info    Info
	run     RunFunc
	cancel  context.CancelFunc
	lastEmt time.Time
	stopReq atomic.Bool // set before cancel so a queued task can never start after a cancel
}

func (t *task) stop() {
	t.stopReq.Store(true)
	t.cancel()
}

// Manager owns all tasks.
type Manager struct {
	mu     sync.Mutex
	tasks  map[string]*task
	order  []string
	kinds  map[string]*kindState
	seq    atomic.Int64
	emit   func(Info)
	onIdle func(kind string, done, failed, skipped, canceled int)
	batch  map[string]*batchStats
	root   context.Context
	stop   context.CancelFunc
}

// kindState is the FIFO line of waiting tasks plus the number running, per kind.
type kindState struct {
	running int
	pending []*task
	ctxs    map[*task]context.Context
}

type batchStats struct{ active, done, failed, skipped, canceled int }

// Limits holds max parallel tasks per kind.
var Limits = map[string]int{KindImage: 3, KindVideo: 1, KindAudio: 2, KindDownload: 2, KindPDF: 2}

// New creates a manager. emit receives every change; onIdle fires when a kind has no more
// queued or running tasks after having some.
func New(emit func(Info), onIdle func(kind string, done, failed, skipped, canceled int)) *Manager {
	ctx, stop := context.WithCancel(context.Background())
	return &Manager{
		tasks:  map[string]*task{},
		kinds:  map[string]*kindState{},
		emit:   emit,
		onIdle: onIdle,
		batch:  map[string]*batchStats{},
		root:   ctx,
		stop:   stop,
	}
}

func (m *Manager) state(kind string) *kindState {
	ks := m.kinds[kind]
	if ks == nil {
		ks = &kindState{ctxs: map[*task]context.Context{}}
		m.kinds[kind] = ks
	}
	return ks
}

// Spec describes one task for AddMany.
type Spec struct {
	Title string
	Run   RunFunc
}

// Add queues a task and returns its ID. Tasks of one kind start in the order they were added.
func (m *Manager) Add(kind, title string, run RunFunc) string {
	return m.AddMany(kind, []Spec{{Title: title, Run: run}})[0]
}

// AddMany queues several tasks atomically, so a batch is never reported finished halfway.
func (m *Manager) AddMany(kind string, specs []Spec) []string {
	ids := make([]string, len(specs))
	list := make([]*task, len(specs))
	m.mu.Lock()
	ks := m.state(kind)
	bs := m.batch[kind]
	if bs == nil || bs.active == 0 {
		bs = &batchStats{}
		m.batch[kind] = bs
	}
	for i, s := range specs {
		id := fmt.Sprintf("t%d", m.seq.Add(1))
		ctx, cancel := context.WithCancel(m.root)
		t := &task{info: Info{ID: id, Kind: kind, Title: s.Title, Status: StatusQueued}, run: s.Run, cancel: cancel}
		m.tasks[id] = t
		m.order = append(m.order, id)
		ks.pending = append(ks.pending, t)
		ks.ctxs[t] = ctx
		bs.active++
		ids[i] = id
		list[i] = t
	}
	m.mu.Unlock()

	for _, t := range list {
		m.publish(t, true)
	}
	m.schedule(kind)
	return ids
}

// schedule starts waiting tasks while the kind has free slots; stopped ones are finished.
func (m *Manager) schedule(kind string) {
	limit := Limits[kind]
	if limit <= 0 {
		limit = 1
	}
	var start []*task
	var startCtx []context.Context
	var dropped []*task
	m.mu.Lock()
	ks := m.state(kind)
	for len(ks.pending) > 0 && ks.running < limit {
		t := ks.pending[0]
		ks.pending = ks.pending[1:]
		ctx := ks.ctxs[t]
		delete(ks.ctxs, t)
		if t.stopReq.Load() || ctx.Err() != nil {
			dropped = append(dropped, t)
			continue
		}
		ks.running++
		start = append(start, t)
		startCtx = append(startCtx, ctx)
	}
	m.mu.Unlock()
	for _, t := range dropped {
		m.finish(t, context.Canceled)
	}
	for i, t := range start {
		go m.exec(startCtx[i], t)
	}
}

// dropQueued finishes waiting tasks that were stopped, without waiting for a free slot.
func (m *Manager) dropQueued(kind string) {
	var dropped []*task
	m.mu.Lock()
	ks := m.state(kind)
	kept := ks.pending[:0]
	for _, t := range ks.pending {
		if t.stopReq.Load() {
			dropped = append(dropped, t)
			delete(ks.ctxs, t)
		} else {
			kept = append(kept, t)
		}
	}
	ks.pending = kept
	m.mu.Unlock()
	for _, t := range dropped {
		m.finish(t, context.Canceled)
	}
}

func (m *Manager) exec(ctx context.Context, t *task) {
	kind := t.info.Kind
	defer func() {
		m.mu.Lock()
		m.state(kind).running--
		m.mu.Unlock()
		m.schedule(kind)
	}()
	t.mu.Lock()
	t.info.Status = StatusRunning
	t.info.Started = time.Now().UnixMilli()
	t.info.Progress = 0
	t.mu.Unlock()
	m.publish(t, true)

	var err error
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				err = Fail(i18n.L("Terjadi kesalahan internal", "Internal error"), fmt.Sprint(rec))
			}
		}()
		err = t.run(ctx, &reporter{m: m, t: t})
	}()
	if err != nil && ctx.Err() != nil {
		err = context.Canceled
	}
	m.finish(t, err)
}

func (m *Manager) finish(t *task, err error) {
	t.mu.Lock()
	t.info.Finished = time.Now().UnixMilli()
	var ue *UserError
	var se *SkipError
	switch {
	case err == nil:
		t.info.Status = StatusDone
		t.info.Progress = 1
	case errors.Is(err, context.Canceled):
		t.info.Status = StatusCanceled
		t.info.Message = i18n.L("Dibatalkan", "Canceled")
	case errors.As(err, &se):
		t.info.Status = StatusSkipped
		t.info.Progress = 1
		t.info.Message = se.Reason
	case errors.As(err, &ue):
		t.info.Status = StatusFailed
		t.info.Message = ue.Message
		t.info.Detail = ue.Detail
	default:
		t.info.Status = StatusFailed
		t.info.Message = firstLine(err.Error())
		t.info.Detail = err.Error()
	}
	status := t.info.Status
	kind := t.info.Kind
	t.mu.Unlock()
	t.cancel()
	m.publish(t, true)

	m.mu.Lock()
	bs := m.batch[kind]
	var fire bool
	var s batchStats
	if bs != nil {
		switch status {
		case StatusDone:
			bs.done++
		case StatusFailed:
			bs.failed++
		case StatusSkipped:
			bs.skipped++
		case StatusCanceled:
			bs.canceled++
		}
		bs.active--
		if bs.active <= 0 {
			fire = true
			s = *bs
			delete(m.batch, kind)
		}
	}
	m.mu.Unlock()
	if fire && m.onIdle != nil {
		m.onIdle(kind, s.done, s.failed, s.skipped, s.canceled)
	}
}

func (m *Manager) publish(t *task, force bool) {
	t.mu.Lock()
	now := time.Now()
	if !force && now.Sub(t.lastEmt) < 150*time.Millisecond {
		t.mu.Unlock()
		return
	}
	t.lastEmt = now
	t.info.Seq++
	info := t.info
	t.mu.Unlock()
	if m.emit != nil {
		m.emit(info)
	}
}

// Cancel stops one task.
func (m *Manager) Cancel(id string) {
	m.mu.Lock()
	t := m.tasks[id]
	m.mu.Unlock()
	if t != nil {
		t.stop()
		m.dropQueued(t.info.Kind)
	}
}

// CancelKind stops every queued or running task of a kind.
func (m *Manager) CancelKind(kind string) {
	m.mu.Lock()
	var list []*task
	for _, t := range m.tasks {
		if t.info.Kind == kind {
			list = append(list, t)
		}
	}
	m.mu.Unlock()
	// Flag everything first, then cancel, so no queued task slips into a freed slot.
	for _, t := range list {
		t.stopReq.Store(true)
	}
	for _, t := range list {
		t.cancel()
	}
	m.dropQueued(kind)
}

// Shutdown cancels everything (used when the app closes).
func (m *Manager) Shutdown() {
	m.mu.Lock()
	var kinds []string
	for _, t := range m.tasks {
		t.stopReq.Store(true)
	}
	for k := range m.kinds {
		kinds = append(kinds, k)
	}
	m.mu.Unlock()
	m.stop()
	for _, k := range kinds {
		m.dropQueued(k)
	}
}

// Get returns a task snapshot.
func (m *Manager) Get(id string) (Info, bool) {
	m.mu.Lock()
	t := m.tasks[id]
	m.mu.Unlock()
	if t == nil {
		return Info{}, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.info, true
}

// List returns all tasks in creation order.
func (m *Manager) List() []Info {
	m.mu.Lock()
	ids := append([]string(nil), m.order...)
	m.mu.Unlock()
	out := make([]Info, 0, len(ids))
	for _, id := range ids {
		if info, ok := m.Get(id); ok {
			out = append(out, info)
		}
	}
	return out
}

// Forget drops finished tasks from memory.
func (m *Manager) Forget(ids []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	drop := map[string]bool{}
	for _, id := range ids {
		if t := m.tasks[id]; t != nil {
			t.mu.Lock()
			st := t.info.Status
			t.mu.Unlock()
			if st != StatusQueued && st != StatusRunning {
				drop[id] = true
				delete(m.tasks, id)
			}
		}
	}
	kept := m.order[:0]
	for _, id := range m.order {
		if !drop[id] {
			kept = append(kept, id)
		}
	}
	m.order = kept
}

type reporter struct {
	m *Manager
	t *task
}

func (r *reporter) Progress(p float64) {
	if p > 1 {
		p = 1
	}
	r.t.mu.Lock()
	r.t.info.Progress = p
	r.t.mu.Unlock()
	r.m.publish(r.t, false)
}

func (r *reporter) Message(msg string) {
	r.t.mu.Lock()
	changed := r.t.info.Message != msg
	r.t.info.Message = msg
	r.t.mu.Unlock()
	r.m.publish(r.t, changed && !strings.Contains(msg, "/s"))
}

func (r *reporter) SetOutput(path string, size int64) {
	r.t.mu.Lock()
	r.t.info.Output = path
	r.t.info.OutSize = size
	r.t.mu.Unlock()
	r.m.publish(r.t, true)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
