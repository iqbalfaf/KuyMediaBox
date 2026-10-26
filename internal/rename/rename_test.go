package rename

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestFindField(t *testing.T) {
	lines := []string{
		"LAPORAN HASIL BELAJAR",
		"Nama Sekolah\t: SMA NEGERI 1 CONTOH",
		"Nama Peserta Didik\t:\tAHMAD FAUZI\tKelas\t: XII IPA 1",
		"NIS/NISN : 12345 / 0098765432",
		"Nama Orang Tua : SUPARMAN",
	}
	cases := []struct {
		labels []string
		want   string
	}{
		{[]string{"Nama Peserta Didik"}, "AHMAD FAUZI"},
		{[]string{"nama peserta didik"}, "AHMAD FAUZI"},
		{[]string{"Nama Siswa", "Nama Peserta Didik"}, "AHMAD FAUZI"},
		{[]string{"Nama"}, ""}, // "Nama Sekolah" and "Nama Orang Tua" are other fields
		{[]string{"Kelas"}, "XII IPA 1"},
		{[]string{"NISN"}, "12345 / 0098765432"},
		{[]string{"Alamat"}, ""},
	}
	for _, c := range cases {
		if got := FindField(lines, c.labels); got != c.want {
			t.Errorf("FindField(%v) = %q, want %q", c.labels, got, c.want)
		}
	}
	if got := FindField([]string{"Nama : BUDI SANTOSO Kelas : X"}, []string{"Nama"}); got != "BUDI SANTOSO" {
		t.Errorf("same-line value = %q", got)
	}
	if got := FindField([]string{"Nama ........ : Siti Aminah"}, []string{"Nama"}); got != "Siti Aminah" {
		t.Errorf("dot leaders = %q", got)
	}
}

func TestNewName(t *testing.T) {
	lines := []string{"Nama Peserta Didik : AHMAD FAUZI", "NISN : 0098765432", "Kelas : XII IPA 1"}
	reads := 0
	doc := File{Name: "rapor_001", Index: 2, Modified: time.Date(2026, 7, 1, 9, 0, 0, 0, time.Local), Lines: func() ([]string, error) {
		reads++
		return lines, nil
	}}
	check := func(f File, o Options, want string) {
		t.Helper()
		got, err := NewName(f, o)
		if err != nil || got != want {
			t.Errorf("NewName(%+v) = %q, %v; want %q", o, got, err, want)
		}
	}
	c := Options{Mode: ModeContent, Labels: DefaultLabels}
	check(doc, c, "AHMAD FAUZI")
	c.Case = "title"
	check(doc, c, "Ahmad Fauzi")
	c.Template = "{nama} - {nisn}"
	check(doc, c, "Ahmad Fauzi - 0098765432")
	c.Template = "{n}. {Kelas} {nama}" // the case applies to the name only
	c.Start, c.Digits = 1, 2
	check(doc, c, "03. XII IPA 1 Ahmad Fauzi")
	c.Template = "{file} {nama}/{date}"
	check(doc, c, "rapor_001 Ahmad Fauzi_2026-07-01")
	reads = 0
	c.Template = "{file} {n}"
	check(doc, c, "rapor_001 03")
	if reads != 0 {
		t.Error("a template without fields must not read the document")
	}
	c.Template = "{nama} {Alamat}"
	if _, err := NewName(doc, c); err == nil || !strings.Contains(err.Error(), "Alamat") {
		t.Errorf("missing field must name the label, got %v", err)
	}
	if _, err := NewName(File{Name: "foto"}, Options{Mode: ModeContent}); err == nil {
		t.Error("content mode needs a document")
	}
	if _, err := NewName(File{Name: "x", Lines: func() ([]string, error) { return nil, errors.New("boom") }}, Options{Mode: ModeContent}); err == nil || err.Error() != "boom" {
		t.Errorf("read error: %v", err)
	}

	img := File{Name: "IMG_20260701_0042", Index: 9, Modified: doc.Modified}
	check(img, Options{Mode: ModePattern, Pattern: "Liburan {n}", Start: 1, Digits: 3}, "Liburan 010")
	check(img, Options{Mode: ModePattern, Pattern: "{date} {file}", Tidy: true}, "2026-07-01 IMG 20260701 0042")
	check(img, Options{Mode: ModePattern, Pattern: "x{n}", Start: 100, Step: -10}, "x10")
	check(img, Options{Mode: ModeReplace, Find: "img_", With: "Foto "}, "Foto 20260701_0042")
	check(img, Options{Mode: ModeReplace, Find: "img_", With: "", MatchCase: true}, "IMG_20260701_0042")
	check(img, Options{Mode: ModeReplace, Find: `^IMG_(\d{4})(\d{2})(\d{2})_(\d+)$`, With: "$3-$2-$1 #$4", Regex: true}, "01-07-2026 #0042")
	check(img, Options{Mode: ModeReplace, Find: "_", With: "$1"}, "IMG$120260701$10042") // literal $ without regex
	check(img, Options{Mode: ModeEdit, Prefix: "{n} - ", CutStart: 4, Start: 1}, "10 - 20260701_0042")
	check(img, Options{Mode: ModeEdit, Suffix: " (final)", CutEnd: 5, Case: "lower"}, "img_20260701 (final)")
	check(File{Name: "laporan_akhir siswa"}, Options{Mode: ModeEdit, Case: "title", Tidy: true}, "Laporan Akhir Siswa")
	check(File{Name: "a"}, Options{Mode: ModeReplace, Find: "a", With: "b:c?"}, "b_c_")

	for _, o := range []Options{
		{Mode: ModePattern},
		{Mode: ModeReplace},
		{Mode: ModeReplace, Find: "(", Regex: true},
		{Mode: ModeEdit, CutStart: 10, CutEnd: 7},
		{Mode: ModeReplace, Find: "IMG_20260701_0042"},
		{Mode: "nope"},
	} {
		if _, err := NewName(img, o); err == nil {
			t.Errorf("%+v must fail", o)
		}
	}
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		b, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		out = append(out, e.Name()+"="+string(b))
	}
	sort.Strings(out)
	return out
}

