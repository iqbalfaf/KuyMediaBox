package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/mediaconv"
	"kuymediabox/internal/naming"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/tools"
	"kuymediabox/internal/whisper"
)

// SubtitleJob is what the Subtitle page sends: speech recognition with whisper.cpp.
type SubtitleJob struct {
	Model     string   `json:"model"`     // tiny base small turbo medium
	Language  string   `json:"language"`  // auto or an ISO code
	Translate bool     `json:"translate"` // translate into English
	Formats   []string `json:"formats"`   // srt vtt txt lrc
	MaxLen    int      `json:"maxLen"`    // characters per line (0 = whole sentences)
	Video     string   `json:"video"`     // none | embed | burn: also save the video with the subtitles
}

// StartSubtitle queues speech-to-subtitle jobs for video or audio files.
func (a *App) StartSubtitle(items []JobItem, job SubtitleJob) ([]JobRef, error) {
	a.waitTools()
	out, err := a.resolveOutput(queue.KindSubtitle)
	if err != nil {
		return nil, err
	}
	ff, probe := a.tools.Path(tools.FFmpeg), a.tools.Path(tools.FFprobe)
	if ff == "" || probe == "" {
		return nil, errNoFFmpeg()
	}
	cli := a.tools.Path(tools.Whisper)
	if cli == "" {
		return nil, errors.New(i18n.L("Whisper belum terpasang. Klik Unduh di banner atas atau di Pengaturan › Tools pendukung.", "Whisper isn't installed. Click Download in the banner above or in Settings › Supporting tools."))
	}
	model := modelPath("whisper", job.Model)
	if m, ok := findModel("whisper", job.Model); !ok || !modelInstalled("whisper", m) {
		return nil, errors.New(i18n.L("Model yang dipilih belum diunduh.", "The chosen model hasn't been downloaded yet."))
	}
	switch job.Video {
	case "embed", "burn":
	default:
		job.Video = "none"
	}
	specs := make([]queue.Spec, len(items))
	for i, it := range items {
		specs[i] = queue.Spec{Title: filepath.Base(it.Path), Input: it.Path, InSize: fileSize(it.Path), Run: a.subtitleTask(it, job, itemOut(out, it.OutDir), ff, probe, cli, model)}
	}
	return refs(items, a.queue.AddMany(queue.KindSubtitle, specs)), nil
}

