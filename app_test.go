package main

import (
	"path/filepath"
	"strings"
	"testing"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/config"
	"kuymediabox/internal/downloader"
)

func testApp(t *testing.T) *App {
	t.Helper()
	return &App{cfg: config.LoadFrom(filepath.Join(t.TempDir(), "settings.json")), collections: map[string]*downloader.Collection{}}
}

func TestOutputSpecResolution(t *testing.T) {
	a := testApp(t)
	// Default mode resolves to the module's own default folder.
	for _, k := range []string{"image", "video", "audio", "download"} {
		spec := a.outputSpec(k)
		if spec.Mode != config.OutputCustom || spec.Dir != appdir.DefaultOutputDir(k) || !strings.HasSuffix(spec.Dir, "KuyMediaBox") {
			t.Errorf("%s: %+v", k, spec)
		}
	}
	if a.GetDefaultDirs()["video"] == a.GetDefaultDirs()["audio"] {
		t.Error("video and audio should have different default folders")
	}

	s := a.cfg.Get()
	s.Outputs["image"] = config.Output{Mode: config.OutputSubfolder}
	s.Outputs["audio"] = config.Output{Mode: config.OutputCustom, Dir: `D:\Musik`}
	if _, err := a.cfg.Set(s); err != nil {
		t.Fatal(err)
	}
	if spec := a.outputSpec("image"); spec.Mode != config.OutputSubfolder || a.OutputFolder("image") != "" {
		t.Errorf("dynamic image output: %+v", spec)
	}
	if a.OutputFolder("audio") != `D:\Musik` {
		t.Errorf("custom audio folder: %q", a.OutputFolder("audio"))
	}
}

func TestCollectionDir(t *testing.T) {
	pl := &downloader.Collection{Type: downloader.TypePlaylist, Title: "Lo-fi: Belajar/Fokus"}
	if got := collectionDir(`D:\Unduh`, pl, true); got != filepath.Join(`D:\Unduh`, "Lo-fi_ Belajar_Fokus") {
		t.Errorf("playlist subfolder: %s", got)
	}
	if got := collectionDir(`D:\Unduh`, pl, false); got != `D:\Unduh` {
		t.Errorf("subfolders off: %s", got)
	}
	video := &downloader.Collection{Type: downloader.TypeVideo, Title: "x"}
	if got := collectionDir(`D:\Unduh`, video, true); got != `D:\Unduh` {
		t.Errorf("single video: %s", got)
	}
}
