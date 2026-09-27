package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/config"
	"kuymediabox/internal/downloader"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/queue"
)

// Command line: KuyMediaBox.exe convert|flow|pdf|download|presets|flows|help …
// Runs without a window, prints progress and exits with 0 (all done), 1 (something failed)
// or 2 (wrong usage).

const cliTempPrefix = "tmp-cli-"

var cliCommands = map[string]bool{
	"convert": true, "flow": true, "workflow": true, "pdf": true, "download": true,
	"presets": true, "flows": true, "workflows": true, "help": true, "--help": true, "-h": true, "/?": true, "version": true, "--version": true,
}

// cliRequested reports whether the arguments are a command (not files from Explorer).
func cliRequested(args []string) bool {
	if len(args) == 0 || !cliCommands[strings.ToLower(args[0])] {
		return false
	}
	_, err := os.Stat(args[0]) // a file that happens to be called "convert"
	return err != nil
}

type cliArgs struct {
	cmd   string
	pos   []string
	flags map[string]string
}

// valueFlags take a value; the others are switches.
var valueFlags = map[string]bool{"preset": true, "kind": true, "out": true, "format": true, "quality": true, "flow": true}

func parseCLI(args []string) (cliArgs, error) {
	c := cliArgs{cmd: strings.ToLower(args[0]), flags: map[string]string{}}
	for i := 1; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			c.pos = append(c.pos, a)
			continue
		}
		name, val, hasVal := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		name = strings.ToLower(name)
		if valueFlags[name] && !hasVal {
			if i+1 >= len(args) {
				return c, fmt.Errorf(i18n.L("--%s butuh nilai", "--%s needs a value"), name)
			}
			i++
			val = args[i]
		}
		if !valueFlags[name] && !hasVal {
			val = "1"
		}
		c.flags[name] = val
	}
	return c, nil
}

func runCLI(args []string) int {
	appdir.UseOwnTemp(fmt.Sprintf("%s%d", cliTempPrefix, os.Getpid()))
	defer os.RemoveAll(appdir.TempDir())
	out := os.Stdout
	c, err := parseCLI(args)
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	switch c.cmd {
	case "help", "--help", "-h", "/?":
		printHelp(out)
		return 0
	case "version", "--version":
		fmt.Fprintln(out, "KuyMediaBox", Version)
		return 0
	}

	a := NewApp()
	a.cli = true
	switch c.cmd {
	case "presets":
		return cliPresets(out, c)
	case "flows", "workflows":
		return cliFlows(a, out)
	}

	a.tools.Detect(context.Background())
	a.markToolsDetected()
	var started int
	switch c.cmd {
	case "convert":
		started, err = a.cliConvert(out, c)
	case "flow", "workflow":
		started, err = a.cliFlow(out, c)
	case "pdf":
		started, err = a.cliPdf(out, c)
	case "download":
		started, err = a.cliDownload(out, c)
	}
	if err != nil {
		fmt.Fprintln(out, "✗", err)
		if started == 0 {
			return 2
		}
	}
	if started == 0 {
		fmt.Fprintln(out, i18n.L("Tidak ada file yang bisa diproses.", "Nothing to process."))
		return 2
	}
	failed := a.cliWait(out)
	if failed > 0 || err != nil {
		return 1
	}
	return 0
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, i18n.L(`KuyMediaBox — perintah tanpa jendela

  kmb convert <file/folder…> [--preset NAMA] [--kind image|video|audio|subtitle] [--out FOLDER]
      Konversi dengan preset (default: pengaturan halaman sekarang). Jenis ditebak dari file.
  kmb flow "<alur kerja>" <file/folder…>
      Jalankan alur kerja yang dibuat di aplikasi.
  kmb pdf <alat> <file.pdf…> [--out FOLDER]
      Alat PDF per file: compress, rotate, ocr, pdf2img, pdf2txt, pdf2md, pdf2csv, pdfa, flatten, …
  kmb download <link…> [--audio] [--format mp3|m4a|opus|flac|wav] [--quality 1080] [--flow NAMA] [--out FOLDER]
  kmb presets [--kind video]     daftar preset
  kmb flows                      daftar alur kerja

Contoh:
  kmb convert rekaman.mkv --preset "WhatsApp"
  kmb flow "Video → lagu MP3" D:\Video
  kmb download https://youtu.be/xxxx --audio

Tanpa perintah kmb, pakai: start /wait KuyMediaBox.exe convert …
Kode keluar: 0 berhasil, 1 ada yang gagal, 2 salah pakai.
`, `KuyMediaBox — window-less commands

  kmb convert <file/folder…> [--preset NAME] [--kind image|video|audio|subtitle] [--out FOLDER]
      Convert with a preset (default: the page's current settings). The type is guessed from the files.
  kmb flow "<workflow>" <file/folder…>
      Run a workflow made in the app.
  kmb pdf <tool> <file.pdf…> [--out FOLDER]
      Per-file PDF tools: compress, rotate, ocr, pdf2img, pdf2txt, pdf2md, pdf2csv, pdfa, flatten, …
  kmb download <link…> [--audio] [--format mp3|m4a|opus|flac|wav] [--quality 1080] [--flow NAME] [--out FOLDER]
  kmb presets [--kind video]     list presets
  kmb flows                      list workflows

Examples:
  kmb convert recording.mkv --preset "WhatsApp"
  kmb flow "Video → MP3 song" D:\Videos
  kmb download https://youtu.be/xxxx --audio

Without the kmb command, use: start /wait KuyMediaBox.exe convert …
Exit codes: 0 done, 1 something failed, 2 wrong usage.
`))
}

