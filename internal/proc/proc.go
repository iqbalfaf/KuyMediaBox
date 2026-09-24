// Package proc runs external tools (ffmpeg, yt-dlp, spotDL) hidden, cancellable and with
// their whole process tree killed on cancel.
package proc

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"kuymediabox/internal/platform"
)

// Command builds a hidden, tree-killing command bound to ctx.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	platform.HideWindow(cmd)
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if err := platform.KillTree(cmd.Process.Pid); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	cmd.WaitDelay = 5 * time.Second
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1", "NO_COLOR=1", "TERM=dumb")
	return cmd
}

// Output runs a command and returns trimmed stdout (stderr appended on failure).
func Output(ctx context.Context, name string, args ...string) (string, error) {
	cmd := Command(ctx, name, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if msg != "" {
			return "", errors.New(LastLines(msg, 6))
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Tail keeps the last N lines written to it.
type Tail struct {
	mu    sync.Mutex
	lines []string
	max   int
}

// NewTail keeps up to max lines.
func NewTail(max int) *Tail { return &Tail{max: max} }

// Add stores one line.
func (t *Tail) Add(line string) {
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lines = append(t.lines, line)
	if len(t.lines) > t.max {
		t.lines = t.lines[len(t.lines)-t.max:]
	}
}

// String returns the kept lines joined by newlines.
func (t *Tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.Join(t.lines, "\n")
}

// Lines returns a copy of the kept lines.
func (t *Tail) Lines() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.lines...)
}

// ScanLines calls fn for every line (split on \n or \r) read from r until EOF.
func ScanLines(r io.Reader, fn func(string)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	sc.Split(splitCRLF)
	for sc.Scan() {
		fn(sc.Text())
	}
	// Drain anything left so the child never blocks on a full pipe.
	_, _ = io.Copy(io.Discard, r)
}

func splitCRLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// LastLines returns the last n non-empty lines of s.
func LastLines(s string, n int) string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r", "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimSpace(l))
		}
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return strings.Join(out, "\n")
}
