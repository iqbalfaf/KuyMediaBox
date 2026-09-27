package main

import (
	"testing"

	"kuymediabox/internal/downloader"
	"kuymediabox/internal/queue"
)

func TestPendingDownloads(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	p := newPendingStore()
	col := &downloader.Collection{URL: "https://www.youtube.com/playlist?list=PLx", Title: "Mix", Source: "youtube"}
	// t0 ends before add() sees it (e.g. "already downloaded").
	p.finished(queue.Info{ID: "t0", Kind: queue.KindDownload, Status: queue.StatusSkipped})
	p.add(col, downloader.Options{Mode: "audio"}, []JobRef{{"a", "t0"}, {"b", "t1"}, {"c", "t2"}, {"d", "t3"}})
	p.finished(queue.Info{ID: "t1", Kind: queue.KindDownload, Status: queue.StatusDone})
	// This session's own batch isn't offered for resuming.
	if s := p.summary(); s.Items != 0 {
		t.Fatalf("live batch offered: %+v", s)
	}
	// Closing the app cancels t2 and t3: they stay pending.
	p.stop()
	p.finished(queue.Info{ID: "t2", Kind: queue.KindDownload, Status: queue.StatusCanceled})
	p.finished(queue.Info{ID: "t3", Kind: queue.KindDownload, Status: queue.StatusCanceled})

	next := newPendingStore() // the next start
	s := next.summary()
	if s.Batches != 1 || s.Items != 2 || s.Titles[0] != "Mix" {
		t.Fatalf("summary %+v", s)
	}
	got := next.take()
	if len(got) != 1 || got[0].Options.Mode != "audio" || len(got[0].IDs) != 2 || got[0].IDs[0] != "c" || got[0].IDs[1] != "d" {
		t.Fatalf("take %+v", got)
	}
	if s := newPendingStore().summary(); s.Items != 0 {
		t.Fatalf("taken batches must be forgotten: %+v", s)
	}
}