func cliPresets(w io.Writer, c cliArgs) int {
	list, err := loadRecipes()
	if err != nil {
		fmt.Fprintln(w, "✗", err)
		return 1
	}
	if len(list) == 0 {
		fmt.Fprintln(w, i18n.L("Belum ada preset. Buka KuyMediaBox sekali supaya preset tersedia.", "No presets yet. Open KuyMediaBox once so the presets are available."))
		return 1
	}
	kind := c.flags["kind"]
	last := ""
	for _, r := range list {
		if kind != "" && r.Kind != kind || r.ID == "current" {
			continue
		}
		if r.Kind != last {
			fmt.Fprintf(w, "\n[%s]\n", r.Kind)
			last = r.Kind
		}
		mark := " "
		if !r.Builtin {
			mark = "*"
		}
		fmt.Fprintf(w, " %s %s\n", mark, r.Name)
	}
	fmt.Fprintln(w, "\n", i18n.L("* = preset buatan sendiri", "* = your own preset"))
	return 0
}

func cliFlows(a *App, w io.Writer) int {
	flows := a.cfg.Get().Workflows
	if len(flows) == 0 {
		fmt.Fprintln(w, i18n.L("Belum ada alur kerja. Buat di aplikasi: Alur kerja.", "No workflows yet. Make one in the app: Workflows."))
		return 0
	}
	for _, f := range flows {
		var steps []string
		for _, s := range f.Steps {
			label := s.Label
			if label == "" {
				label = s.Tool
			}
			steps = append(steps, s.Kind+": "+label)
		}
		fmt.Fprintf(w, "%s\n    %s\n", f.Name, strings.Join(steps, "  →  "))
	}
	return 0
}

// expandPaths turns folders into the files directly inside them.
func expandPaths(args []string) ([]string, error) {
	var out []string
	for _, p := range args {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf(i18n.L("Tidak ditemukan: %s", "Not found: %s"), p)
		}
		if !st.IsDir() {
			out = append(out, abs)
			continue
		}
		entries, _ := os.ReadDir(abs)
		for _, e := range entries {
			if !e.IsDir() {
				out = append(out, filepath.Join(abs, e.Name()))
			}
		}
	}
	return out, nil
}

func kindOfFile(p string) string {
	ext := strings.ToLower(filepath.Ext(p))
	switch {
	case imageconv.InputExts[ext]:
		return queue.KindImage
	case videoExts[ext]:
		return queue.KindVideo
	case audioExts[ext]:
		return queue.KindAudio
	case ext == ".pdf":
		return kindPDF
	}
	return ""
}

