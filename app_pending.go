package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/downloader"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/queue"
)

// Downloads that were still waiting or running when the app closed are remembered and can
// be resumed at the next start. A batch keeps the link, the chosen items and the options;
// resuming reads the link again (fresh download addresses) and starts what is left.

type pendingBatch struct {
	ID      string             `json:"id"`
	URL     string             `json:"url"`
	Title   string             `json:"title"`
	Source  string             `json:"source"`
	Options downloader.Options `json:"options"`
	IDs     []string           `json:"ids"` // items not finished yet
	Added   int64              `json:"added"`
}

// PendingSummary is shown in the Download page's "resume" banner.
type PendingSummary struct {
	Batches int      `json:"batches"`
	Items   int      `json:"items"`
	Titles  []string `json:"titles"`
}

type pendingStore struct {
	mu       sync.Mutex
	path     string
	batches  map[string]*pendingBatch
	tasks    map[string][2]string // task id → batch id, entry id
	early    map[string]bool      // tasks that ended before add() saw them
	seq      int
	stopping bool // the app is closing: canceled tasks stay pending
}

func newPendingStore() *pendingStore {
	p := &pendingStore{path: filepath.Join(appdir.DataDir(), "pending-downloads.json"), batches: map[string]*pendingBatch{}, tasks: map[string][2]string{}, early: map[string]bool{}}
	if data, err := os.ReadFile(p.path); err == nil {
		data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // edited by hand in Notepad
		var list []*pendingBatch
		if json.Unmarshal(data, &list) == nil {
			for _, b := range list {
				if len(b.IDs) > 0 && b.URL != "" {
					p.batches[b.ID] = b
				}
			}
		}
	}
	return p
}

func (p *pendingStore) saveLocked() {
	list := make([]*pendingBatch, 0, len(p.batches))
	for _, b := range p.batches {
		if len(b.IDs) > 0 {
			list = append(list, b)
		}
	}
	if len(list) == 0 {
		_ = os.Remove(p.path)
		return
	}
	data, _ := json.MarshalIndent(list, "", " ")
	tmp := p.path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, p.path)
	}
}

// add records a started batch and the task of each item.
func (p *pendingStore) add(col *downloader.Collection, o downloader.Options, refs []JobRef) {
	if len(refs) == 0 || col.URL == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seq++
	b := &pendingBatch{ID: fmt.Sprintf("b%d-%d", time.Now().UnixNano(), p.seq), URL: col.URL, Title: col.Title, Source: col.Source, Options: o, Added: time.Now().Unix()}
	for _, r := range refs {
		if p.early[r.TaskID] {
			delete(p.early, r.TaskID)
			continue
		}
		b.IDs = append(b.IDs, r.ItemID)
		p.tasks[r.TaskID] = [2]string{b.ID, r.ItemID}
	}
	if len(b.IDs) == 0 {
		return
	}
	p.batches[b.ID] = b
	p.saveLocked()
}

// finished drops an item once its task has ended (for any reason but closing the app).
func (p *pendingStore) finished(info queue.Info) {
	if info.Kind != queue.KindDownload {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	ref, ok := p.tasks[info.ID]
	if !ok {
		if len(p.early) < 10000 {
			p.early[info.ID] = true
		}
		return
	}
	if p.stopping && info.Status == queue.StatusCanceled {
		return
	}
	delete(p.tasks, info.ID)
	if b := p.batches[ref[0]]; b != nil {
		for i, id := range b.IDs {
			if id == ref[1] {
				b.IDs = append(b.IDs[:i], b.IDs[i+1:]...)
				break
			}
		}
		if len(b.IDs) == 0 {
			delete(p.batches, b.ID)
		}
		p.saveLocked()
	}
}

func (p *pendingStore) stop() {
	p.mu.Lock()
	p.stopping = true
	p.mu.Unlock()
}

// take returns and forgets the batches left from an earlier session (not this one's).
func (p *pendingStore) take() []*pendingBatch {
	p.mu.Lock()
	defer p.mu.Unlock()
	live := map[string]bool{}
	for _, ref := range p.tasks {
		live[ref[0]] = true
	}
	var out []*pendingBatch
	for id, b := range p.batches {
		if !live[id] {
			out = append(out, b)
			delete(p.batches, id)
		}
	}
	p.saveLocked()
	return out
}

func (p *pendingStore) summary() PendingSummary {
	p.mu.Lock()
	defer p.mu.Unlock()
	live := map[string]bool{}
	for _, ref := range p.tasks {
		live[ref[0]] = true
	}
	var s PendingSummary
	for id, b := range p.batches {
		if live[id] {
			continue
		}
		s.Batches++
		s.Items += len(b.IDs)
		if len(s.Titles) < 5 {
			s.Titles = append(s.Titles, b.Title)
		}
	}
	return s
}

// PendingDownloads tells the Download page about downloads left unfinished last time.
func (a *App) PendingDownloads() PendingSummary { return a.pending.summary() }

// DiscardPendingDownloads forgets the unfinished downloads of the last session.
func (a *App) DiscardPendingDownloads() { a.pending.take() }

// ResumePendingDownloads reads the links of the last session again and queues the items
// that didn't finish. It returns how many items were queued.
func (a *App) ResumePendingDownloads() (int, error) {
	batches := a.pending.take()
	total := 0
	var firstErr error
	for _, b := range batches {
		col, err := a.AnalyzeLink(b.URL)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", b.Title, err)
			}
			continue
		}
		refs, err := a.StartDownloads(col.Key, b.IDs, b.Options)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", b.Title, err)
			}
			continue
		}
		total += len(refs)
	}
	if total == 0 && firstErr != nil {
		return 0, fmt.Errorf(i18n.L("Unduhan tidak bisa dilanjutkan: %w", "The downloads can't be resumed: %w"), firstErr)
	}
	return total, nil
}
