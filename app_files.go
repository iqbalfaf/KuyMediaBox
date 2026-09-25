package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
	case queue.KindAudio:
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
	kindPDF:         {DisplayName: "PDF", Pattern: "*.pdf"},
	kindPDFImage:    {DisplayName: "Images", Pattern: "*.jpg;*.jpeg;*.jfif;*.png;*.webp;*.gif;*.bmp;*.tif;*.tiff;*.heic;*.heif;*.avif"},
	kindWord:        {DisplayName: "Word", Pattern: "*.doc;*.docx;*.docm;*.dot;*.dotx;*.odt;*.rtf;*.wpd"},
	kindExcel:       {DisplayName: "Excel", Pattern: "*.xls;*.xlsx;*.xlsm;*.xlsb;*.ods;*.csv"},
	kindPPT:         {DisplayName: "PowerPoint", Pattern: "*.ppt;*.pptx;*.pptm;*.pps;*.ppsx;*.odp"},
	kindHTML:        {DisplayName: "HTML", Pattern: "*.html;*.htm;*.mhtml;*.svg"},
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
	ID   string          `json:"id"`
	Path string          `json:"path"`
	Tags *mediaconv.Tags `json:"tags,omitempty"` // audio tag editor
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
		target, release, err := a.namer.Reserve(item.Path, out, suffix, ext, s.Conflict)
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
}

func errNoFFmpeg() error {
	return errors.New(i18n.L("FFmpeg belum terpasang. Buka Pengaturan untuk mengunduhnya.", "FFmpeg is not installed. Open Settings to download it."))
}

// StartVideo queues video conversions (or audio extraction, joining, frame export).
func (a *App) StartVideo(items []JobItem, job VideoJob) ([]JobRef, error) {
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
	if job.Mode != "audio" && job.Mode != "frames" && job.Video.Codec != "copy" && job.Video.Format != "gif" {
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
	return a.folderTask(it.Path, out, suffix, func(ctx context.Context, dir, name string, r queue.Reporter) ([]string, error) {
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
		plan, err := mediaconv.MergeVideoPlan(paths, infos, tmp, job.Video, enc)
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