func cliOutDir(c cliArgs) (string, error) {
	dir := c.flags["out"]
	if dir == "" {
		return "", nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return abs, os.MkdirAll(abs, 0o755)
}

func (a *App) cliConvert(w io.Writer, c cliArgs) (int, error) {
	files, err := expandPaths(c.pos)
	if err != nil {
		return 0, err
	}
	outDir, err := cliOutDir(c)
	if err != nil {
		return 0, err
	}
	list, err := loadRecipes()
	if err != nil {
		return 0, err
	}
	groups := map[string][]string{}
	skipped := 0
	for _, f := range files {
		k := c.flags["kind"]
		if k == "" {
			k = kindOfFile(f)
		}
		if k == "" || k == kindPDF {
			skipped++
			continue
		}
		groups[k] = append(groups[k], f)
	}
	if skipped > 0 {
		fmt.Fprintf(w, i18n.L("Dilewati %d file yang bukan gambar/video/audio.\n", "Skipped %d files that aren't images/video/audio.\n"), skipped)
	}
	kinds := make([]string, 0, len(groups))
	for k := range groups {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	started := 0
	var errs []string
	for _, k := range kinds {
		r, err := findRecipe(list, k, c.flags["preset"])
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		fmt.Fprintf(w, "%s · %s · %d file\n", k, r.Name, len(groups[k]))
		ids, err := a.startFlowStep(config.FlowStep{Kind: k, Job: r.Job}, groups[k], outDir, false)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		started += len(ids)
	}
	if len(errs) > 0 {
		return started, errors.New(strings.Join(errs, "; "))
	}
	return started, nil
}

func (a *App) cliFlow(w io.Writer, c cliArgs) (int, error) {
	if len(c.pos) < 2 {
		return 0, errors.New(i18n.L("Pakai: kmb flow \"<alur kerja>\" <file…>", "Usage: kmb flow \"<workflow>\" <file…>"))
	}
	files, err := expandPaths(c.pos[1:])
	if err != nil {
		return 0, err
	}
	a.flows.hook = func(r FlowResult) {
		if r.Error != "" {
			fmt.Fprintf(w, "✗ %s: %s\n", filepath.Base(r.Input), r.Error)
		} else {
			fmt.Fprintf(w, "✓ %s → %s\n", filepath.Base(r.Input), r.Output)
		}
	}
	a.cliQuietDone = true // the workflow reports each file itself
	return a.RunWorkflow(c.pos[0], files)
}

func (a *App) cliPdf(w io.Writer, c cliArgs) (int, error) {
	if len(c.pos) < 2 {
		return 0, errors.New(i18n.L("Pakai: kmb pdf <alat> <file.pdf…>", "Usage: kmb pdf <tool> <file.pdf…>"))
	}
	tool := strings.ToLower(c.pos[0])
	files, err := expandPaths(c.pos[1:])
	if err != nil {
		return 0, err
	}
	outDir, err := cliOutDir(c)
	if err != nil {
		return 0, err
	}
	var pdfs []string
	for _, f := range files {
		if kindOfFile(f) == kindPDF {
			pdfs = append(pdfs, f)
		}
	}
	job := json.RawMessage("{}")
	if list, _ := loadRecipes(); list != nil {
		if r, err := findRecipe(list, kindPDF, ""); err == nil {
			job = r.Job
		}
	}
	ids, err := a.startFlowStep(config.FlowStep{Kind: kindPDF, Tool: tool, Job: job}, pdfs, outDir, false)
	return len(ids), err
}

func (a *App) cliDownload(w io.Writer, c cliArgs) (int, error) {
	if len(c.pos) == 0 {
		return 0, errors.New(i18n.L("Pakai: kmb download <link…>", "Usage: kmb download <link…>"))
	}
	outDir, err := cliOutDir(c)
	if err != nil {
		return 0, err
	}
	o := downloader.Options{SkipExisting: true, Embed: true, OutDir: outDir, Quality: c.flags["quality"], AudioFormat: c.flags["format"], AudioQuality: "auto"}
	if c.flags["audio"] != "" || c.flags["format"] != "" {
		o.Mode = "audio"
	}
	if name := c.flags["flow"]; name != "" {
		wf, err := a.workflow(name)
		if err != nil {
			return 0, err
		}
		o.Workflow = wf.ID
		a.flows.hook = func(r FlowResult) {
			if r.Error != "" {
				fmt.Fprintf(w, "✗ %s: %s\n", wf.Name, r.Error)
			} else {
				fmt.Fprintf(w, "✓ %s → %s\n", wf.Name, r.Output)
			}
		}
	}
	started := 0
	var errs []string
	for _, link := range c.pos {
		fmt.Fprintf(w, "%s %s\n", i18n.L("Memeriksa", "Checking"), link)
		col, err := a.AnalyzeLink(link)
		if err != nil {
			errs = append(errs, link+": "+err.Error())
			continue
		}
		ids := make([]string, 0, len(col.Entries))
		for _, e := range col.Entries {
			if !e.Unavailable {
				ids = append(ids, e.ID)
			}
		}
		refs, err := a.StartDownloads(col.Key, ids, o)
		if err != nil {
			errs = append(errs, link+": "+err.Error())
			continue
		}
		fmt.Fprintf(w, "  %s · %d item\n", col.Title, len(refs))
		started += len(refs)
	}
	if len(errs) > 0 {
		return started, errors.New(strings.Join(errs, "; "))
	}
	return started, nil
}

// cliWait prints progress until every task and workflow is finished; returns the failures.
// Ctrl+C cancels everything.
func (a *App) cliWait(w io.Writer) int {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	defer signal.Stop(stop)
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			fmt.Fprintln(w, i18n.L("Membatalkan…", "Cancelling…"))
			for _, k := range []string{queue.KindImage, queue.KindVideo, queue.KindAudio, queue.KindDownload, queue.KindPDF, queue.KindSubtitle} {
				a.queue.CancelKind(k)
			}
		})
	}
	seen := map[string]bool{}
	lastPct := map[string]int{}
	failed := 0
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			cancel()
		case <-tick.C:
		}
		busy := false
		for _, t := range a.queue.List() {
			switch t.Status {
			case queue.StatusQueued:
				busy = true
			case queue.StatusRunning:
				busy = true
				pct := int(t.Progress*10) * 10
				if pct > lastPct[t.ID] {
					lastPct[t.ID] = pct
					fmt.Fprintf(w, "  … %s %d%%\n", t.Title, pct)
				}
			default:
				if seen[t.ID] {
					continue
				}
				seen[t.ID] = true
				flowStep := t.Output != "" && inFlowTemp(t.Output)
				switch {
				case t.Status == queue.StatusDone && !flowStep && !a.cliQuietDone:
					fmt.Fprintf(w, "✓ %s → %s\n", t.Title, t.Output)
				case t.Status == queue.StatusSkipped:
					fmt.Fprintf(w, "– %s: %s\n", t.Title, t.Message)
				case t.Status == queue.StatusFailed:
					failed++
					msg := t.Message
					if t.Detail != "" {
						msg += " (" + firstLine(t.Detail) + ")"
					}
					fmt.Fprintf(w, "✗ %s: %s\n", t.Title, msg)
				case t.Status == queue.StatusCanceled:
					failed++
				}
			}
		}
		if !busy && !a.flowsBusy() {
			return failed
		}
	}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

