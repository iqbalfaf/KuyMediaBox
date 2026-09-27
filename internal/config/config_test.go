package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsWhenFileMissing(t *testing.T) {
	st := LoadFrom(filepath.Join(t.TempDir(), "none.json"))
	s := st.Get()
	for _, k := range OutputKinds {
		want := OutputDefault
		if k == "subtitle" {
			want = OutputSame // subtitles sit next to the video
		}
		if s.Outputs[k].Mode != want {
			t.Errorf("%s: mode %q, want %q", k, s.Outputs[k].Mode, want)
		}
	}
	// A settings file from before the Subtitle page still gets "next to the video".
	old := Settings{Outputs: map[string]Output{"video": {Mode: OutputDefault}}}
	old.Normalize()
	if old.Outputs["subtitle"].Mode != OutputSame {
		t.Errorf("old settings: subtitle mode %q", old.Outputs["subtitle"].Mode)
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
	}, Conflict: "?", Suffix: `a/b:c`, Language: "fr"}
	s.Normalize()
	if s.Outputs["image"].Mode != OutputDefault || s.Outputs["video"].Mode != OutputSubfolder || s.Outputs["video"].Dir != "" ||
		s.Outputs["audio"].Mode != OutputDefault || s.Outputs["download"].Mode != OutputDefault {
		t.Fatalf("outputs %+v", s.Outputs)
	}
	if _, ok := s.Outputs["other"]; ok {
		t.Fatal("unknown kind kept")
	}
	if s.Language != LangID {
		t.Fatalf("language %q, want id", s.Language)
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
	s.Language = LangEN
	if _, err := st.Set(s); err != nil {
		t.Fatal(err)
	}
	got := LoadFrom(p).Get()
	if got.Outputs["video"].Dir != `D:\Hasil Video` || got.Notify || got.Language != LangEN || got.Outputs["image"].Mode != OutputDefault {
		t.Fatalf("round trip %+v", got)
	}

	// Tray, workflows and workflow watch rules survive a restart; broken steps are dropped.
	s = got
	s.Tray = true
	s.Workflows = []Workflow{
		{ID: "w1", Name: " Lagu rapi ", Steps: []FlowStep{{Kind: "audio", Label: "MP3", Job: json.RawMessage(`{"mode":"convert"}`)}, {Kind: "bogus"}, {Kind: "pdf", Tool: "compress"}}},
		{ID: "", Name: "no id"},
	}
	s.Watch = []WatchRule{{ID: "r1", Dir: `D:\Masuk`, Kind: "flow", Options: json.RawMessage(`{"workflow":"w1"}`), Enabled: true}}
	if _, err := st.Set(s); err != nil {
		t.Fatal(err)
	}
	got = LoadFrom(p).Get()
	if !got.Tray || len(got.Workflows) != 1 || got.Workflows[0].Name != "Lagu rapi" || len(got.Workflows[0].Steps) != 2 {
		t.Fatalf("workflows %+v tray %v", got.Workflows, got.Tray)
	}
	if string(got.Workflows[0].Steps[1].Job) != "{}" || len(got.Watch) != 1 || got.Watch[0].Kind != "flow" {
		t.Fatalf("steps %+v watch %+v", got.Workflows[0].Steps, got.Watch)
	}

	// A file from an older version: missing keys keep their defaults, legacy download folder migrates.
	os.WriteFile(p, []byte(`{"suffix":"_x","downloadDir":"E:\\Unduhan"}`), 0o644)
	old := LoadFrom(p).Get()
	if old.Suffix != "_x" || !old.Notify || !old.SkipDownloaded || !old.DownloadSubfolders || old.Language != LangID {
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