func write(t *testing.T, dir string, files ...string) {
	t.Helper()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(strings.TrimSuffix(f, filepath.Ext(f))), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestApplyAndUndo(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.pdf", "b.pdf", "c.pdf", "d.PDF", "taken.pdf", "same.pdf")
	p := func(n string) string { return filepath.Join(dir, n) }
	res := Apply([]Job{
		{ID: "1", Path: p("a.pdf"), Name: "b"},     // swap a and b
		{ID: "2", Path: p("b.pdf"), Name: "a"},     //
		{ID: "3", Path: p("c.pdf"), Name: "taken"}, // exists outside the batch: numbered
		{ID: "4", Path: p("d.PDF"), Name: "TAKEN", Ext: ".pdf"},
		{ID: "5", Path: p("same.pdf"), Name: "same"}, // nothing to do
		{ID: "6", Path: p("gone.pdf"), Name: "x"},
		{ID: "7", Path: p("a.pdf"), Name: " "},
	}, "")
	want := []string{"TAKEN (2).pdf=d", "a.pdf=b", "b.pdf=a", "same.pdf=same", "taken (1).pdf=c", "taken.pdf=taken"}
	if got := listDir(t, dir); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("after apply: %v\nwant %v", got, want)
	}
	if !res[4].Skipped || res[5].Error == "" || res[6].Error == "" || res[0].To != p("b.pdf") || res[3].To != p("TAKEN (2).pdf") {
		t.Fatalf("results: %+v", res)
	}

	// Only the letter case changes.
	r := Apply([]Job{{ID: "1", Path: p("same.pdf"), Name: "SAME"}}, "")
	if r[0].Error != "" || r[0].To != p("SAME.pdf") {
		t.Fatalf("case-only: %+v", r)
	}
	if got := listDir(t, dir); !strings.Contains(strings.Join(got, " "), "SAME.pdf=same") {
		t.Fatalf("case-only: %v", got)
	}

	back := Undo(append(res, r...))
	for _, b := range back {
		if b.Error != "" {
			t.Errorf("undo: %+v", b)
		}
	}
	want = []string{"a.pdf=a", "b.pdf=b", "c.pdf=c", "d.PDF=d", "same.pdf=same", "taken.pdf=taken"}
	if got := listDir(t, dir); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("after undo: %v\nwant %v", got, want)
	}

	// Copies go to another folder; twins are numbered; undo deletes them.
	out := filepath.Join(t.TempDir(), "hasil")
	res = Apply([]Job{{ID: "1", Path: p("a.pdf"), Name: "Budi"}, {ID: "2", Path: p("b.pdf"), Name: "budi"}}, out)
	if got := listDir(t, out); strings.Join(got, " ") != "Budi.pdf=a budi (1).pdf=b" {
		t.Fatalf("copies: %v", got)
	}
	if !exists(p("a.pdf")) || !res[0].Copy {
		t.Fatal("copy must keep the source")
	}
	Undo(res)
	if got := listDir(t, out); len(got) != 0 {
		t.Fatalf("undo copies: %v", got)
	}
}
