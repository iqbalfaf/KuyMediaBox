package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/downloader"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/naming"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/tools"
)

func (a *App) env() downloader.Env {
	s := a.cfg.Get()
	return downloader.Env{
		CookiesBrowser: s.CookiesBrowser,
		CookiesFile:    s.CookiesFile,
		SpotifyAuth:    s.SpotifyLogin,
		NameTemplate:   s.NameTemplate,
		SpotifyTpl:     s.SpotifyTemplate,
		RateLimitKB:    rateShare(s.DownloadLimitKB, s.Parallel["download"]),
		YtDlp:          a.tools.Path(tools.YtDlp),
		FFmpeg:         a.tools.Path(tools.FFmpeg),
		SpotDL:         a.tools.Path(tools.SpotDL),
		GalleryDL:      a.tools.Path(tools.GalleryDL),
		JSKind:         a.tools.JSRuntimeKind(),
		JSPath:         a.tools.Path(tools.JSRuntime),
		ArchivePath:    filepath.Join(appdir.DataDir(), "download-archive.txt"),
		TempDir:        appdir.TempDir(),
	}
}

// DetectLinks splits pasted text into recognised links.
func (a *App) DetectLinks(text string) []downloader.Link {
	var out []downloader.Link
	for _, raw := range downloader.SplitLinks(text) {
		out = append(out, downloader.Detect(raw))
	}
	if out == nil {
		out = []downloader.Link{}
	}
	return out
}

// AnalyzeLink reads what a link contains (video, playlist, channel, Spotify list).
func (a *App) AnalyzeLink(raw string) (*downloader.Collection, error) {
	link := downloader.Detect(raw)
	env := a.env()
	var col *downloader.Collection
	var err error
	switch {
	case downloader.IsSocial(link.Source):
		col, err = downloader.AnalyzeSocial(context.Background(), env, link)
	case link.Type == downloader.TypeUnknown && link.Source == downloader.SourceSpotify:
		return nil, errors.New(i18n.L("Link Spotify ini belum didukung. Gunakan link lagu, album, atau playlist.", "This Spotify link isn't supported yet. Use a track, album or playlist link."))
	case link.Type == downloader.TypeUnknown:
		return nil, errors.New(i18n.L("Link tidak dikenali. Tempel link video, playlist, channel, lagu, album, atau playlist.", "Link not recognized. Paste a video, playlist, channel, track or album link."))
	case link.Source == downloader.SourceSpotify:
		col, err = downloader.AnalyzeSpotify(context.Background(), env, link)
	default:
		col, err = downloader.AnalyzeYouTube(context.Background(), env, link)
	}
	if err != nil {
		return nil, err
	}
	if len(col.Entries) == 0 {
		return nil, errors.New(i18n.L("Tidak ada video/lagu yang bisa diunduh di link ini", "There are no videos/songs to download in this link"))
	}
	a.colMu.Lock()
	a.colSeq++
	col.Key = "c" + strconv.Itoa(a.colSeq)
	a.collections[col.Key] = col
	a.colMu.Unlock()
	return col, nil
}

// SetEntrySource replaces the automatic YouTube match of a Spotify song with a chosen link
// ("" goes back to the automatic match).
func (a *App) SetEntrySource(key, id, url string) (downloader.Entry, error) {
	url = strings.TrimSpace(url)
	if url != "" {
		l := downloader.Detect(url)
		if l.Type == downloader.TypeUnknown || l.Source == downloader.SourceSpotify || downloader.IsSocial(l.Source) {
			return downloader.Entry{}, errors.New(i18n.L("Tempel link video YouTube (atau YouTube Music)", "Paste a YouTube (or YouTube Music) video link"))
		}
		url = l.URL
	}
	a.colMu.Lock()
	defer a.colMu.Unlock()
	col := a.collections[key]
	if col == nil {
		return downloader.Entry{}, errors.New(i18n.L("Data link sudah kedaluwarsa, periksa link lagi", "Link data expired, check the link again"))
	}
	for i := range col.Entries {
		if col.Entries[i].ID == id {
			col.Entries[i].Source = url
			return col.Entries[i], nil
		}
	}
	return downloader.Entry{}, errors.New(i18n.L("Lagu tidak ditemukan", "Song not found"))
}

