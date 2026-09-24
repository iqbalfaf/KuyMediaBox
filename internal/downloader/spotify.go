package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/i18n"
	"kuymediabox/internal/naming"
	"kuymediabox/internal/proc"
	"kuymediabox/internal/queue"
)

// spotifySong mirrors the fields of spotDL's .spotdl save file that we use.
type spotifySong struct {
	Name         string   `json:"name"`
	Artists      []string `json:"artists"`
	Artist       string   `json:"artist"`
	AlbumName    string   `json:"album_name"`
	AlbumArtist  string   `json:"album_artist"`
	Duration     float64  `json:"duration"`
	Year         any      `json:"year"`
	Date         string   `json:"date"`
	TrackNumber  int      `json:"track_number"`
	TracksCount  int      `json:"tracks_count"`
	DiscNumber   int      `json:"disc_number"`
	SongID       string   `json:"song_id"`
	URL          string   `json:"url"`
	CoverURL     string   `json:"cover_url"`
	DownloadURL  string   `json:"download_url"`
	ListName     string   `json:"list_name"`
	ListPosition int      `json:"list_position"`
	Genres       []string `json:"genres"`
}

func (s spotifySong) artistLine() string {
	if len(s.Artists) > 0 {
		return strings.Join(s.Artists, ", ")
	}
	return s.Artist
}

func (e Env) spotdlSave(ctx context.Context, urls []string, preload bool) ([]spotifySong, error) {
	if e.SpotDL == "" {
		return nil, errors.New(i18n.L("spotDL belum terpasang. Buka Pengaturan untuk mengunduhnya.", "spotDL is not installed. Open Settings to download it."))
	}
	f, err := os.CreateTemp(e.TempDir, "list-*.spotdl")
	if err != nil {
		return nil, err
	}
	saveFile := f.Name()
	f.Close()
	os.Remove(saveFile)
	defer os.Remove(saveFile)

	args := []string{"save"}
	args = append(args, urls...)
	args = append(args, "--save-file", saveFile, "--log-level", "ERROR")
	if preload {
		args = append(args, "--preload")
	}
	if e.FFmpeg != "" {
		args = append(args, "--ffmpeg", e.FFmpeg)
	}
	out, runErr := proc.Output(ctx, e.SpotDL, args...)
	data, readErr := os.ReadFile(saveFile)
	if readErr != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		detail := out
		if runErr != nil {
			detail = runErr.Error()
		}
		return nil, friendlySpotifyError(detail)
	}
	var songs []spotifySong
	if err := json.Unmarshal(data, &songs); err != nil {
		return nil, fmt.Errorf(i18n.L("daftar lagu Spotify tidak bisa dibaca: %w", "Spotify song list can't be read: %w"), err)
	}
	return songs, nil
}

func friendlySpotifyError(detail string) error {
	low := strings.ToLower(detail)
	msg := ""
	switch {
	case strings.Contains(low, "rate") && strings.Contains(low, "limit"), strings.Contains(low, "429"):
		msg = i18n.L("Spotify sedang membatasi permintaan. Coba lagi beberapa menit lagi.", "Spotify is rate-limiting requests. Try again in a few minutes.")
	case strings.Contains(low, "404") || strings.Contains(low, "not found") || strings.Contains(low, "non existing id"):
		msg = i18n.L("Playlist/album tidak ditemukan. Playlist buatan Spotify atau private tidak bisa dibaca.", "Playlist/album not found. Spotify-made or private playlists can't be read.")
	case strings.Contains(low, "connection") || strings.Contains(low, "timed out") || strings.Contains(low, "getaddrinfo"):
		msg = i18n.L("Koneksi bermasalah. Periksa internet lalu coba lagi.", "Connection problem. Check your internet and try again.")
	}
	if msg == "" {
		last := proc.LastLines(detail, 1)
		if last == "" {
			last = i18n.L("tidak ada lagu yang terbaca", "no songs could be read")
		}
		if len([]rune(last)) > 140 {
			last = string([]rune(last)[:140]) + "…"
		}
		msg = i18n.L("Gagal membaca Spotify: ", "Couldn't read Spotify: ") + last
	}
	return queue.Fail(msg, detail)
}

