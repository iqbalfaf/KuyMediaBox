package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsWhenFileMissing(t *testing.T) {
	st := LoadFrom(filepath.Join(t.TempDir(), "none.json"))
	s := st.Get()
	for _, k := range OutputKinds {
		if s.Outputs[k].Mode != OutputDefault {
			t.Errorf("%s: mode %q, want default", k, s.Outputs[k].Mode)
		}
	}
	if !s.Notify || !s.SkipDownloaded || !s.DownloadSubfolders || s.Suffix != "_converted" || s.Conflict != ConflictRename {
		t.Errorf("unexpected defaults %+v", s)
	}
}

func TestNormalize(t *testing.T) {
	s := Settings{Outputs: map[string]Output{
		"image":    {Mode: OutputCustom, Dir: ""},        // custom without folder → default
		"video":    {Mode: OutputSubfolder, Dir: "x"},    // dynamic keeps mode, drops dir
		"audio":    {Mode: "rubbish"},                    // unknown → default
		"download": {Mode: OutputSame},                   // downloads cannot be "same"
		"other":    {Mode: OutputCustom, Dir: `C:\temp`}, // unknown kind dropped
	}, Conflict: "?", Suffix: `a/b:c`}
	s.Normalize()
	if s.Outputs["image"].Mode != OutputDefault || s.Outputs["video"].Mode != OutputSubfolder || s.Outputs["video"].Dir != "" ||
		s.Outputs["audio"].Mode != OutputDefault || s.Outputs["download"].Mode != OutputDefault {
		t.Fatalf("outputs %+v", s.Outputs)
	}
	if _, ok := s.Outputs["other"]; ok {
		t.Fatal("unknown kind kept")
	}
	if s.Conflict != ConflictRename || s.Suffix != "abc" {
		t.Fatalf("conflict %q suffix %q", s.Conflict, s.Suffix)
	}
}

func TestSaveLoadRoundTripAndPartialFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	st := LoadFrom(p)
	s := st.Get()
	s.Outputs["video"] = Output{Mode: OutputCustom, Dir: `D:\Hasil Video`}
	s.Notify = false
	if _, err := st.Set(s); err != nil {
		t.Fatal(err)
	}
	got := LoadFrom(p).Get()
	if got.Outputs["video"].Dir != `D:\Hasil Video` || got.Notify || got.Outputs["image"].Mode != OutputDefault {
		t.Fatalf("round trip %+v", got)
	}

	// A file from an older version: missing keys keep their defaults, legacy download folder migrates.
	os.WriteFile(p, []byte(`{"suffix":"_x","downloadDir":"E:\\Unduhan"}`), 0o644)
	old := LoadFrom(p).Get()
	if old.Suffix != "_x" || !old.Notify || !old.SkipDownloaded || !old.DownloadSubfolders {
		t.Fatalf("partial file lost defaults: %+v", old)
	}
	if o := old.Outputs["download"]; o.Mode != OutputCustom || o.Dir != `E:\Unduhan` {
		t.Fatalf("legacy download dir not migrated: %+v", o)
	}

	os.WriteFile(p, []byte(`{not json`), 0o644)
	if LoadFrom(p).Get().Suffix != "_converted" {
		t.Fatal("corrupt file must fall back to defaults")
	}
}
