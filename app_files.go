package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/mediaconv"
	"kuymediabox/internal/naming"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/tools"
)

var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".mov": true, ".avi": true, ".webm": true, ".flv": true, ".wmv": true,
	".3gp": true, ".ts": true, ".m4v": true, ".mpg": true, ".mpeg": true, ".mts": true, ".m2ts": true,
	".ogv": true, ".vob": true,
}

var audioExts = map[string]bool{
	".mp3": true, ".wav": true, ".flac": true, ".aac": true, ".m4a": true, ".ogg": true, ".opus": true,
	".wma": true, ".aiff": true, ".aif": true, ".amr": true, ".ape": true, ".wv": true, ".mka": true, ".oga": true,
}

func acceptExt(kind, ext string) bool {
	ext = strings.ToLower(ext)
	switch kind {
	case queue.KindImage:
		return imageconv.InputExts[ext]
	case queue.KindVideo:
		return videoExts[ext]
	case queue.KindAudio:
		return audioExts[ext] || videoExts[ext]
	}
	return false
}

// FileItem is a file added to a converter page.
type FileItem struct {
	ID            string  `json:"id"`
	Path          string  `json:"path"`
	Name          string  `json:"name"`
	Ext           string  `json:"ext"`
	Size          int64   `json:"size"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	Duration      float64 `json:"duration"`
	Format        string  `json:"format"`
	VideoCodec    string  `json:"videoCodec"`
	FPS           float64 `json:"fps"`
	AudioCodec    string  `json:"audioCodec"`
	SampleRate    int     `json:"sampleRate"`
	BitsPerSample int     `json:"bitsPerSample"`
	Channels      int     `json:"channels"`
	HasVideo      bool    `json:"hasVideo"`
	HasAudio      bool    `json:"hasAudio"`
	HasCover      bool    `json:"hasCover"`
	Error         string  `json:"error"`
}

const maxFiles = 5000

// collect expands folders (recursively) and filters by extension.
func collect(kind string, paths []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		k := strings.ToLower(filepath.Clean(p))
		if !seen[k] && len(out) < maxFiles {
			seen[k] = true
			out = append(out, p)
		}
	}
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if !st.IsDir() {
			if acceptExt(kind, filepath.Ext(p)) {
				add(p)
			}
			continue
		}
		_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if path != p && (strings.HasPrefix(name, ".") || strings.EqualFold(name, naming.SubfolderName)) {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(name, ".") || strings.Contains(name, ".kmb-part.") {
				return nil
			}
			if acceptExt(kind, filepath.Ext(name)) {
				add(path)
			}
			if len(out) >= maxFiles {
				return fs.SkipAll
			}
			return nil
		})
	}
	return out
}

// AddPaths turns dropped/picked paths into file items with media info.
func (a *App) AddPaths(kind string, paths []string) []FileItem {
	files := collect(kind, paths)
	items := make([]FileItem, len(files))
	ffprobe := a.tools.Path(tools.FFprobe)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i, p := range files {
		wg.Add(1)
		go func(i int, p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			items[i] = a.describe(kind, p, ffprobe)
		}(i, p)
	}
	wg.Wait()
	return items
}

var itemSeq struct {
	sync.Mutex
	n int
}

func nextItemID() string {
	itemSeq.Lock()
	defer itemSeq.Unlock()
	itemSeq.n++
	return fmt.Sprintf("f%d", itemSeq.n)
}

func (a *App) describe(kind, path, ffprobe string) FileItem {
	it := FileItem{ID: nextItemID(), Path: path, Name: filepath.Base(path), Ext: strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")}
	if st, err := os.Stat(path); err == nil {
		it.Size = st.Size()
	}
	if kind == queue.KindImage {
		size, format, err := imageconv.Config(path)
		if err != nil {
			// ICO and exotic formats may still convert through the fallback decoders.
			if it.Ext != "ico" {
				it.Error = i18n.L("Format tidak dikenali atau file rusak", "Unknown format or damaged file")
			}
			return it
		}
		it.Width, it.Height, it.Format = size.W, size.H, format
		return it
	}
	if ffprobe == "" {
		it.Error = i18n.L("FFmpeg belum terpasang", "FFmpeg is not installed")
		return it
	}
	info, err := ffmpeg.Probe(context.Background(), ffprobe, path)
	if err != nil {
		it.Error = i18n.L("File tidak bisa dibaca", "File can't be read")
		return it
	}
	it.Width, it.Height, it.Duration = info.Width, info.Height, info.Duration
	it.Format, it.VideoCodec, it.FPS = info.Format, info.VideoCodec, info.FPS
	it.AudioCodec, it.SampleRate, it.BitsPerSample, it.Channels = info.AudioCodec, info.SampleRate, info.BitsPerSample, info.Channels
	it.HasVideo, it.HasAudio, it.HasCover = info.HasVideo, info.HasAudio, info.CoverIndex >= 0
	switch {
	case kind == queue.KindVideo && !info.HasVideo:
		it.Error = i18n.L("Tidak ada video di file ini", "This file has no video")
	case kind == queue.KindAudio && !info.HasAudio:
		it.Error = i18n.L("Tidak ada audio di file ini", "This file has no audio")
	}
	return it
}

var dialogFilters = map[string]wruntime.FileFilter{
	queue.KindImage: {DisplayName: "Images", Pattern: "*.jpg;*.jpeg;*.jfif;*.png;*.webp;*.gif;*.bmp;*.tif;*.tiff;*.heic;*.heif;*.avif;*.ico"},
	queue.KindVideo: {DisplayName: "Video", Pattern: "*.mp4;*.mkv;*.mov;*.avi;*.webm;*.flv;*.wmv;*.3gp;*.ts;*.m4v;*.mpg;*.mpeg;*.mts;*.m2ts;*.ogv;*.vob"},
	queue.KindAudio: {DisplayName: "Audio & video", Pattern: "*.mp3;*.wav;*.flac;*.aac;*.m4a;*.ogg;*.opus;*.wma;*.aiff;*.aif;*.amr;*.ape;*.wv;*.mka;*.oga;*.mp4;*.mkv;*.mov;*.avi;*.webm;*.flv;*.wmv;*.m4v"},
}

// PickFiles opens a multi-file dialog for a converter page.
func (a *App) PickFiles(kind string) ([]FileItem, error) {
	filter, ok := dialogFilters[kind]
	if !ok {
		return nil, errors.New(i18n.L("jenis tidak dikenal", "unknown kind"))
	}
	switch kind {
	case queue.KindImage:
		filter.DisplayName = i18n.L("Gambar", "Images")
	case queue.KindAudio:
		filter.DisplayName = i18n.L("Audio & video", "Audio & video")
	}
	paths, err := wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:   i18n.L("Pilih file", "Choose files"),
		Filters: []wruntime.FileFilter{filter, {DisplayName: i18n.L("Semua file", "All files"), Pattern: "*.*"}},
	})
	if err != nil || len(paths) == 0 {
		return []FileItem{}, err
	}
	return a.AddPaths(kind, paths), nil
}

// PickFolder opens a folder dialog and adds every matching file inside it.
func (a *App) PickFolder(kind string) ([]FileItem, error) {
	dir, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: i18n.L("Pilih folder", "Choose a folder")})
	if err != nil || dir == "" {
		return []FileItem{}, err
	}
	return a.AddPaths(kind, []string{dir}), nil
}

// JobItem identifies a file to convert.
type JobItem struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// JobRef links a page item to its queue task.
type JobRef struct {
	ItemID string `json:"itemId"`
	TaskID string `json:"taskId"`
}

// resolveOutput returns the module's result folder spec, making sure a fixed folder is usable.
func (a *App) resolveOutput(kind string) (naming.OutputSpec, error) {
	out := a.outputSpec(kind)
	if out.Mode == "custom" {
		if strings.TrimSpace(out.Dir) == "" {
			return out, errors.New(i18n.L("Pilih folder hasil terlebih dulu", "Choose an output folder first"))
		}
		if err := os.MkdirAll(out.Dir, 0o755); err != nil {
			return out, fmt.Errorf(i18n.L("Folder hasil tidak bisa dipakai (%s): %w", "Output folder can't be used (%s): %w"), out.Dir, err)
		}
	}
	return out, nil
}

// convertTask wraps the reserve → temp → commit dance shared by all converters.
func (a *App) convertTask(item JobItem, out naming.OutputSpec, ext string, work func(ctx context.Context, tmp string, r queue.Reporter) error) queue.RunFunc {
	return func(ctx context.Context, r queue.Reporter) error {
		if _, err := os.Stat(item.Path); err != nil {
			return queue.Fail(i18n.L("File asli tidak ditemukan (dipindah atau dihapus?)", "Source file not found (moved or deleted?)"), err.Error())
		}
		s := a.cfg.Get()
		target, release, err := a.namer.Reserve(item.Path, out, s.Suffix, ext, s.Conflict)
		if errors.Is(err, naming.ErrExists) {
			r.SetOutput(target, 0)
			return queue.Skip(i18n.L("File hasil sudah ada", "Output file already exists"))
		}
		if err != nil {
			return queue.Fail(err.Error(), "")
		}
		defer release()
		tmp := naming.TempPath(target)
		defer os.Remove(tmp)
		if err := work(ctx, tmp, r); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if err := naming.Commit(tmp, target); err != nil {
			return queue.Fail(i18n.L("Tidak bisa menyimpan file hasil", "Can't save the output file"), err.Error())
		}
		st, err := os.Stat(target)
		if err != nil {
			return queue.Fail(i18n.L("File hasil hilang setelah disimpan", "Output file disappeared after saving"), err.Error())
		}
		r.SetOutput(target, st.Size())
		return nil
	}
}

// StartImage queues image conversions.
func (a *App) StartImage(items []JobItem, o imageconv.Options) ([]JobRef, error) {
	o.Normalize()
	out, err := a.resolveOutput(queue.KindImage)
	if err != nil {
		return nil, err
	}
	ffmpegPath := a.tools.Path(tools.FFmpeg)
	specs := make([]queue.Spec, len(items))
	for i, it := range items {
		it := it
		specs[i] = queue.Spec{Title: filepath.Base(it.Path), Run: a.convertTask(it, out, o.Format, func(ctx context.Context, tmp string, r queue.Reporter) error {
			r.Message(i18n.L("Mengonversi…", "Converting…"))
			if err := imageconv.Convert(ctx, it.Path, tmp, o, ffmpegPath, r.Progress); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return queue.Fail(err.Error(), err.Error())
			}
			return nil
		})}
	}
	return refs(items, a.queue.AddMany(queue.KindImage, specs)), nil
}

// VideoJob is what the Video page sends.
type VideoJob struct {
	Mode  string                 `json:"mode"` // video | audio
	Video mediaconv.VideoOptions `json:"video"`
	Audio mediaconv.AudioOptions `json:"audio"`
}

// StartVideo queues video conversions (or audio extraction).
func (a *App) StartVideo(items []JobItem, job VideoJob) ([]JobRef, error) {
	out, err := a.resolveOutput(queue.KindVideo)
	if err != nil {
		return nil, err
	}
	ff, probe := a.tools.Path(tools.FFmpeg), a.tools.Path(tools.FFprobe)
	if ff == "" || probe == "" {
		return nil, errors.New(i18n.L("FFmpeg belum terpasang. Buka Pengaturan untuk mengunduhnya.", "FFmpeg is not installed. Open Settings to download it."))
	}
	job.Video.Normalize()
	job.Audio.Normalize()
	enc := a.encoders()
	if job.Mode != "audio" && job.Video.Codec != "copy" && job.Video.Format != "gif" && mediaconv.PickEncoder(job.Video.Codec, enc) == "" {
		return nil, fmt.Errorf(i18n.L("Encoder %s tidak tersedia di FFmpeg ini. Pilih codec lain.", "The %s encoder isn't available in this FFmpeg. Choose another codec."), strings.ToUpper(job.Video.Codec))
	}
	kind := queue.KindVideo
	specs := make([]queue.Spec, len(items))
	for i, it := range items {
		it := it
		ext := job.Video.Format
		if job.Mode == "audio" {
			ext = job.Audio.Format
		}
		specs[i] = queue.Spec{Title: filepath.Base(it.Path), Run: a.convertTask(it, out, ext, func(ctx context.Context, tmp string, r queue.Reporter) error {
			info, err := ffmpeg.Probe(ctx, probe, it.Path)
			if err != nil {
				return queue.Fail(i18n.L("File tidak bisa dibaca", "File can't be read"), err.Error())
			}
			var args []string
			if job.Mode == "audio" {
				args, err = mediaconv.AudioArgs(it.Path, tmp, info, job.Audio)
			} else {
				args, err = mediaconv.VideoArgs(it.Path, tmp, info, job.Video, enc)
			}
			if err != nil {
				return queue.Fail(err.Error(), "")
			}
			return runFFmpeg(ctx, ff, args, info.Duration, r)
		})}
	}
	return refs(items, a.queue.AddMany(kind, specs)), nil
}

// StartAudio queues audio conversions.
func (a *App) StartAudio(items []JobItem, o mediaconv.AudioOptions) ([]JobRef, error) {
	out, err := a.resolveOutput(queue.KindAudio)
	if err != nil {
		return nil, err
	}
	ff, probe := a.tools.Path(tools.FFmpeg), a.tools.Path(tools.FFprobe)
	if ff == "" || probe == "" {
		return nil, errors.New(i18n.L("FFmpeg belum terpasang. Buka Pengaturan untuk mengunduhnya.", "FFmpeg is not installed. Open Settings to download it."))
	}
	o.Normalize()
	specs := make([]queue.Spec, len(items))
	for i, it := range items {
		it := it
		specs[i] = queue.Spec{Title: filepath.Base(it.Path), Run: a.convertTask(it, out, o.Format, func(ctx context.Context, tmp string, r queue.Reporter) error {
			info, err := ffmpeg.Probe(ctx, probe, it.Path)
			if err != nil {
				return queue.Fail(i18n.L("File tidak bisa dibaca", "File can't be read"), err.Error())
			}
			args, err := mediaconv.AudioArgs(it.Path, tmp, info, o)
			if err != nil {
				return queue.Fail(err.Error(), "")
			}
			return runFFmpeg(ctx, ff, args, info.Duration, r)
		})}
	}
	return refs(items, a.queue.AddMany(queue.KindAudio, specs)), nil
}

func runFFmpeg(ctx context.Context, ff string, args []string, duration float64, r queue.Reporter) error {
	r.Message(i18n.L("Mengonversi…", "Converting…"))
	return ffmpeg.Run(ctx, ff, args, duration, func(p float64, speed string) {
		r.Progress(p)
		if speed != "" && speed != "N/A" {
			r.Message(i18n.L("Mengonversi · ", "Converting · ") + speed)
		}
	})
}

func refs(items []JobItem, ids []string) []JobRef {
	out := make([]JobRef, len(items))
	for i := range items {
		out[i] = JobRef{ItemID: items[i].ID, TaskID: ids[i]}
	}
	return out
}