// AnalyzeSpotify reads a Spotify track, album or playlist through spotDL.
func AnalyzeSpotify(ctx context.Context, env Env, link Link) (*Collection, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	songs, err := env.spotdlSave(ctx, []string{link.URL}, false)
	if err != nil {
		return nil, err
	}
	if len(songs) == 0 {
		return nil, errors.New(i18n.L("tidak ada lagu di link ini (playlist kosong, private, atau buatan Spotify)", "no songs in this link (empty, private or Spotify-made playlist)"))
	}
	col := &Collection{Source: SourceSpotify, Type: link.Type, URL: link.URL, TabCounts: map[string]int{}}
	switch link.Type {
	case TypeAlbum:
		col.Title = firstNonEmpty(songs[0].AlbumName, songs[0].ListName, "Album")
		col.Subtitle = firstNonEmpty(songs[0].AlbumArtist, songs[0].artistLine())
	case TypePlaylist:
		col.Title = firstNonEmpty(songs[0].ListName, "Playlist Spotify")
	default:
		col.Title = songs[0].Name
		col.Subtitle = songs[0].artistLine()
	}
	col.Thumbnail = songs[0].CoverURL
	sortSongs(songs, link.Type == TypeAlbum)
	for i := range songs {
		s := songs[i]
		idx := i + 1
		col.Entries = append(col.Entries, Entry{
			ID:        firstNonEmpty(s.SongID, s.URL, strconv.Itoa(i)),
			URL:       s.URL,
			Title:     s.Name,
			Artist:    s.artistLine(),
			Album:     s.AlbumName,
			Duration:  s.Duration,
			Index:     idx,
			Thumbnail: s.CoverURL,
			song:      &songs[i],
		})
	}
	return col, nil
}

// sortSongs orders an album by disc/track number and a playlist by its list position.
// spotDL does not guarantee the order of its save file.
func sortSongs(songs []spotifySong, album bool) {
	sort.SliceStable(songs, func(i, j int) bool {
		a, b := songs[i], songs[j]
		if album {
			if a.DiscNumber != b.DiscNumber {
				return a.DiscNumber < b.DiscNumber
			}
			return a.TrackNumber < b.TrackNumber
		}
		if a.ListPosition > 0 && b.ListPosition > 0 {
			return a.ListPosition < b.ListPosition
		}
		return false
	})
}

// Matcher resolves Spotify tracks to YouTube URLs once per download batch.
type Matcher struct {
	env  Env
	urls []string
	once sync.Once
	done chan struct{}
	res  map[string]string
	err  error
}

// NewMatcher prepares a matcher for the given Spotify track URLs.
func NewMatcher(env Env, urls []string) *Matcher {
	return &Matcher{env: env, urls: urls, done: make(chan struct{})}
}

