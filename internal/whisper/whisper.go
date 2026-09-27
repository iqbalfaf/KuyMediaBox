// Package whisper runs whisper.cpp (whisper-cli.exe) to turn speech into subtitles, and
// manages its ggml model files.
package whisper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
)

// Model is a speech model offered in the UI.
type Model struct {
	ID     string `json:"id"`
	File   string `json:"file"`
	SizeMB int    `json:"sizeMB"`
	Note   string `json:"note"`
}

// Models from small/fast to large/accurate. Files come from the whisper.cpp model repository.
var Models = []Model{
	{ID: "tiny", File: "ggml-tiny.bin", SizeMB: 75},
	{ID: "base", File: "ggml-base.bin", SizeMB: 142},
	{ID: "small", File: "ggml-small.bin", SizeMB: 466},
	{ID: "turbo", File: "ggml-large-v3-turbo-q5_0.bin", SizeMB: 547},
	{ID: "medium", File: "ggml-medium.bin", SizeMB: 1463},
}

// ModelURL is where a model file is downloaded from.
func ModelURL(m Model) string {
	return "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/" + m.File
}

// Find returns the model with an id.
func Find(id string) (Model, bool) {
	for _, m := range Models {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

// ModelPath is the file of a model inside dir (the app's whisper model folder).
func ModelPath(dir string, m Model) string { return filepath.Join(dir, m.File) }

// Installed reports whether a model file is present (and not a partial download).
func Installed(dir string, m Model) bool {
	st, err := os.Stat(ModelPath(dir, m))
	return err == nil && st.Size() > int64(m.SizeMB)*1000*1000*9/10
}

// Options of one transcription.
type Options struct {
	Language  string   // "auto" or an ISO code (id, en, …)
	Translate bool     // translate into English
	Formats   []string // srt vtt txt lrc
	MaxLen    int      // characters per subtitle line (0 = whisper's segments as they are)
}

// Languages whisper handles well, offered in the UI (ISO 639-1).
var Languages = []string{"auto", "id", "en", "ms", "jv", "su", "ar", "zh", "ja", "ko", "hi", "th", "vi", "tl", "es", "pt", "fr", "de", "it", "nl", "ru", "tr"}

var validFormats = map[string]string{"srt": "-osrt", "vtt": "-ovtt", "txt": "-otxt", "lrc": "-olrc"}

// Normalize fixes invalid options.
func (o *Options) Normalize() {
	ok := false
	for _, l := range Languages {
		if l == o.Language {
			ok = true
		}
	}
	if !ok {
		o.Language = "auto"
	}
	var f []string
	seen := map[string]bool{}
	for _, x := range o.Formats {
		if _, ok := validFormats[x]; ok && !seen[x] {
			seen[x] = true
			f = append(f, x)
		}
	}
	if len(f) == 0 {
		f = []string{"srt"}
	}
	o.Formats = f
	if o.MaxLen < 0 || o.MaxLen > 200 {
		o.MaxLen = 0
	}
}

// Args builds the whisper-cli arguments; outBase is the output path without extension.
func Args(model, wav, outBase string, o Options) []string {
	o.Normalize()
	threads := max(1, min(runtime.NumCPU(), 8))
	a := []string{"-m", model, "-f", wav, "-of", outBase, "-l", o.Language, "-t", strconv.Itoa(threads), "-pp"}
	if o.Translate {
		a = append(a, "-tr")
	}
	for _, f := range o.Formats {
		a = append(a, validFormats[f])
	}
	// Line length is applied afterwards by Reflow: whisper's own -ml splits mid-sentence
	// and leaves one-word cues.
	return a
}

var (
	reProgress = regexp.MustCompile(`progress\s*=\s*(\d+)%`)
	reSegment  = regexp.MustCompile(`^\[(\d+):(\d+):(\d+)\.(\d+)\s*-->\s*(\d+):(\d+):(\d+)\.(\d+)\]`)
)

// segmentEnd reads the end time of a printed segment "[00:00:01.000 --> 00:00:05.280]  text".
func segmentEnd(line string) (float64, bool) {
	m := reSegment.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[5])
	mi, _ := strconv.Atoi(m[6])
	s, _ := strconv.Atoi(m[7])
	ms, _ := strconv.Atoi(m[8])
	return float64(h*3600+mi*60+s) + float64(ms)/1000, true
}

// Run executes whisper-cli. duration (seconds of audio) turns printed segments into
// progress; progress receives 0..1.
func Run(ctx context.Context, cli string, args []string, duration float64, progress func(float64)) error {
	cmd := proc.Command(ctx, cli, args...)
	cmd.Dir = filepath.Dir(cli) // the ggml DLLs sit next to the program
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf(i18n.L("whisper tidak bisa dijalankan: %w", "whisper can't be started: %w"), err)
	}
	var mu sync.Mutex
	best := 0.0
	report := func(p float64) {
		mu.Lock()
		defer mu.Unlock()
		if p > best && p <= 1 {
			best = p
			progress(p)
		}
	}
	tail := proc.NewTail(30)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		proc.ScanLines(stdout, func(line string) {
			if end, ok := segmentEnd(line); ok && duration > 0 {
				report(end / duration)
			}
		})
	}()
	go func() {
		defer wg.Done()
		proc.ScanLines(stderr, func(line string) {
			tail.Add(line)
			if m := reProgress.FindStringSubmatch(line); m != nil {
				v, _ := strconv.Atoi(m[1])
				report(float64(v) / 100)
			}
		})
	}()
	wg.Wait()
	err = cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		msg := tail.String()
		switch {
		case strings.Contains(msg, "failed to load model") || strings.Contains(msg, "invalid model"):
			return errors.New(i18n.L("File model whisper rusak. Hapus lalu unduh ulang modelnya.", "The whisper model file is damaged. Delete it and download it again."))
		case strings.Contains(strings.ToLower(msg), "out of memory") || strings.Contains(msg, "failed to allocate"):
			return errors.New(i18n.L("Memori tidak cukup untuk model ini. Pilih model yang lebih kecil.", "Not enough memory for this model. Pick a smaller one."))
		}
		return fmt.Errorf("whisper: %v\n%s", err, proc.LastLines(msg, 6))
	}
	return nil
}
