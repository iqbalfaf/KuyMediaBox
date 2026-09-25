package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Reading a link through spotDL can take a minute or more (Spotify's shared API key is
// rate-limited). The public embed page carries the same list — titles, artists, durations,
// cover — and answers in about a second, also for Spotify-made playlists that spotDL can't
// read. Full tags (album, track number, genre) still come from spotDL while downloading.

type embedImage struct {
	URL   string `json:"url"`
	Width int    `json:"maxWidth"`
}

type embedEntity struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Duration    int64  `json:"duration"` // ms
	ReleaseDate struct {
		ISO string `json:"isoString"`
	} `json:"releaseDate"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Authors []struct {
		Name string `json:"name"`
	} `json:"authors"`
	TrackList []struct {
		URI      string `json:"uri"`
		Title    string `json:"title"`
		Subtitle string `json:"subtitle"`
		Duration int64  `json:"duration"`
	} `json:"trackList"`
	CoverArt struct {
		Sources []struct {
			URL   string `json:"url"`
			Width int    `json:"width"`
		} `json:"sources"`
	} `json:"coverArt"`
	VisualIdentity struct {
		Image []embedImage `json:"image"`
	} `json:"visualIdentity"`
}

// embedLimit is the most tracks the embed page lists; longer playlists are read with spotDL.
const embedLimit = 100

var reNextData = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

var embedClient = &http.Client{Timeout: 30 * time.Second}

// spotifyEmbed reads the embed page of a track, album or playlist.
func spotifyEmbed(ctx context.Context, kind, id string) (*embedEntity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://open.spotify.com/embed/"+kind+"/"+id, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept-Language", "en")
	resp, err := embedClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify embed: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return parseEmbed(body)
}

func parseEmbed(page []byte) (*embedEntity, error) {
	m := reNextData.FindSubmatch(page)
	if m == nil {
		return nil, errors.New("spotify embed: no data")
	}
	var data struct {
		Props struct {
			PageProps struct {
				State struct {
					Data struct {
						Entity *embedEntity `json:"entity"`
					} `json:"data"`
				} `json:"state"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &data); err != nil {
		return nil, err
	}
	e := data.Props.PageProps.State.Data.Entity
	if e == nil || (e.Name == "" && e.Title == "") {
		return nil, errors.New("spotify embed: empty")
	}
	return e, nil
}

func (e *embedEntity) cover() string {
	best, width := "", -1
	for _, im := range e.VisualIdentity.Image {
		if im.Width > width {
			best, width = im.URL, im.Width
		}
	}
	for _, s := range e.CoverArt.Sources {
		if s.Width > width {
			best, width = s.URL, s.Width
		}
	}
	return best
}

func (e *embedEntity) displayName() string { return firstNonEmpty(e.Name, e.Title) }

func trackIDFromURI(uri string) string {
	if i := strings.LastIndex(uri, ":"); i >= 0 && strings.HasPrefix(uri, "spotify:track:") {
		return uri[i+1:]
	}
	return ""
}

// embedSongs turns an embed page into songs (partial metadata) for the link list.
func embedSongs(link Link, e *embedEntity) []spotifySong {
	year := ""
	if len(e.ReleaseDate.ISO) >= 4 {
		year = e.ReleaseDate.ISO[:4]
	}
	date := ""
	if len(e.ReleaseDate.ISO) >= 10 {
		date = e.ReleaseDate.ISO[:10]
	}
	cover := e.cover()
	if link.Type == TypeTrack {
		var artists []string
		for _, a := range e.Artists {
			artists = append(artists, a.Name)
		}
		if len(artists) == 0 && e.Subtitle != "" {
			artists = []string{e.Subtitle}
		}
		s := spotifySong{Name: e.displayName(), Artists: artists, Artist: firstOf(artists), Duration: float64(e.Duration) / 1000,
			SongID: e.ID, URL: "https://open.spotify.com/track/" + firstNonEmpty(e.ID, link.ID), CoverURL: cover, Date: date}
		if year != "" {
			s.Year = year
		}
		return []spotifySong{s}
	}
	owner := e.Subtitle
	if owner == "" && len(e.Authors) > 0 {
		owner = e.Authors[0].Name
	}
	var songs []spotifySong
	for i, t := range e.TrackList {
		id := trackIDFromURI(t.URI)
		if id == "" { // podcast episodes and local files can't be matched
			continue
		}
		s := spotifySong{Name: t.Title, Artists: splitArtists(t.Subtitle), Artist: t.Subtitle, Duration: float64(t.Duration) / 1000,
			SongID: id, URL: "https://open.spotify.com/track/" + id}
		if link.Type == TypeAlbum {
			s.AlbumName, s.AlbumArtist, s.CoverURL = e.displayName(), owner, cover
			s.TrackNumber, s.TracksCount, s.DiscNumber = i+1, len(e.TrackList), 1
			s.Date = date
			if year != "" {
				s.Year = year
			}
		} else {
			s.ListName, s.ListPosition = e.displayName(), i+1
		}
		songs = append(songs, s)
	}
	return songs
}

// splitArtists splits the embed's "A, B" artist line.
func splitArtists(line string) []string {
	var out []string
	for _, a := range strings.Split(line, ",") {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}