// Resolve returns the YouTube URL for a Spotify track URL ("" if spotDL found none).
func (m *Matcher) Resolve(ctx context.Context, spotifyURL string) (string, error) {
	m.once.Do(func() {
		go func() {
			defer close(m.done)
			// Independent of any single task so cancelling one track does not break the rest.
			rctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			defer cancel()
			songs, err := m.env.spotdlSave(rctx, m.urls, true)
			m.res = map[string]string{}
			m.err = err
			for _, s := range songs {
				if s.DownloadURL != "" {
					m.res[s.URL] = s.DownloadURL
				}
			}
		}()
	})
	select {
	case <-m.done:
		return m.res[spotifyURL], m.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// DownloadSpotify downloads one Spotify entry: match on YouTube, fetch audio, tag it.
func DownloadSpotify(ctx context.Context, env Env, entry Entry, dir, fileName string, o Options, matcher *Matcher, r queue.Reporter) (string, error) {
	if entry.song == nil {
		return "", queue.Fail(i18n.L("Data lagu tidak lengkap, periksa link lagi", "Song data incomplete, check the link again"), "")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", queue.Fail(i18n.L("Tidak bisa membuat folder tujuan", "Can't create the destination folder"), err.Error())
	}
	final := filepath.Join(dir, fileName+"."+o.AudioFormat)
	if _, err := os.Stat(final); err == nil {
		r.SetOutput(final, 0)
		return final, queue.Skip(i18n.L("File sudah ada", "File already exists"))
	}

	r.Progress(-1)
	r.Message(i18n.L("Mencocokkan lagu di YouTube…", "Matching the song on YouTube…"))
	src, err := matcher.Resolve(ctx, entry.song.URL)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if src == "" {
		// spotDL could not match: fall back to a YouTube Music style search.
		src = "ytsearch1:" + entry.song.artistLine() + " - " + entry.song.Name + " audio"
		_ = err
	}

	work, err := os.MkdirTemp(env.TempDir, "sp-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)

	job := ytJob{URL: src, Dir: work, Template: "audio", Opts: o, Phases: 1, NoEmbed: true}
	job.Opts.Mode = "audio"
	audio, err := env.runYtDlp(ctx, job, r, 0.9)
	if err != nil {
		var ue *queue.UserError
		if errors.As(err, &ue) && strings.Contains(src, "ytsearch1:") {
			ue.Message = i18n.L("Lagu tidak ditemukan di YouTube", "Song not found on YouTube")
		}
		return "", err
	}

	r.Message(i18n.L("Menyematkan judul, artis & cover…", "Embedding title, artist & cover…"))
	cover := ""
	if entry.song.CoverURL != "" {
		if p, cerr := fetchCover(ctx, entry.song.CoverURL, work); cerr == nil {
			cover = p
		}
	}
	tmpOut := filepath.Join(work, "tagged."+o.AudioFormat)
	if err := tagAudio(ctx, env.FFmpeg, audio, cover, tmpOut, *entry.song, o.AudioFormat); err != nil {
		return "", err
	}
	if err := naming.Commit(tmpOut, final); err != nil {
		return "", queue.Fail(i18n.L("Tidak bisa menyimpan file", "Can't save the file"), err.Error())
	}
	if st, err := os.Stat(final); err == nil {
		r.SetOutput(final, st.Size())
	}
	return final, nil
}

func fetchCover(ctx context.Context, url, dir string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cover: %s", resp.Status)
	}
	p := filepath.Join(dir, "cover.jpg")
	f, err := os.Create(p)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 20<<20)); err != nil {
		f.Close()
		return "", err
	}
	return p, f.Close()
}

func tagAudio(ctx context.Context, ffmpegPath, in, cover, out string, s spotifySong, format string) error {
	if ffmpegPath == "" {
		return queue.Fail(i18n.L("FFmpeg belum terpasang", "FFmpeg is not installed"), "")
	}
	args := []string{"-i", in}
	withCover := cover != "" && format != "opus" && format != "wav" // WAV has no picture tag
	if withCover {
		args = append(args, "-i", cover, "-map", "0:a:0", "-map", "1:v:0", "-c:v", "mjpeg", "-disposition:v:0", "attached_pic")
	} else {
		args = append(args, "-map", "0:a:0")
	}
	args = append(args, "-c:a", "copy", "-map_metadata", "-1")
	meta := map[string]string{
		"title":        s.Name,
		"artist":       s.artistLine(),
		"album":        s.AlbumName,
		"album_artist": firstNonEmpty(s.AlbumArtist, s.Artist),
	}
	if s.TrackNumber > 0 {
		tr := strconv.Itoa(s.TrackNumber)
		if s.TracksCount > 0 {
			tr += "/" + strconv.Itoa(s.TracksCount)
		}
		meta["track"] = tr
	}
	if s.DiscNumber > 0 {
		meta["disc"] = strconv.Itoa(s.DiscNumber)
	}
	if y := fmt.Sprint(s.Year); y != "" && y != "<nil>" && y != "0" {
		meta["date"] = strings.TrimSuffix(y, ".0")
	} else if len(s.Date) >= 4 {
		meta["date"] = s.Date[:4]
	}
	if len(s.Genres) > 0 {
		meta["genre"] = s.Genres[0]
	}
	for k, v := range meta {
		if strings.TrimSpace(v) != "" {
			args = append(args, "-metadata", k+"="+v)
		}
	}
	if format == "mp3" {
		args = append(args, "-id3v2_version", "3")
	}
	args = append(args, out)
	return ffmpeg.Run(ctx, ffmpegPath, args, 0, nil)
}

// SpotifyFileName builds "NN - Artist - Title" (number optional).
func SpotifyFileName(e Entry, numbering bool, width int) string {
	name := e.Title
	if e.Artist != "" {
		name = e.Artist + " - " + e.Title
	}
	if numbering && e.Index > 0 {
		name = fmt.Sprintf("%0*d - %s", width, e.Index, name)
	}
	return naming.SanitizeFileName(name)
}
