package queue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type collector struct {
	mu   sync.Mutex
	last map[string]Info
	idle chan [5]int
}

func newCollector() *collector {
	return &collector{last: map[string]Info{}, idle: make(chan [5]int, 4)}
}

func (c *collector) emit(i Info) {
	c.mu.Lock()
	c.last[i.ID] = i
	c.mu.Unlock()
}

func (c *collector) get(id string) Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last[id]
}

func TestStatusesAndIdle(t *testing.T) {
	c := newCollector()
	m := New(c.emit, func(kind string, d, f, s, x int) { c.idle <- [5]int{d, f, s, x, 0} })

	ids := m.AddMany(KindImage, []Spec{
		{"ok", func(ctx context.Context, r Reporter) error { r.Progress(0.5); return nil }},
		{"bad", func(ctx context.Context, r Reporter) error { return Fail("gagal", "detail") }},
		{"skip", func(ctx context.Context, r Reporter) error { return Skip("sudah ada") }},
		{"panic", func(ctx context.Context, r Reporter) error { panic("boom") }},
	})
	ok, bad, skip, panicky := ids[0], ids[1], ids[2], ids[3]

	select {
	case got := <-c.idle:
		if got != [5]int{1, 2, 1, 0, 0} {
			t.Fatalf("idle stats %v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("idle never fired")
	}
	if s := c.get(ok).Status; s != StatusDone {
		t.Errorf("ok: %s", s)
	}
	if i := c.get(bad); i.Status != StatusFailed || i.Message != "gagal" || i.Detail != "detail" {
		t.Errorf("bad: %+v", i)
	}
	if i := c.get(skip); i.Status != StatusSkipped || i.Message != "sudah ada" {
		t.Errorf("skip: %+v", i)
	}
	if s := c.get(panicky).Status; s != StatusFailed {
		t.Errorf("panic: %s", s)
	}
	if !errors.Is(Skip("x"), ErrSkipped) {
		t.Error("Skip must match ErrSkipped")
	}
}

func TestCancelRunningAndQueued(t *testing.T) {
	c := newCollector()
	m := New(c.emit, func(string, int, int, int, int) { c.idle <- [5]int{} })
	started := make(chan struct{})
	block := func(ctx context.Context, r Reporter) error {
		close(started)
		<-ctx.Done()
		return errors.New("killed")
	}
	a := m.Add(KindVideo, "a", block) // video limit is 1, so b waits in the queue
	b := m.Add(KindVideo, "b", func(ctx context.Context, r Reporter) error { return nil })
	<-started
	m.CancelKind(KindVideo)
	select {
	case <-c.idle:
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not finish tasks")
	}
	if s := c.get(a).Status; s != StatusCanceled {
		t.Errorf("running task: %s", s)
	}
	if s := c.get(b).Status; s != StatusCanceled {
		t.Errorf("queued task: %s", s)
	}
}

func TestFIFOOrder(t *testing.T) {
	m := New(nil, nil)
	var mu sync.Mutex
	var got []int
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		i := i
		wg.Add(1)
		m.Add(KindVideo, "x", func(ctx context.Context, r Reporter) error {
			defer wg.Done()
			mu.Lock()
			got = append(got, i)
			mu.Unlock()
			return nil
		})
	}
	wg.Wait()
	for i, v := range got {
		if v != i {
			t.Fatalf("tasks ran out of order: %v", got)
		}
	}
}

func TestCancelSingleQueued(t *testing.T) {
	c := newCollector()
	m := New(c.emit, nil)
	release := make(chan struct{})
	started := make(chan struct{})
	m.Add(KindVideo, "a", func(ctx context.Context, r Reporter) error { close(started); <-release; return nil })
	b := m.Add(KindVideo, "b", func(ctx context.Context, r Reporter) error { return nil })
	<-started
	m.Cancel(b)
	if s := c.get(b).Status; s != StatusCanceled {
		t.Fatalf("queued task should be canceled right away, got %s", s)
	}
	close(release)
}

func TestConcurrencyLimit(t *testing.T) {
	m := New(nil, nil)
	var mu sync.Mutex
	running, peak := 0, 0
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		m.Add(KindAudio, "x", func(ctx context.Context, r Reporter) error {
			defer wg.Done()
			mu.Lock()
			running++
			if running > peak {
				peak = running
			}
			mu.Unlock()
			time.Sleep(30 * time.Millisecond)
			mu.Lock()
			running--
			mu.Unlock()
			return nil
		})
	}
	wg.Wait()
	if peak > Limits[KindAudio] {
		t.Fatalf("peak %d exceeds limit %d", peak, Limits[KindAudio])
	}
}

// Updates are emitted after the task lock is released, so they can arrive out of order.
// The snapshot with the highest Seq must always be the final state.
func TestSeqOrdersUpdates(t *testing.T) {
	var mu sync.Mutex
	var all []Info
	m := New(func(i Info) {
		mu.Lock()
		all = append(all, i)
		mu.Unlock()
	}, nil)
	for round := 0; round < 20; round++ {
		all = nil
		id := m.Add(KindImage, "x", func(ctx context.Context, r Reporter) error {
			var wg sync.WaitGroup
			for g := 0; g < 8; g++ {
				wg.Add(1)
				go func(g int) {
					defer wg.Done()
					for i := 0; i < 50; i++ {
						r.Progress(float64(i) / 50)
						r.Message(string(rune('a' + (g+i)%26)))
					}
				}(g)
			}
			wg.Wait()
			return nil
		})
		deadline := time.Now().Add(5 * time.Second)
		for {
			if info, _ := m.Get(id); info.Status == StatusDone || time.Now().After(deadline) {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		mu.Lock()
		seen := map[uint64]bool{}
		var best Info
		for _, i := range all {
			if i.ID != id {
				continue
			}
			if seen[i.Seq] {
				t.Fatalf("round %d: duplicate seq %d", round, i.Seq)
			}
			seen[i.Seq] = true
			if i.Seq > best.Seq {
				best = i
			}
		}
		mu.Unlock()
		if best.Status != StatusDone {
			t.Fatalf("round %d: highest seq %d has status %s", round, best.Seq, best.Status)
		}
		if snap, _ := m.Get(id); snap.Seq != best.Seq {
			t.Fatalf("round %d: snapshot seq %d, last emitted %d", round, snap.Seq, best.Seq)
		}
	}
}
