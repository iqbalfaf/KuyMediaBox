package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"kuymediabox/internal/proc"
	"kuymediabox/internal/queue"
)

func (e Env) commonArgs() []string {
	args := []string{"--ignore-config", "--no-warnings", "--no-colors", "--encoding", "utf-8"}
	if e.JSPath != "" && e.JSKind != "" {
		args = append(args, "--js-runtimes", e.JSKind+":"+e.JSPath)
	}
	if e.FFmpeg != "" {
		args = append(args, "--ffmpeg-location", e.FFmpeg)
	}
	return args
}

type ytInfo struct {
	ID          string    `json:"id"`
	Type        string    `json:"_type"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	WebpageURL  string    `json:"webpage_url"`
	Duration    *float64  `json:"duration"`
	Channel     string    `json:"channel"`
	Uploader    string    `json:"uploader"`
	UploadDate  string    `json:"upload_date"`
	Timestamp   *float64  `json:"timestamp"`
	ReleaseTS   *float64  `json:"release_timestamp"`
	Thumbnail   string    `json:"thumbnail"`
	Thumbnails  []ytThumb `json:"thumbnails"`
	Entries     []ytInfo  `json:"entries"`
	IEKey       string    `json:"ie_key"`
	Extractor   string    `json:"extractor_key"`
	Availabilty string    `json:"availability"`
}

type ytThumb struct {
	URL string `json:"url"`
}

func (e Env) dumpJSON(ctx context.Context, url string, extra ...string) (*ytInfo, error) {
	args := append(e.commonArgs(), "-J")
	args = append(args, extra...)
	args = append(args, "--", url)
	out, err := proc.Output(ctx, e.YtDlp, args...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, friendlyYtError(err.Error())
	}
	var info ytInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		return nil, fmt.Errorf("jawaban yt-dlp tidak bisa dibaca: %w", err)
	}
	return &info, nil
}

// AnalyzeYouTube reads a video, playlist or channel (and other yt-dlp supported pages).
func AnalyzeYouTube(ctx context.Context, env Env, link Link) (*Collection, error) {
	if env.YtDlp == "" {
		return nil, errors.New("yt-dlp belum terpasang. Buka Pengaturan untuk mengunduhnya.")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	archived := readArchive(env.ArchivePath)
	col := &Collection{Source: link.Source, Type: link.Type, URL: link.URL, TabCounts: map[string]int{}}

	switch link.Type {
	case TypeChannel:
		tabs := []string{"videos", "shorts", "streams"}
		results := make([]*ytInfo, len(tabs))
		errs := make([]error, len(tabs))
		var wg sync.WaitGroup
		for i, tab := range tabs {
			wg.Add(1)
			go func(i int, tab string) {
				defer wg.Done()
				results[i], errs[i] = env.dumpJSON(ctx, link.URL+"/"+tab, "--flat-playlist", "--extractor-args", "youtubetab:approximate_date")
			}(i, tab)
		}
		wg.Wait()
		ok := false
		for i, tab := range tabs {
			r := results[i]
			if errs[i] != nil || r == nil {
				continue
			}
			ok = true
			if col.Title == "" {
				col.Title = firstNonEmpty(r.Channel, r.Uploader, trimTabSuffix(r.Title))
			}
			n := 0
			for _, en := range r.Entries {
				if entry, good := flatEntry(en, tab); good {
					n++
					entry.Index = n
					entry.Archived = archived["youtube "+entry.ID]
					col.Entries = append(col.Entries, entry)
				}
			}
			col.TabCounts[tab] = n
		}
		if !ok {
			for _, err := range errs {
				if err != nil {
					return nil, err
				}
			}
			return nil, errors.New("channel tidak bisa dibaca")
		}
		col.Subtitle = link.ID
	case TypePlaylist:
		r, err := env.dumpJSON(ctx, link.URL, "--flat-playlist")
		if err != nil {
			return nil, err
		}
		col.Title = firstNonEmpty(r.Title, "Playlist")
		col.Subtitle = firstNonEmpty(r.Channel, r.Uploader)
		n := 0
		for _, en := range r.Entries {
			n++
			entry, good := flatEntry(en, "")
			if !good {
				entry.Unavailable = true
			}
			entry.Index = n
			entry.Archived = archived["youtube "+entry.ID]
			col.Entries = append(col.Entries, entry)
		}
	default: // single video (YouTube or another site)
		r, err := env.dumpJSON(ctx, link.URL, "--no-playlist")
		if err != nil {
			return nil, err
		}
		if len(r.Entries) > 0 { // some sites return a playlist anyway
			r = &r.Entries[0]
		}
		entry := Entry{ID: r.ID, URL: firstNonEmpty(r.WebpageURL, link.URL), Title: r.Title, Index: 1, Date: r.UploadDate}
		if r.Duration != nil {
			entry.Duration = *r.Duration
		}
		entry.Thumbnail = r.Thumbnail
		if link.Source == SourceYouTube {
			entry.Thumbnail = "https://i.ytimg.com/vi/" + r.ID + "/mqdefault.jpg"
			entry.Archived = archived["youtube "+r.ID]
		}
		col.Title = r.Title
		col.Subtitle = firstNonEmpty(r.Channel, r.Uploader)
		col.Entries = []Entry{entry}
		col.Type = TypeVideo
	}
	if len(col.Entries) > 0 {
		col.Thumbnail = col.Entries[0].Thumbnail
	}
	return col, nil
}

func flatEntry(en ytInfo, tab string) (Entry, bool) {
	e := Entry{ID: en.ID, Title: en.Title, Tab: tab, Date: en.UploadDate}
	if en.Duration != nil {
		e.Duration = *en.Duration
	}
	if e.Date == "" {
		ts := en.Timestamp
		if ts == nil {
			ts = en.ReleaseTS
		}
		if ts != nil && *ts > 0 {
			e.Date = time.Unix(int64(*ts), 0).UTC().Format("20060102")
		}
	}
	if reYTID.MatchString(en.ID) {
		e.URL = "https://www.youtube.com/watch?v=" + en.ID
		e.Thumbnail = "https://i.ytimg.com/vi/" + en.ID + "/mqdefault.jpg"
	} else {
		e.URL = en.URL
	}
	low := strings.ToLower(en.Title)
	if e.ID == "" || low == "[private video]" || low == "[deleted video]" || strings.Contains(en.Availabilty, "private") {
		if e.Title == "" {
			e.Title = "Video tidak tersedia"
		}
		return e, false
	}
	return e, true
}

func trimTabSuffix(s string) string {
	for _, suf := range []string{" - Videos", " - Shorts", " - Live", " - Streams"} {
		s = strings.TrimSuffix(s, suf)
	}
	return s
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// readArchive loads yt-dlp's download archive ("youtube <id>" per line).
func readArchive(path string) map[string]bool {
	out := map[string]bool{}
	if path == "" {
		return out
	}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			out[line] = true
		}
	}
	return out
}

// Archived reports whether a YouTube id is recorded in the archive.
func Archived(path, id string) bool { return readArchive(path)["youtube "+id] }

// ytDownload runs yt-dlp for one URL and returns the final file path.
type ytJob struct {
	URL      string
	Dir      string
	Template string // yt-dlp output template relative to Dir, without extension
	Opts     Options
	Archive  bool
	Phases   int
	NoEmbed  bool
	PathFile string // yt-dlp appends the final file path here
}

func escapeTemplate(s string) string { return strings.ReplaceAll(s, "%", "%%") }

func (e Env) ytArgs(job ytJob) []string {
	o := job.Opts
	args := e.commonArgs()
	args = append(args,
		"--newline", "--progress", "--no-playlist", "--no-mtime", "--no-overwrites", "--continue",
		"--progress-template", "download:[KMB] %(progress.downloaded_bytes)s %(progress.total_bytes)s %(progress.total_bytes_estimate)s %(progress.speed)s %(progress.eta)s",
		"--print-to-file", "after_move:%(filepath)s", escapeTemplate(job.PathFile),
		"-P", job.Dir,
		"-o", job.Template+".%(ext)s",
	)
	if o.Mode == "audio" {
		q := "0"
		if o.AudioQuality == "192" || o.AudioQuality == "320" {
			q = o.AudioQuality + "K"
		}
		args = append(args, "-f", "ba/b", "-x", "--audio-format", o.AudioFormat, "--audio-quality", q)
	} else {
		sort := "res"
		if o.Quality != "best" {
			sort = "res:" + o.Quality
		}
		if o.Container == "mp4" {
			args = append(args, "-S", sort+",vcodec:h264,acodec:m4a", "--merge-output-format", "mp4", "--remux-video", "mp4")
		} else {
			args = append(args, "-S", sort, "--merge-output-format", "mkv")
		}
	}
	if o.Embed && !job.NoEmbed {
		args = append(args, "--embed-metadata", "--embed-thumbnail", "--convert-thumbnails", "jpg")
	}
	if job.Archive && e.ArchivePath != "" {
		args = append(args, "--download-archive", e.ArchivePath)
	}
	return append(args, "--", job.URL)
}

// runYtDlp executes a job, reporting progress, and returns the output path.
func (e Env) runYtDlp(ctx context.Context, job ytJob, r queue.Reporter, progressScale float64) (string, error) {
	if e.YtDlp == "" {
		return "", queue.Fail("yt-dlp belum terpasang", "")
	}
	pf, err := os.CreateTemp(e.TempDir, "path-*.txt")
	if err != nil {
		return "", err
	}
	job.PathFile = pf.Name()
	pf.Close()
	defer os.Remove(job.PathFile)
	cmd := proc.Command(ctx, e.YtDlp, e.ytArgs(job)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("yt-dlp tidak bisa dijalankan: %w", err)
	}
	tail := proc.NewTail(60)
	var (
		mu      sync.Mutex
		output  string
		skipped string
		phase   int
		lastPct float64
		phases  = max(1, job.Phases)
	)
	handle := func(line string) {
		tail.Add(line)
		switch {
		case strings.HasPrefix(line, "[KMB] "):
			f := strings.Fields(strings.TrimPrefix(line, "[KMB] "))
			if len(f) < 5 {
				return
			}
			done := num(f[0])
			total := num(f[1])
			if total <= 0 {
				total = num(f[2])
			}
			if total <= 0 {
				r.Progress(-1)
				return
			}
			pct := done / total
			mu.Lock()
			if pct+0.2 < lastPct && phase < phases-1 {
				phase++ // a new stream (audio after video) started
			}
			lastPct = pct
			overall := (float64(phase) + pct) / float64(phases)
			mu.Unlock()
			r.Progress(overall * progressScale)
			if pct >= 0.999 {
				r.Message("Memproses…")
				return
			}
			msg := "Mengunduh"
			if sp := num(f[3]); sp > 0 {
				msg += " · " + humanBytes(sp) + "/s"
			}
			if eta := num(f[4]); eta > 0 {
				msg += " · sisa " + humanDuration(eta)
			}
			r.Message(msg)
		case strings.Contains(line, "has already been recorded in the archive"):
			mu.Lock()
			skipped = "Sudah pernah diunduh"
			mu.Unlock()
		case strings.Contains(line, "has already been downloaded"):
			mu.Lock()
			skipped = "File sudah ada"
			mu.Unlock()
		case strings.HasPrefix(line, "[Merger]"):
			r.Message("Menggabungkan video & audio…")
		case strings.HasPrefix(line, "[ExtractAudio]"):
			r.Message("Mengubah ke " + strings.ToUpper(job.Opts.AudioFormat) + "…")
		case strings.HasPrefix(line, "[VideoRemuxer]"), strings.HasPrefix(line, "[VideoConvertor]"):
			r.Message("Menyesuaikan format…")
		case strings.HasPrefix(line, "[EmbedThumbnail]"), strings.HasPrefix(line, "[Metadata]"):
			r.Message("Menyematkan info & thumbnail…")
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); proc.ScanLines(stdout, handle) }()
	go func() { defer wg.Done(); proc.ScanLines(stderr, handle) }()
	wg.Wait()
	err = cmd.Wait()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	mu.Lock()
	defer mu.Unlock()
	if err != nil {
		return "", friendlyYtError(tail.String())
	}
	if data, rerr := os.ReadFile(job.PathFile); rerr == nil {
		lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(data), "\r", "")), "\n")
		output = strings.TrimSpace(lines[len(lines)-1])
	}
	if skipped != "" && output == "" {
		return "", queue.Skip(skipped)
	}
	if output == "" {
		return "", queue.Fail("yt-dlp selesai tanpa file hasil", tail.String())
	}
	if skipped == "File sudah ada" {
		return output, queue.Skip(skipped)
	}
	return output, nil
}

// DownloadYouTube downloads one entry into dir. prefix is a literal file-name prefix.
func DownloadYouTube(ctx context.Context, env Env, entry Entry, dir, prefix string, datePrefix bool, o Options, r queue.Reporter) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", queue.Fail("Tidak bisa membuat folder tujuan", err.Error())
	}
	tpl := escapeTemplate(prefix)
	if datePrefix {
		tpl += "%(upload_date>%Y-%m-%d)s - "
	}
	tpl += "%(title).150B"
	phases := 1
	if o.Mode == "video" {
		phases = 2
	}
	r.Message("Menyiapkan…")
	r.Progress(-1)
	out, err := env.runYtDlp(ctx, ytJob{URL: entry.URL, Dir: dir, Template: tpl, Opts: o, Archive: o.SkipExisting, Phases: phases}, r, 1)
	if out != "" {
		if st, statErr := os.Stat(out); statErr == nil {
			r.SetOutput(out, st.Size())
		}
	}
	return out, err
}

func num(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func humanBytes(b float64) string {
	units := []string{"B", "KB", "MB", "GB"}
	i := 0
	for b >= 1024 && i < len(units)-1 {
		b /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", b, units[i])
	}
	return strings.Replace(fmt.Sprintf("%.1f %s", b, units[i]), ".", ",", 1)
}

func humanDuration(sec float64) string {
	s := int(sec)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s%3600/60, s%60)
	}
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

// friendlyYtError maps yt-dlp output to a short Indonesian message.
func friendlyYtError(out string) error {
	detail := strings.TrimSpace(out)
	low := strings.ToLower(detail)
	var msg string
	switch {
	case strings.Contains(low, "not a bot"):
		msg = "YouTube meminta verifikasi \"bukan bot\". Coba lagi beberapa saat lagi."
	case strings.Contains(low, "confirm your age") || strings.Contains(low, "age-restricted") || strings.Contains(low, "inappropriate for some users"):
		msg = "Video dibatasi umur (perlu login YouTube)"
	case strings.Contains(low, "private video"):
		msg = "Video privat"
	case strings.Contains(low, "members-only") || strings.Contains(low, "join this channel"):
		msg = "Khusus member channel"
	case strings.Contains(low, "http error 429") || strings.Contains(low, "too many requests"):
		msg = "Terlalu banyak permintaan ke YouTube. Coba lagi nanti."
	case strings.Contains(low, "requested format is not available"):
		msg = "Kualitas/format yang dipilih tidak tersedia untuk video ini"
	case strings.Contains(low, "javascript runtime") || strings.Contains(low, "challenge solving failed") || strings.Contains(low, "n challenge"):
		msg = "yt-dlp butuh JS runtime yang berfungsi. Cek Pengaturan › Tools."
	case strings.Contains(low, "ffmpeg not found") || strings.Contains(low, "ffprobe and ffmpeg not found") || strings.Contains(low, "ffmpeg is not installed"):
		msg = "FFmpeg belum terpasang. Buka Pengaturan untuk mengunduhnya."
	case strings.Contains(low, "does not have a") && strings.Contains(low, "tab"):
		msg = "Channel ini tidak punya konten jenis tersebut"
	case strings.Contains(low, "video unavailable") || strings.Contains(low, "this video is not available") || strings.Contains(low, "has been removed"):
		msg = "Video tidak tersedia"
	case strings.Contains(low, "unsupported url"):
		msg = "Link ini tidak didukung"
	case strings.Contains(low, "unable to download") || strings.Contains(low, "getaddrinfo") || strings.Contains(low, "timed out") || strings.Contains(low, "connection"):
		msg = "Koneksi bermasalah. Periksa internet lalu coba lagi."
	case strings.Contains(low, "no space left"):
		msg = "Ruang disk penuh"
	}
	if msg == "" {
		msg = "Gagal: " + lastErrorLine(detail)
	}
	return queue.Fail(msg, detail)
}

func lastErrorLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, "ERROR:") {
			l = strings.TrimSpace(strings.TrimPrefix(l, "ERROR:"))
			if len([]rune(l)) > 140 {
				l = string([]rune(l)[:140]) + "…"
			}
			return l
		}
	}
	l := proc.LastLines(s, 1)
	if len([]rune(l)) > 140 {
		l = string([]rune(l)[:140]) + "…"
	}
	if l == "" {
		l = "kesalahan tidak diketahui"
	}
	return l
}