// ---- the "kmb" command -------------------------------------------------------------------

// CLICommand describes the kmb.cmd shortcut that runs the command line from any console.
type CLICommand struct {
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
	OnPath    bool   `json:"onPath"` // its folder is on PATH, so "kmb" works everywhere
}

// cliCmdDir is %LOCALAPPDATA%\Microsoft\WindowsApps, which Windows puts on the user's PATH.
func cliCmdDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WindowsApps")
}

func cliCmdScript() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	// start /wait makes the console wait for the window-less app and keeps its exit code.
	return "@echo off\r\nstart \"\" /b /wait \"" + exe + "\" %*\r\nexit /b %errorlevel%\r\n", nil
}

// GetCLICommand reports whether the kmb command is installed.
func (a *App) GetCLICommand() CLICommand {
	p := filepath.Join(cliCmdDir(), "kmb.cmd")
	c := CLICommand{Path: p}
	_, err := os.Stat(p)
	c.Installed = err == nil
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.EqualFold(filepath.Clean(d), filepath.Clean(cliCmdDir())) {
			c.OnPath = true
		}
	}
	return c
}

// SetCLICommand installs or removes the kmb command.
func (a *App) SetCLICommand(on bool) (CLICommand, error) {
	p := filepath.Join(cliCmdDir(), "kmb.cmd")
	if !on {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return a.GetCLICommand(), err
		}
		return a.GetCLICommand(), nil
	}
	script, err := cliCmdScript()
	if err != nil {
		return a.GetCLICommand(), err
	}
	if err := os.MkdirAll(cliCmdDir(), 0o755); err != nil {
		return a.GetCLICommand(), err
	}
	if err := os.WriteFile(p, []byte(script), 0o644); err != nil {
		return a.GetCLICommand(), err
	}
	return a.GetCLICommand(), nil
}

// refreshCLICommand points an installed kmb command at the current exe (a portable exe that
// moved or was updated under a new name).
func (a *App) refreshCLICommand() {
	p := filepath.Join(cliCmdDir(), "kmb.cmd")
	cur, err := os.ReadFile(p)
	if err != nil {
		return
	}
	if script, err := cliCmdScript(); err == nil && string(cur) != script {
		_ = os.WriteFile(p, []byte(script), 0o644)
	}
}
