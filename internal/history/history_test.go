package history

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestAddListRemove(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "h.jsonl"))
	for i := 1; i <= 3; i++ {
		if err := s.Add(Entry{ID: fmt.Sprintf("e%d", i), Time: int64(i), Status: "done"}); err != nil {
			t.Fatal(err)
		}
	}
	list := s.List(0)
	if len(list) != 3 || list[0].ID != "e3" || list[2].ID != "e1" {
		t.Fatalf("list %+v", list)
	}
	if got := s.List(2); len(got) != 2 {
		t.Fatalf("limit: %d", len(got))
	}
	if err := s.Remove([]string{"e2"}); err != nil {
		t.Fatal(err)
	}
	if list = s.List(0); len(list) != 2 || list[1].ID != "e1" {
		t.Fatalf("after remove %+v", list)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if len(s.List(0)) != 0 {
		t.Fatal("not cleared")
	}
}

func TestTrim(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "h.jsonl"))
	for i := 0; i < Keep*6/5+5; i++ {
		_ = s.Add(Entry{ID: fmt.Sprintf("e%d", i)})
	}
	if n := len(s.List(0)); n > Keep*6/5 {
		t.Fatalf("not trimmed: %d", n)
	}
}
