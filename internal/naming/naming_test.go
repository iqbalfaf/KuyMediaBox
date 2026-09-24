package naming

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"kuymediabox/internal/config"
)

func touch(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSubfolderDefault(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "foto.png")
	touch(t, in)
	n := NewNamer()
	got, release, err := n.Reserve(in, OutputSpec{Mode: config.OutputSubfolder}, "_converted", "jpg", config.ConflictRename)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	want := filepath.Join(dir, "converted", "foto_converted.jpg")
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestNeverOverwritesSource(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "lagu.mp3")
	touch(t, in)
	n := NewNamer()
	got, release, err := n.Reserve(in, OutputSpec{Mode: config.OutputSame}, "", "mp3", config.ConflictOverwrite)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if got == in {
		t.Fatal("source would be overwritten")
	}
	if got != filepath.Join(dir, "lagu (1).mp3") {
		t.Fatalf("unexpected %s", got)
	}
}

func TestConflictPolicies(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "a.png")
	touch(t, in)
	existing := filepath.Join(dir, "a_x.jpg")
	touch(t, existing)
	n := NewNamer()
	spec := OutputSpec{Mode: config.OutputSame}

	got, rel, err := n.Reserve(in, spec, "_x", "jpg", config.ConflictRename)
	if err != nil || got != filepath.Join(dir, "a_x (1).jpg") {
		t.Fatalf("rename: %s %v", got, err)
	}
	rel()

	_, _, err = n.Reserve(in, spec, "_x", "jpg", config.ConflictSkip)
	if !errors.Is(err, ErrExists) {
		t.Fatalf("skip: want ErrExists, got %v", err)
	}

	got, rel, err = n.Reserve(in, spec, "_x", "jpg", config.ConflictOverwrite)
	if err != nil || got != existing {
		t.Fatalf("overwrite: %s %v", got, err)
	}
	rel()
}

func TestReservedNamesAreUnique(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "x.png")
	b := filepath.Join(dir, "x.jpg")
	touch(t, a)
	touch(t, b)
	n := NewNamer()
	spec := OutputSpec{Mode: config.OutputSubfolder}
	p1, r1, _ := n.Reserve(a, spec, "", "webp", config.ConflictOverwrite)
	p2, r2, _ := n.Reserve(b, spec, "", "webp", config.ConflictOverwrite)
	defer r1()
	defer r2()
	if p1 == p2 {
		t.Fatalf("two running tasks got the same output %s", p1)
	}
}

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		`a/b:c*?`:  "a_b_c__",
		"  CON ":   "_CON",
		"judul. ":  "judul",
		"":         "file",
		"Lagu Ok!": "Lagu Ok!",
	}
	for in, want := range cases {
		if got := SanitizeFileName(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestCommit(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "out.mp4")
	tmp := TempPath(final)
	if filepath.Ext(tmp) != ".mp4" {
		t.Fatalf("temp path must keep extension: %s", tmp)
	}
	touch(t, final)
	touch(t, tmp)
	if err := Commit(tmp, final); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatal("temp should be gone")
	}
}
