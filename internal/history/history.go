// Package history keeps a log of finished tasks as JSON lines in %LOCALAPPDATA%\KuyMediaBox.
package history

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Entry is one finished task.
type Entry struct {
	ID      string `json:"id"`
	Time    int64  `json:"time"` // finished, unix ms
	Started int64  `json:"started"`
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Input   string `json:"input"`
	Output  string `json:"output"`
	InSize  int64  `json:"inSize"`
	OutSize int64  `json:"outSize"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

// Keep is how many entries survive a trim; the file is trimmed when it grows past Keep*1.2.
const Keep = 5000

// Store appends entries to a file and reads them back.
type Store struct {
	mu    sync.Mutex
	path  string
	count int // lines in the file, -1 until counted
}

// Open uses path (created on first write).
func Open(path string) *Store { return &Store{path: path, count: -1} }

// Default opens history.jsonl in dir.
func Default(dir string) *Store { return Open(filepath.Join(dir, "history.jsonl")) }

// Add appends an entry.
func (s *Store) Add(e Entry) error {
	if len(e.Detail) > 4000 {
		e.Detail = e.Detail[len(e.Detail)-4000:]
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.count < 0 {
		s.count = len(s.readLocked())
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	s.count++
	if s.count > Keep*6/5 {
		list := s.readLocked()
		if len(list) > Keep {
			list = list[len(list)-Keep:]
		}
		err = s.writeLocked(list)
	}
	return err
}

// List returns the entries newest first (at most limit when limit > 0).
func (s *Store) List(limit int) []Entry {
	s.mu.Lock()
	list := s.readLocked()
	s.mu.Unlock()
	out := make([]Entry, 0, len(list))
	for i := len(list) - 1; i >= 0; i-- {
		out = append(out, list[i])
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// Remove deletes entries by id.
func (s *Store) Remove(ids []string) error {
	drop := map[string]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var kept []Entry
	for _, e := range s.readLocked() {
		if !drop[e.ID] {
			kept = append(kept, e)
		}
	}
	return s.writeLocked(kept)
}

// Clear deletes every entry.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count = 0
	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *Store) readLocked() []Entry {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}
	var out []Entry
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.ID != "" {
			out = append(out, e)
		}
	}
	return out
}

func (s *Store) writeLocked(list []Entry) error {
	var buf bytes.Buffer
	for _, e := range list {
		line, err := json.Marshal(e)
		if err != nil {
			continue
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	s.count = len(list)
	return os.Rename(tmp, s.path)
}
