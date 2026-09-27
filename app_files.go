package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/config"
	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/mediaconv"
	"kuymediabox/internal/naming"
	"kuymediabox/internal/pdf"
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
	case kindPDF:
		return ext == ".pdf"
	case kindPDFImage:
		return imageconv.InputExts[ext]
	case kindWord, kindExcel, kindPPT:
		return pdf.OfficeExts[ext] == officeFamily[kind]
	case kindHTML:
		return ext == ".html" || ext == ".htm" || ext == ".mhtml" || ext == ".svg"
	case queue.KindImage:
		return imageconv.InputExts[ext]
	case queue.KindVideo:
		return videoExts[ext]
	case queue.KindAudio, queue.KindSubtitle:
		return audioExts[ext] || videoExts[ext]
	}
	return false
}

// FileItem is a file added to a converter page.
type FileItem struct {
	ID            string            `json:"id"`
	Path          string            `json:"path"`
	Name          string            `json:"name"`
	Ext           string            `json:"ext"`
	Size          int64             `json:"size"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	Duration      float64           `json:"duration"`
	Format        string            `json:"format"`
	VideoCodec    string            `json:"videoCodec"`
	FPS           float64           `json:"fps"`
	AudioCodec    string            `json:"audioCodec"`
	SampleRate    int               `json:"sampleRate"`
	BitsPerSample int               `json:"bitsPerSample"`
	Channels      int               `json:"channels"`
	HasVideo      bool              `json:"hasVideo"`
	HasAudio      bool              `json:"hasAudio"`
	HasCover      bool              `json:"hasCover"`
	Pages         int               `json:"pages"`
	Encrypted     bool              `json:"encrypted"` // PDF uses a password or permissions
	Locked        bool              `json:"locked"`    // PDF needs a password to open
	SubCodec      string            `json:"subCodec"`  // first subtitle track inside the file
	SubFile       string            `json:"subFile"`   // subtitle file next to the video
	Tags          map[string]string `json:"tags"`      // audio: title, artist, album, …
	Cue           string            `json:"cue"`       // audio: cue sheet next to the file
	CueTracks     int               `json:"cueTracks"` // number of songs in the cue sheet
	Error         string            `json:"error"`
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
	a.waitTools()
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
	switch kind {
	case kindPDF:
		info, err := pdf.Inspect(context.Background(), path)
		if err != nil {
			it.Error = i18n.L("Bukan PDF yang valid atau file rusak", "Not a valid PDF or damaged file")
			return it
		}
		it.Pages, it.Encrypted, it.Locked = info.Pages, info.Encrypted, info.Locked
		it.Width, it.Height = int(info.Width), int(info.Height)
		return it
	case kindWord, kindExcel, kindPPT, kindHTML:
		return it
	}
	if kind == queue.KindImage || kind == kindPDFImage {
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
	it.SubCodec, it.Tags = info.SubCodec, map[string]string{}
	for _, k := range []string{"title", "artist", "album", "album_artist", "date", "genre", "track"} {
		if v := info.Tags[k]; v != "" {
			it.Tags[k] = v
		}
	}
	if kind == queue.KindVideo {
		it.SubFile = mediaconv.FindSubtitle(path)
	}
	if kind == queue.KindAudio && !info.HasVideo {
		if cue := mediaconv.FindCue(path); cue != "" {
			if c, err := mediaconv.ParseCue(cue); err == nil {
				it.Cue, it.CueTracks = cue, len(c.Tracks)
			}
		}
	}
	switch {
	case kind == queue.KindVideo && !info.HasVideo:
		it.Error = i18n.L("Tidak ada video di file ini", "This file has no video")
	case (kind == queue.KindAudio || kind == queue.KindSubtitle) && !info.HasAudio:
		it.Error = i18n.L("Tidak ada audio di file ini", "This file has no audio")
	}
	return it
}

var dialogFilters = map[string]wruntime.FileFilter{
	queue.KindImage:    {DisplayName: "Images", Pattern: "*.jpg;*.jpeg;*.jfif;*.png;*.webp;*.gif;*.bmp;*.tif;*.tiff;*.heic;*.heif;*.avif;*.ico"},
	queue.KindVideo:    {DisplayName: "Video", Pattern: "*.mp4;*.mkv;*.mov;*.avi;*.webm;*.flv;*.wmv;*.3gp;*.ts;*.m4v;*.mpg;*.mpeg;*.mts;*.m2ts;*.ogv;*.vob"},
	queue.KindAudio:    {DisplayName: "Audio & video", Pattern: "*.mp3;*.wav;*.flac;*.aac;*.m4a;*.ogg;*.opus;*.wma;*.aiff;*.aif;*.amr;*.ape;*.wv;*.mka;*.oga;*.mp4;*.mkv;*.mov;*.avi;*.webm;*.flv;*.wmv;*.m4v"},
	queue.KindSubtitle: {DisplayName: "Video & audio", Pattern: "*.mp4;*.mkv;*.mov;*.avi;*.webm;*.flv;*.wmv;*.3gp;*.ts;*.m4v;*.mpg;*.mpeg;*.mts;*.m2ts;*.mp3;*.wav;*.flac;*.aac;*.m4a;*.ogg;*.opus;*.wma"},
	kindPDF:            {DisplayName: "PDF", Pattern: "*.pdf"},
	kindPDFImage:       {DisplayName: "Images", Pattern: "*.jpg;*.jpeg;*.jfif;*.png;*.webp;*.gif;*.bmp;*.tif;*.tiff;*.heic;*.heif;*.avif"},
	kindWord:           {DisplayName: "Word", Pattern: "*.doc;*.docx;*.docm;*.dot;*.dotx;*.odt;*.rtf;*.wpd"},
	kindExcel:          {DisplayName: "Excel", Pattern: "*.xls;*.xlsx;*.xlsm;*.xlsb;*.ods;*.csv"},
	kindPPT:            {DisplayName: "PowerPoint", Pattern: "*.ppt;*.pptx;*.pptm;*.pps;*.ppsx;*.odp"},
	kindHTML:           {DisplayName: "HTML", Pattern: "*.html;*.htm;*.mhtml;*.svg"},
}

// PickFiles opens a multi-file dialog for a converter page.
func (a *App) PickFiles(kind string) ([]FileItem, error) {
	filter, ok := dialogFilters[kind]
	if !ok {
		return nil, errors.New(i18n.L("jenis tidak dikenal", "unknown kind"))
	}
	switch kind {
	case queue.KindImage, kindPDFImage:
		filter.DisplayName = i18n.L("Gambar", "Images")
	case queue.KindAudio, queue.KindSubtitle:
		filter.DisplayName = i18n.L("Video & audio", "Video & audio")
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
	ID   string          `json:"id"`
	Path string          `json:"path"`
	Tags *mediaconv.Tags `json:"tags,omitempty"` // audio tag editor
	// Set by workflows and the command line: result folder for this file instead of the
	// module's, and Plain keeps the source name (no suffix) for in-between results.
	OutDir string `json:"outDir,omitempty"`
	Plain  bool   `json:"plain,omitempty"`
}

// itemOut applies a per-file result folder.
func itemOut(out naming.OutputSpec, dir string) naming.OutputSpec {
	if dir == "" {
		return out
	}
	return naming.OutputSpec{Mode: config.OutputCustom, Dir: dir}
}

func fileSize(p string) int64 {
	if st, err := os.Stat(p); err == nil {
		return st.Size()
	}
	return 0
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
	return a.convertTaskSuffix(item, out, "", ext, work)
}

// convertTaskSuffix is convertTask with a fixed name suffix ("" = the user's suffix).
func (a *App) convertTaskSuffix(item JobItem, out naming.OutputSpec, suffix, ext string, work func(ctx context.Context, tmp string, r queue.Reporter) error) queue.RunFunc {
	return func(ctx context.Context, r queue.Reporter) error {
		if _, err := os.Stat(item.Path); err != nil {
			return queue.Fail(i18n.L("File asli tidak ditemukan (dipindah atau dihapus?)", "Source file not found (moved or deleted?)"), err.Error())
		}
		s := a.cfg.Get()
		if suffix == "" {
			suffix = s.Suffix
		}
		if item.Plain {
			suffix = ""
		}
		target, release, err := a.namer.Reserve(item.Path, itemOut(out, item.OutDir), suffix, ext, s.Conflict)
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
	a.waitTools()
	o.Normalize()
	out, err := a.resolveOutput(queue.KindImage)
	if err != nil {
		return nil, err
	}
	ffmpegPath := a.tools.Path(tools.FFmpeg)
	specs := make([]queue.Spec, len(items))
	for i, it := range items {
		it := it
		specs[i] = queue.Spec{Title: filepath.Base(it.Path), Input: it.Path, InSize: fileSize(it.Path), Run: a.convertTask(it, out, o.Format, func(ctx context.Context, tmp string, r queue.Reporter) error {
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
	Mode        string                 `json:"mode"` // video | audio | merge | frames
	Video       mediaconv.VideoOptions `json:"video"`
	Audio       mediaconv.AudioOptions `json:"audio"`
	FrameEvery  float64                `json:"frameEvery"`  // frames: seconds between pictures
	FrameFormat string                 `json:"frameFormat"` // frames: jpg | png
	Sheet       SheetOptions           `json:"sheet"`       // sheet: contact sheet layout
}

// SheetOptions lay out a contact sheet (a grid of pictures from the video).
type SheetOptions struct {
	Cols   int    `json:"cols"`   // 2 … 10
	Rows   int    `json:"rows"`   // 1 … 20
	Width  int    `json:"width"`  // width of one picture in pixels
	Format string `json:"format"` // jpg | png
	Times  bool   `json:"times"`  // timestamp on every picture
}

func errNoFFmpeg() error {
	return errors.New(i18n.L("FFmpeg belum terpasang. Buka Pengaturan untuk mengunduhnya.", "FFmpeg is not installed. Open Settings to download it."))
}

// StartVideo queues video conversions (or audio extraction, joining, frame export).
func (a *App) StartVideo(items []JobItem, job VideoJob) ([]JobRef, error) {
	a.waitTools()
	out, err := a.resolveOutput(queue.KindVideo)
	if err != nil {
		return nil, err
	}
	ff, probe := a.tools.Path(tools.FFmpeg), a.tools.Path(tools.FFprobe)
	if ff == "" || probe == "" {
		return nil, errNoFFmpeg()
	}
	job.Video.Normalize()
	job.Audio.Normalize()
	if job.Audio.Format == mediaconv.FormatOriginal {
		job.Audio.Format = "m4a"
	}
	enc := a.encoders()
	if job.Video.HW != "" {
		enc = a.allEncoders()
	}
	if job.Mode != "audio" && job.Mode != "frames" && job.Mode != "sheet" && job.Video.Codec != "copy" && job.Video.Format != "gif" {
		if job.Video.HW != "" {
			if e := mediaconv.HWEncoder(job.Video.HW, job.Video.Codec); e == "" || !enc[e] {
				return nil, fmt.Errorf(i18n.L("Akselerasi GPU %s tidak mendukung codec %s di PC ini", "GPU acceleration %s doesn't support %s on this PC"), strings.ToUpper(job.Video.HW), strings.ToUpper(job.Video.Codec))
			}
		} else if mediaconv.PickEncoder(job.Video.Codec, enc) == "" {
			return nil, fmt.Errorf(i18n.L("Encoder %s tidak tersedia di FFmpeg ini. Pilih codec lain.", "The %s encoder isn't available in this FFmpeg. Choose another codec."), strings.ToUpper(job.Video.Codec))
		}
	}
	if err := checkTrim(job.Video.TrimStart, job.Video.TrimEnd); err != nil {
		return nil, err
	}
	if err := checkMusic(job.Video); err != nil {
		return nil, err
	}
	if job.Mode == "merge" {
		return a.startMergeVideo(items, job, out, ff, probe, enc)
	}
	specs := make([]queue.Spec, len(items))
	for i, it := range items {
		it := it
		spec := queue.Spec{Title: filepath.Base(it.Path), Input: it.Path, InSize: fileSize(it.Path)}
		if job.Mode == "frames" {
			spec.Run = a.framesTask(it, out, job, ff, probe)
			specs[i] = spec
			continue
		}
		if job.Mode == "sheet" {
			spec.Run = a.sheetTask(it, out, job.Sheet, ff, probe)
			specs[i] = spec
			continue
		}
		ext := job.Video.Format
		if job.Mode == "audio" {
			ext = job.Audio.Format
		}
		spec.Run = a.convertTask(it, out, ext, func(ctx context.Context, tmp string, r queue.Reporter) error {
			info, err := ffmpeg.Probe(ctx, probe, it.Path)
			if err != nil {
				return queue.Fail(i18n.L("File tidak bisa dibaca", "File can't be read"), err.Error())
			}
			if job.Mode == "audio" {
				o := job.Audio
				o.TrimStart, o.TrimEnd = job.Video.TrimStart, job.Video.TrimEnd
				plan, err := mediaconv.AudioPlan(it.Path, tmp, info, o, nil, 0, 0)
				if err != nil {
					return queue.Fail(err.Error(), "")
				}
				return runPlan(ctx, ff, plan, r)
			}
			vo := job.Video
			if vo.Subtitles != "none" {
				vo.SubFile = mediaconv.FindSubtitle(it.Path)
				if vo.SubFile == "" && info.SubCodec == "" {
					r.Message(i18n.L("Tidak ada subtitle — dikonversi tanpa subtitle", "No subtitles found — converting without them"))
				}
			}
			work, err := os.MkdirTemp(appdir.TempDir(), "vid-*")
			if err != nil {
				return queue.Fail(err.Error(), "")
			}
			defer os.RemoveAll(work)
			if vo.Watermark.Enabled {
				w, h := mediaconv.OutputSize(info.Width, info.Height, vo)
				if vo.WatermarkFile, err = watermarkLayer(vo.Watermark, w, h, work, ff); err != nil {
					return err
				}
			}
			plan, err := mediaconv.VideoPlan(it.Path, tmp, info, vo, enc, work)
			if err != nil {
				return queue.Fail(err.Error(), "")
			}
			return runPlan(ctx, ff, plan, r)
		})
		specs[i] = spec
	}
	return refs(items, a.queue.AddMany(queue.KindVideo, specs)), nil
}

// watermarkLayer draws the watermark at the video size into work.
func watermarkLayer(w imageconv.Watermark, width, height int, work, ff string) (string, error) {
	out := filepath.Join(work, "watermark.png")
	if err := mediaconv.WatermarkLayer(w, width, height, out, ff); err != nil {
		return "", queue.Fail(i18n.L("Watermark tidak bisa dibuat (logo rusak atau tidak ditemukan?)", "The watermark can't be made (logo damaged or missing?)"), err.Error())
	}
	return out, nil
}

// checkMusic makes sure the background music of a video job can be read.
func checkMusic(o mediaconv.VideoOptions) error {
	if o.Music.File == "" {
		return nil
	}
	if _, err := os.Stat(o.Music.File); err != nil {
		return fmt.Errorf(i18n.L("File musik latar tidak ditemukan: %s", "Background music file not found: %s"), filepath.Base(o.Music.File))
	}
	return nil
}

func checkTrim(start, end string) error {
	s, ok1 := mediaconv.ParseTime(start)
	e, ok2 := mediaconv.ParseTime(end)
	if !ok1 || !ok2 {
		return errors.New(i18n.L("Format waktu potong tidak valid. Contoh: 1:30 atau 00:01:30", "Invalid trim time. Example: 1:30 or 00:01:30"))
	}
	if e > 0 && e <= s {
		return errors.New(i18n.L("Waktu akhir harus setelah waktu mulai", "The end time must be after the start time"))
	}
	return nil
}

// framesTask saves pictures from a video into a folder "<name>_frames".
func (a *App) framesTask(it JobItem, out naming.OutputSpec, job VideoJob, ff, probe string) queue.RunFunc {
	format := job.FrameFormat
	if format != "png" {
		format = "jpg"
	}
	suffix := i18n.L("_bingkai", "_frames")
	return a.folderTask(it.Path, itemOut(out, it.OutDir), suffix, func(ctx context.Context, dir, name string, r queue.Reporter) ([]string, error) {
		info, err := ffmpeg.Probe(ctx, probe, it.Path)
		if err != nil {
			return nil, queue.Fail(i18n.L("File tidak bisa dibaca", "File can't be read"), err.Error())
		}
		plan, err := mediaconv.FramesPlan(it.Path, filepath.Join(dir, name+"_%04d."+format), info, job.Video, job.FrameEvery, format)
		if err != nil {
			return nil, queue.Fail(err.Error(), "")
		}
		if err := runPlan(ctx, ff, plan, r); err != nil {
			return nil, err
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*."+format))
		return files, nil
	})
}

// sheetTask saves a contact sheet: pictures spread evenly over the video in a grid.
func (a *App) sheetTask(it JobItem, out naming.OutputSpec, o SheetOptions, ff, probe string) queue.RunFunc {
	o.Cols = max(2, min(o.Cols, 10))
	o.Rows = max(1, min(o.Rows, 20))
	o.Width = max(160, min(o.Width, 960))
	if o.Format != "png" {
		o.Format = "jpg"
	}
	return a.convertTaskSuffix(it, out, i18n.L("_lembar", "_sheet"), o.Format, func(ctx context.Context, tmp string, r queue.Reporter) error {
		info, err := ffmpeg.Probe(ctx, probe, it.Path)
		if err != nil {
			return queue.Fail(i18n.L("File tidak bisa dibaca", "File can't be read"), err.Error())
		}
		if !info.HasVideo || info.Duration <= 0 {
			return queue.Fail(i18n.L("Durasi video tidak diketahui", "Unknown video duration"), "")
		}
		work, err := os.MkdirTemp(appdir.TempDir(), "sheet-*")
		if err != nil {
			return queue.Fail(err.Error(), "")
		}
		defer os.RemoveAll(work)
		n := o.Cols * o.Rows
		cells := make([]imageconv.SheetCell, 0, n)
		r.Message(i18n.L("Mengambil gambar…", "Taking pictures…"))
		for i := 0; i < n; i++ {
			t := info.Duration * (float64(i) + 0.5) / float64(n)
			pic := filepath.Join(work, fmt.Sprintf("%03d.png", i))
			args := []string{"-ss", strconv.FormatFloat(t, 'f', 3, 64), "-i", it.Path, "-frames:v", "1", "-vf", fmt.Sprintf("scale=%d:-2", o.Width), pic}
			if err := ffmpeg.RunIn(ctx, ff, work, args, 0, nil); err != nil {
				return err
			}
			if _, err := os.Stat(pic); err != nil {
				continue // past the last frame
			}
			label := ""
			if o.Times {
				label = clock(t)
			}
			cells = append(cells, imageconv.SheetCell{Path: pic, Label: label})
			r.Progress(float64(i+1) / float64(n+1))
		}
		header := []string{filepath.Base(it.Path), fmt.Sprintf("%s · %d×%d · %s · %s", clock(info.Duration), info.Width, info.Height, strings.ToUpper(info.VideoCodec), humanSize(fileSize(it.Path)))}
		r.Message(i18n.L("Menyusun lembar…", "Building the sheet…"))
		if err := imageconv.ContactSheet(cells, header, o.Cols, o.Width, tmp, o.Format); err != nil {
			return queue.Fail(i18n.L("Lembar kontak tidak bisa dibuat", "The contact sheet can't be made"), err.Error())
		}
		return nil
	})
}

// clock formats seconds as h:mm:ss or m:ss.
func clock(sec float64) string {
	s := int(sec + 0.5)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s%3600/60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%d KB", n>>10)
	}
}

// startMergeVideo joins all items into one video named after the first one.
func (a *App) startMergeVideo(items []JobItem, job VideoJob, out naming.OutputSpec, ff, probe string, enc map[string]bool) ([]JobRef, error) {
	if len(items) < 2 {
		return nil, errors.New(i18n.L("Tambahkan minimal 2 video untuk digabung", "Add at least 2 videos to join"))
	}
	paths := make([]string, len(items))
	var size int64
	for i, it := range items {
		paths[i] = it.Path
		size += fileSize(it.Path)
	}
	first := items[0]
	run := a.convertTaskSuffix(first, out, i18n.L("_gabungan", "_joined"), job.Video.Format, func(ctx context.Context, tmp string, r queue.Reporter) error {
		infos, err := probeAll(ctx, probe, paths)
		if err != nil {
			return err
		}
		vo := job.Video
		if vo.Watermark.Enabled {
			work, err := os.MkdirTemp(appdir.TempDir(), "vid-*")
			if err != nil {
				return queue.Fail(err.Error(), "")
			}
			defer os.RemoveAll(work)
			w, h := mediaconv.OutputSize(infos[0].Width, infos[0].Height, vo)
			if vo.WatermarkFile, err = watermarkLayer(vo.Watermark, w-w%2, h-h%2, work, ff); err != nil {
				return err
			}
		}
		plan, err := mediaconv.MergeVideoPlan(paths, infos, tmp, vo, enc)
		if err != nil {
			return queue.Fail(err.Error(), "")
		}
		return runPlan(ctx, ff, plan, r)
	})
	return a.addCombined(queue.KindVideo, items, size, run), nil
}

func probeAll(ctx context.Context, probe string, paths []string) ([]ffmpeg.Info, error) {
	infos := make([]ffmpeg.Info, len(paths))
	for i, p := range paths {
		info, err := ffmpeg.Probe(ctx, probe, p)
		if err != nil {
			return nil, queue.Fail(fmt.Sprintf(i18n.L("File tidak bisa dibaca: %s", "File can't be read: %s"), filepath.Base(p)), err.Error())
		}
		infos[i] = info
	}
	return infos, nil
}

// addCombined queues one task made from all items; every item points at it.
func (a *App) addCombined(kind string, items []JobItem, size int64, run queue.RunFunc) []JobRef {
	title := fmt.Sprintf("%s (+%d)", filepath.Base(items[0].Path), len(items)-1)
	id := a.queue.AddMany(kind, []queue.Spec{{Title: title, Input: items[0].Path, InSize: size, Run: run}})[0]
	res := make([]JobRef, len(items))
	for i, it := range items {
		res[i] = JobRef{ItemID: it.ID, TaskID: id}
	}
	return res
}

// AudioJob is what the Audio page sends.
type AudioJob struct {
	Mode    string                 `json:"mode"` // convert | merge
	Options mediaconv.AudioOptions `json:"options"`
}

// StartAudio queues audio conversions or one join of all items.
func (a *App) StartAudio(items []JobItem, job AudioJob) ([]JobRef, error) {
	a.waitTools()
	out, err := a.resolveOutput(queue.KindAudio)
	if err != nil {
		return nil, err
	}
	ff, probe := a.tools.Path(tools.FFmpeg), a.tools.Path(tools.FFprobe)
	if ff == "" || probe == "" {
		return nil, errNoFFmpeg()
	}
	o := job.Options
	o.Normalize()
	if err := checkTrim(o.TrimStart, o.TrimEnd); err != nil {
		return nil, err
	}
	if job.Mode == "merge" {
		return a.startMergeAudio(items, o, out, ff, probe)
	}
	if job.Mode == "cue" {
		if o.Format == mediaconv.FormatOriginal {
			return nil, errors.New(i18n.L("Pilih format hasil untuk memisah lagu", "Pick an output format to split the tracks"))
		}
		specs := make([]queue.Spec, len(items))
		for i, it := range items {
			specs[i] = queue.Spec{Title: filepath.Base(it.Path), Input: it.Path, InSize: fileSize(it.Path), Run: a.cueTask(it, o, out, ff, probe)}
		}
		return refs(items, a.queue.AddMany(queue.KindAudio, specs)), nil
	}
	specs := make([]queue.Spec, len(items))
	for i, it := range items {
		it := it
		ext := o.Format
		if ext == mediaconv.FormatOriginal {
			ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(it.Path)), ".")
		}
		specs[i] = queue.Spec{Title: filepath.Base(it.Path), Input: it.Path, InSize: fileSize(it.Path), Run: a.convertTask(it, out, ext, func(ctx context.Context, tmp string, r queue.Reporter) error {
			info, err := ffmpeg.Probe(ctx, probe, it.Path)
			if err != nil {
				return queue.Fail(i18n.L("File tidak bisa dibaca", "File can't be read"), err.Error())
			}
			var lead, trail float64
			if o.RemoveSilence {
				r.Message(i18n.L("Mencari bagian hening…", "Looking for silence…"))
				r.Progress(-1)
				if lead, trail, err = ffmpeg.Silence(ctx, ff, it.Path, -50, info.Duration); err != nil {
					return err
				}
			}
			plan, err := mediaconv.AudioPlan(it.Path, tmp, info, o, it.Tags, lead, trail)
			if err != nil {
				return queue.Fail(err.Error(), "")
			}
			return runPlan(ctx, ff, plan, r)
		})}
	}
	return refs(items, a.queue.AddMany(queue.KindAudio, specs)), nil
}

// cueTask splits an album file into one file per song of its cue sheet, with tags.
func (a *App) cueTask(it JobItem, o mediaconv.AudioOptions, out naming.OutputSpec, ff, probe string) queue.RunFunc {
	return a.folderTask(it.Path, itemOut(out, it.OutDir), "", func(ctx context.Context, dir, name string, r queue.Reporter) ([]string, error) {
		cuePath := mediaconv.FindCue(it.Path)
		if cuePath == "" {
			return nil, queue.Fail(i18n.L("File CUE tidak ditemukan di samping file ini", "No CUE file found next to this file"), "")
		}
		cue, err := mediaconv.ParseCue(cuePath)
		if err != nil {
			return nil, queue.Fail(err.Error(), "")
		}
		info, err := ffmpeg.Probe(ctx, probe, it.Path)
		if err != nil {
			return nil, queue.Fail(i18n.L("File tidak bisa dibaca", "File can't be read"), err.Error())
		}
		cover := findCover(filepath.Dir(it.Path))
		var files []string
		for i, t := range cue.Tracks {
			if t.Start >= info.Duration && info.Duration > 0 {
				break
			}
			r.Message(fmt.Sprintf(i18n.L("Lagu %d dari %d", "Track %d of %d"), i+1, len(cue.Tracks)))
			to := o
			to.TrimStart, to.TrimEnd = cue.TrackRange(i)
			tags := cue.TrackTags(i)
			tags.Cover = cover
			title := t.Title
			if title == "" {
				title = fmt.Sprintf(i18n.L("Lagu %d", "Track %d"), t.Number)
			}
			file := filepath.Join(dir, naming.SanitizeFileName(fmt.Sprintf("%02d - %s", t.Number, title))+"."+o.Format)
			plan, err := mediaconv.AudioPlan(it.Path, file, info, to, tags, 0, 0)
			if err != nil {
				return nil, queue.Fail(err.Error(), "")
			}
			n := len(cue.Tracks)
			err = ffmpeg.RunIn(ctx, ff, "", plan.Passes[0], plan.Duration, func(p float64, _ string) {
				if p >= 0 {
					r.Progress((float64(i) + p) / float64(n))
				}
			})
			if err != nil {
				return nil, err
			}
			files = append(files, file)
		}
		return files, nil
	})
}

// findCover returns the album picture of a folder (cover.jpg, folder.jpg, front.png, …).
func findCover(dir string) string {
	for _, n := range []string{"cover", "folder", "front", "album", "Cover", "Folder", "Front"} {
		for _, ext := range []string{".jpg", ".jpeg", ".png"} {
			if p := filepath.Join(dir, n+ext); exists(p) {
				return p
			}
		}
	}
	return ""
}

func (a *App) startMergeAudio(items []JobItem, o mediaconv.AudioOptions, out naming.OutputSpec, ff, probe string) ([]JobRef, error) {
	if len(items) < 2 {
		return nil, errors.New(i18n.L("Tambahkan minimal 2 file untuk digabung", "Add at least 2 files to join"))
	}
	if o.Format == mediaconv.FormatOriginal {
		return nil, errors.New(i18n.L("Pilih format hasil untuk menggabung audio", "Pick an output format to join audio"))
	}
	paths := make([]string, len(items))
	var size int64
	for i, it := range items {
		paths[i] = it.Path
		size += fileSize(it.Path)
	}
	run := a.convertTaskSuffix(items[0], out, i18n.L("_gabungan", "_joined"), o.Format, func(ctx context.Context, tmp string, r queue.Reporter) error {
		infos, err := probeAll(ctx, probe, paths)
		if err != nil {
			return err
		}
		plan, err := mediaconv.MergeAudioPlan(paths, infos, tmp, o)
		if err != nil {
			return queue.Fail(err.Error(), "")
		}
		return runPlan(ctx, ff, plan, r)
	})
	return a.addCombined(queue.KindAudio, items, size, run), nil
}

// runPlan executes a conversion plan, splitting progress over the passes.
func runPlan(ctx context.Context, ff string, plan mediaconv.Plan, r queue.Reporter) error {
	for _, args := range plan.Prep {
		if err := ffmpeg.RunIn(ctx, ff, plan.Dir, args, 0, nil); err != nil {
			return err
		}
	}
	n := len(plan.Passes)
	for i, args := range plan.Passes {
		i := i
		label := i18n.L("Mengonversi", "Converting")
		if n > 1 {
			label = fmt.Sprintf(i18n.L("Tahap %d dari %d", "Pass %d of %d"), i+1, n)
		}
		r.Message(label + "…")
		err := ffmpeg.RunIn(ctx, ff, plan.Dir, args, plan.Duration, func(p float64, speed string) {
			if p >= 0 {
				r.Progress((float64(i) + p) / float64(n))
			} else {
				r.Progress(-1)
			}
			if speed != "" && speed != "N/A" {
				r.Message(label + " · " + speed)
			}
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// startWatched converts new files of a watched folder with the rule's saved settings.
func (a *App) startWatched(rule config.WatchRule, paths []string) error {
	items := make([]JobItem, len(paths))
	for i, p := range paths {
		items[i] = JobItem{ID: nextItemID(), Path: p}
	}
	var err error
	switch rule.Kind {
	case queue.KindImage:
		var o imageconv.Options
		if err = json.Unmarshal(rule.Options, &o); err == nil {
			_, err = a.StartImage(items, o)
		}
	case queue.KindVideo:
		var job VideoJob
		if err = json.Unmarshal(rule.Options, &job); err == nil {
			if job.Mode == "merge" || job.Mode == "" {
				job.Mode = "video"
			}
			_, err = a.StartVideo(items, job)
		}
	case queue.KindAudio:
		var job AudioJob
		if err = json.Unmarshal(rule.Options, &job); err == nil {
			job.Mode = "convert"
			_, err = a.StartAudio(items, job)
		}
	case "flow":
		var o flowWatch
		if err = json.Unmarshal(rule.Options, &o); err == nil {
			_, err = a.RunWorkflow(o.Workflow, paths)
		}
	}
	return err
}

func refs(items []JobItem, ids []string) []JobRef {
	out := make([]JobRef, len(items))
	for i := range items {
		out[i] = JobRef{ItemID: items[i].ID, TaskID: ids[i]}
	}
	return out
}