func (a *App) subtitleTask(it JobItem, job SubtitleJob, out naming.OutputSpec, ff, probe, cli, model string) queue.RunFunc {
	return func(ctx context.Context, r queue.Reporter) error {
		if _, err := os.Stat(it.Path); err != nil {
			return queue.Fail(i18n.L("File asli tidak ditemukan (dipindah atau dihapus?)", "Source file not found (moved or deleted?)"), err.Error())
		}
		info, err := ffmpeg.Probe(ctx, probe, it.Path)
		if err != nil {
			return queue.Fail(i18n.L("File tidak bisa dibaca", "File can't be read"), err.Error())
		}
		if !info.HasAudio {
			return queue.Fail(i18n.L("File ini tidak punya suara", "This file has no sound"), "")
		}
		work, err := os.MkdirTemp(appdir.TempDir(), "sub-*")
		if err != nil {
			return queue.Fail(err.Error(), "")
		}
		defer os.RemoveAll(work)

		// 1. Speech as 16 kHz mono WAV, the input whisper expects.
		r.Message(i18n.L("Menyiapkan audio…", "Preparing the audio…"))
		wav := filepath.Join(work, "speech.wav")
		if err := ffmpeg.RunIn(ctx, ff, work, []string{"-i", it.Path, "-map", "0:a:0", "-vn", "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wav}, info.Duration, func(p float64, _ string) {
			if p >= 0 {
				r.Progress(p * 0.05)
			}
		}); err != nil {
			return err
		}

		// 2. Recognition. SRT is always made: the video options need it.
		o := whisper.Options{Language: job.Language, Translate: job.Translate, Formats: append([]string{}, job.Formats...), MaxLen: job.MaxLen}
		o.Normalize()
		formats := o.Formats
		if job.Video != "none" && !contains(o.Formats, "srt") {
			o.Formats = append(o.Formats, "srt")
		}
		r.Message(i18n.L("Mengenali ucapan…", "Recognising speech…"))
		base := filepath.Join(work, "speech")
		if err := whisper.Run(ctx, cli, whisper.Args(model, wav, base, o), info.Duration, func(p float64) {
			r.Progress(0.05 + p*0.85)
		}); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return queue.Fail(firstLineOf(err.Error()), err.Error())
		}

		for _, f := range []string{"srt", "vtt"} {
			if err := whisper.ReflowFile(base+"."+f, o.MaxLen); err != nil && !os.IsNotExist(err) {
				return queue.Fail(err.Error(), "")
			}
		}

		// 3. Subtitle files next to (or in the folder of) the source.
		s := a.cfg.Get()
		var first string
		var firstSize int64
		for _, f := range formats {
			src := base + "." + f
			if _, err := os.Stat(src); err != nil {
				return queue.Fail(i18n.L("Whisper tidak menghasilkan subtitle", "Whisper produced no subtitles"), err.Error())
			}
			target, release, err := a.namer.Reserve(it.Path, out, "", f, s.Conflict)
			if errors.Is(err, naming.ErrExists) {
				continue
			}
			if err != nil {
				return queue.Fail(err.Error(), "")
			}
			err = copyFile(src, target)
			release()
			if err != nil {
				return queue.Fail(i18n.L("Tidak bisa menyimpan file hasil", "Can't save the output file"), err.Error())
			}
			a.markProduced(target)
			if first == "" {
				first, firstSize = target, fileSize(target)
			}
		}

		// 4. Optionally the video with the subtitles embedded or burned in.
		if job.Video != "none" && info.HasVideo {
			r.Message(i18n.L("Menyimpan video bersubtitle…", "Saving the subtitled video…"))
			vo := mediaconv.VideoOptions{Format: "mkv", Codec: "copy", Subtitles: job.Video, SubFile: base + ".srt", AudioMode: "copy"}
			switch ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(it.Path)), "."); {
			case job.Video == "burn":
				vo.Format, vo.Codec, vo.Quality, vo.AudioMode = "mp4", "h264", "tinggi", "auto"
			case ext == "mp4" || ext == "mov" || ext == "webm" || ext == "mkv":
				vo.Format = ext
			}
			if vo.Codec == "copy" && !mediaconv.CanCopyVideo(info.VideoCodec, vo.Format) {
				vo.Format = "mkv"
			}
			target, release, err := a.namer.Reserve(it.Path, out, i18n.L("_subtitle", "_subtitled"), vo.Format, s.Conflict)
			if err == nil {
				tmp := naming.TempPath(target)
				plan, perr := mediaconv.VideoPlan(it.Path, tmp, info, vo, a.encoders(), work)
				if perr == nil {
					perr = runPlan(ctx, ff, plan, progressFrom(r, 0.9, 0.1))
				}
				if perr == nil {
					perr = naming.Commit(tmp, target)
				}
				os.Remove(tmp)
				release()
				if perr != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					return queue.Fail(i18n.L("Subtitle jadi, tapi video bersubtitle gagal dibuat", "Subtitles done, but the subtitled video failed"), perr.Error())
				}
				a.markProduced(target)
				first, firstSize = target, fileSize(target)
			} else if !errors.Is(err, naming.ErrExists) {
				return queue.Fail(err.Error(), "")
			}
		}
		if first == "" {
			return queue.Skip(i18n.L("File hasil sudah ada", "Output file already exists"))
		}
		r.SetOutput(first, firstSize)
		return nil
	}
}

// progressFrom maps a sub-step's 0..1 progress into [start, start+span] of a task.
func progressFrom(r queue.Reporter, start, span float64) queue.Reporter {
	return scaledReporter{r, start, span}
}

type scaledReporter struct {
	queue.Reporter
	start, span float64
}

func (s scaledReporter) Progress(p float64) {
	if p < 0 {
		s.Reporter.Progress(-1)
		return
	}
	s.Reporter.Progress(s.start + p*s.span)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := naming.TempPath(dst)
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, in); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return naming.Commit(tmp, dst)
}

// SubtitleLanguages lists the languages offered for recognition.
func (a *App) SubtitleLanguages() []string { return whisper.Languages }

var _ = fmt.Sprintf