// ForgetCollection releases a link removed from the page.
func (a *App) ForgetCollection(key string) {
	a.colMu.Lock()
	delete(a.collections, key)
	a.colMu.Unlock()
}

// CollectionDir returns the folder a collection downloads into.
func (a *App) CollectionDir(key string) string {
	a.colMu.Lock()
	col := a.collections[key]
	a.colMu.Unlock()
	if col == nil {
		return a.downloadBase()
	}
	return collectionDir(a.downloadBase(), col, a.cfg.Get().DownloadSubfolders)
}

// downloadBase is the Download module's result folder (default or chosen by the user).
func (a *App) downloadBase() string {
	return a.outputSpec("download").Dir
}

func collectionDir(base string, col *downloader.Collection, subfolders bool) string {
	if !subfolders {
		return base
	}
	switch col.Type {
	case downloader.TypePlaylist, downloader.TypeChannel, downloader.TypeAlbum, downloader.TypeArtist:
		name := strings.ReplaceAll(naming.SanitizeFileName(col.Title), "%", "")
		return filepath.Join(base, name)
	case downloader.TypeProfile, downloader.TypeBoard, downloader.TypeSearch:
		return filepath.Join(base, strings.ReplaceAll(downloader.PostFolderName(col), "%", ""))
	case downloader.TypePost:
		if len(col.Entries) > 1 {
			return filepath.Join(base, strings.ReplaceAll(downloader.PostFolderName(col), "%", ""))
		}
	}
	return base
}

