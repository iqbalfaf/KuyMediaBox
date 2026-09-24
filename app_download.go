package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/downloader"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/naming"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/tools"
)

func (a *App) env() downloader.Env {
	return downloader.Env{
		YtDlp:       a.tools.Path(tools.YtDlp),
		FFmpeg:      a.tools.Path(tools.FFmpeg),
		SpotDL:      a.tools.Path(tools.SpotDL),
		JSKind:      a.tools.JSRuntimeKind(),
		JSPath:      a.tools.Path(tools.JSRuntime),
		ArchivePath: filepath.Join(appdir.DataDir(), "download-archive.txt"),
		TempDir:     appdir.TempDir(),
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
	case downloader.TypePlaylist, downloader.TypeChannel, downloader.TypeAlbum:
		name := strings.ReplaceAll(naming.SanitizeFileName(col.Title), "%", "")
		return filepath.Join(base, name)
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
	env := a.env()
	if env.YtDlp == "" {
		return nil, errors.New(i18n.L("yt-dlp belum terpasang. Buka Pengaturan untuk mengunduhnya.", "yt-dlp is not installed. Open Settings to download it."))
	}
	if env.FFmpeg == "" {
		return nil, errors.New(i18n.L("FFmpeg belum terpasang. Buka Pengaturan untuk mengunduhnya.", "FFmpeg is not installed. Open Settings to download it."))
	}
	if col.Source == downloader.SourceSpotify && env.SpotDL == "" {
		return nil, errors.New(i18n.L("spotDL belum terpasang. Buka Pengaturan untuk mengunduhnya.", "spotDL is not installed. Open Settings to download it."))
	}
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

	specs := make([]queue.Spec, len(chosen))
	out := make([]JobRef, len(chosen))
	for i, e := range chosen {
		e := e
		title := e.Title
		if e.Artist != "" {
			title = e.Artist + " - " + e.Title
		}
		var run queue.RunFunc
		if col.Source == downloader.SourceSpotify {
			name := downloader.SpotifyFileName(e, o.Numbering && col.Type != downloader.TypeTrack, width)
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
		specs[i] = queue.Spec{Title: title, Run: run}
		out[i].ItemID = e.ID
	}
	taskIDs := a.queue.AddMany(queue.KindDownload, specs)
	for i := range out {
		out[i].TaskID = taskIDs[i]
	}
	return out, nil
}