// StartDownloads queues the selected entries of a collection.
func (a *App) StartDownloads(key string, ids []string, o downloader.Options) ([]JobRef, error) {
	a.colMu.Lock()
	col := a.collections[key]
	a.colMu.Unlock()
	if col == nil {
		return nil, errors.New(i18n.L("Data link sudah kedaluwarsa, periksa link lagi", "Link data expired, check the link again"))
	}
	o.Normalize(col.Source)
	if s, e := o.SectionStart, o.SectionEnd; s != "" || e != "" {
		st, ok1 := downloader.ParseClock(s)
		en, ok2 := downloader.ParseClock(e)
		if !ok1 || !ok2 || (en > 0 && en <= st) {
			return nil, errors.New(i18n.L("Waktu potong tidak valid. Contoh: 1:30 sampai 2:45", "Invalid cut time. Example: 1:30 to 2:45"))
		}
	}
	env := a.env()
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var chosen []downloader.Entry
	for _, e := range col.Entries {
		if want[e.ID] && !e.Unavailable {
			chosen = append(chosen, e)
		}
	}
	if len(chosen) == 0 {
		return nil, errors.New(i18n.L("Belum ada item yang dipilih", "No items selected"))
	}
	onlyPictures := true
	for _, e := range chosen {
		if e.Kind != downloader.KindImage {
			onlyPictures = false
		}
	}
	if !onlyPictures {
		if env.YtDlp == "" {
			return nil, errors.New(i18n.L("yt-dlp belum terpasang. Buka Pengaturan untuk mengunduhnya.", "yt-dlp is not installed. Open Settings to download it."))
		}
		if env.FFmpeg == "" {
			return nil, errors.New(i18n.L("FFmpeg belum terpasang. Buka Pengaturan untuk mengunduhnya.", "FFmpeg is not installed. Open Settings to download it."))
		}
	}
	if col.Source == downloader.SourceSpotify && env.SpotDL == "" {
		return nil, errors.New(i18n.L("spotDL belum terpasang. Buka Pengaturan untuk mengunduhnya.", "spotDL is not installed. Open Settings to download it."))
	}

	dir := collectionDir(a.downloadBase(), col, a.cfg.Get().DownloadSubfolders)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf(i18n.L("Folder download tidak bisa dipakai (%s): %w", "Download folder can't be used (%s): %w"), dir, err)
	}
	width := len(strconv.Itoa(len(col.Entries)))
	if width < 2 {
		width = 2
	}

	var matcher *downloader.Matcher
	if col.Source == downloader.SourceSpotify {
		var urls []string
		for _, e := range chosen {
			urls = append(urls, e.URL)
		}
		matcher = downloader.NewMatcher(env, urls)
	}

	// Albums, playlists and artists can get an .m3u8 playlist, refreshed as songs finish.
	var list *m3uWriter
	switch col.Type {
	case downloader.TypePlaylist, downloader.TypeAlbum, downloader.TypeArtist:
		if o.Playlist {
			list = &m3uWriter{path: filepath.Join(dir, downloader.PlaylistName(col)), items: map[string]downloader.PlaylistItem{}}
		}
	}

	specs := make([]queue.Spec, len(chosen))
	out := make([]JobRef, len(chosen))
	for i, e := range chosen {
		e := e
		title := e.Title
		if e.Artist != "" {
			title = e.Artist + " - " + e.Title
		}
		var run queue.RunFunc
		if downloader.IsSocial(col.Source) {
			name := downloader.SocialFileName(col, e, width)
			title = name
			run = func(ctx context.Context, r queue.Reporter) error {
				_, err := downloader.DownloadSocial(ctx, env, e, dir, name, o, r)
				return err
			}
		} else if col.Source == downloader.SourceSpotify {
			name := downloader.SpotifyFileName(e, o.Numbering && col.Type != downloader.TypeTrack, width, env.SpotifyTpl)
			run = func(ctx context.Context, r queue.Reporter) error {
				_, err := downloader.DownloadSpotify(ctx, env, e, dir, name, o, matcher, r)
				return err
			}
		} else {
			prefix := ""
			if col.Type == downloader.TypePlaylist && o.Numbering {
				prefix = fmt.Sprintf("%0*d - ", width, e.Index)
			}
			datePrefix := col.Type == downloader.TypeChannel
			run = func(ctx context.Context, r queue.Reporter) error {
				_, err := downloader.DownloadYouTube(ctx, env, e, dir, prefix, datePrefix, o, r)
				return err
			}
		}
		if list != nil {
			run = list.wrap(e, title, run)
		}
		specs[i] = queue.Spec{Title: title, Input: e.URL, Run: run}
		out[i].ItemID = e.ID
	}
	taskIDs := a.queue.AddMany(queue.KindDownload, specs)
	for i := range out {
		out[i].TaskID = taskIDs[i]
	}
	return out, nil
}

// m3uWriter keeps a playlist file in step with finished downloads.
type m3uWriter struct {
	mu    sync.Mutex
	path  string
	items map[string]downloader.PlaylistItem
}

// wrap records the task's result file and rewrites the playlist.
func (m *m3uWriter) wrap(e downloader.Entry, title string, run queue.RunFunc) queue.RunFunc {
	return func(ctx context.Context, r queue.Reporter) error {
		rec := &outRecorder{Reporter: r}
		err := run(ctx, rec)
		if rec.path != "" && (err == nil || errors.Is(err, queue.ErrSkipped)) {
			if _, statErr := os.Stat(rec.path); statErr == nil {
				m.mu.Lock()
				m.items[e.ID] = downloader.PlaylistItem{Index: e.Index, Path: rec.path, Title: title, Duration: e.Duration}
				list := make([]downloader.PlaylistItem, 0, len(m.items))
				for _, it := range m.items {
					list = append(list, it)
				}
				_ = downloader.WriteM3U(m.path, list)
				m.mu.Unlock()
			}
		}
		return err
	}
}

// outRecorder remembers the output path a task reports.
type outRecorder struct {
	queue.Reporter
	path string
}

func (o *outRecorder) SetOutput(path string, size int64) {
	o.path = path
	o.Reporter.SetOutput(path, size)
}

// rateShare splits the total download speed limit over the downloads running at once.
func rateShare(totalKB, parallel int) int {
	if totalKB <= 0 {
		return 0
	}
	return max(32, totalKB/max(1, parallel))
}
