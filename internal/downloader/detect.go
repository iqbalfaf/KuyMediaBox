// Package downloader reads YouTube/Spotify links and downloads their items with yt-dlp
// (Spotify tracks are matched to YouTube through spotDL's metadata).
package downloader

import (
	"net/url"
	"regexp"
	"strings"
)

// Link sources and types.
const (
	SourceYouTube = "youtube"
	SourceSpotify = "spotify"
	SourceOther   = "other"

	TypeVideo    = "video"
	TypePlaylist = "playlist"
	TypeChannel  = "channel"
	TypeTrack    = "track"
	TypeAlbum    = "album"
	TypeUnknown  = "unknown"
)

// Link is the result of recognising a pasted URL.
type Link struct {
	Source string `json:"source"`
	Type   string `json:"type"`
	URL    string `json:"url"` // normalised URL to analyse
	ID     string `json:"id"`
}

var (
	reYTID        = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	reSpotifyPath = regexp.MustCompile(`^/(?:intl-[a-z]{2}(?:-[a-z]{2})?/)?(track|album|playlist|artist)/([A-Za-z0-9]+)`)
	reChannelPath = regexp.MustCompile(`^/(@[^/]+|channel/[^/]+|c/[^/]+|user/[^/]+)`)
)

// Detect recognises a URL. Unknown http(s) links are passed to yt-dlp as "other".
func Detect(raw string) Link {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Link{Type: TypeUnknown}
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return Link{Type: TypeUnknown, URL: raw}
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	host = strings.TrimPrefix(host, "m.")

	switch host {
	case "youtu.be":
		id := strings.Trim(u.Path, "/")
		if reYTID.MatchString(id) {
			return Link{Source: SourceYouTube, Type: TypeVideo, ID: id, URL: "https://www.youtube.com/watch?v=" + id}
		}
	case "youtube.com", "music.youtube.com", "youtube-nocookie.com":
		q := u.Query()
		p := u.Path
		switch {
		case p == "/watch" && reYTID.MatchString(q.Get("v")):
			id := q.Get("v")
			return Link{Source: SourceYouTube, Type: TypeVideo, ID: id, URL: "https://www.youtube.com/watch?v=" + id}
		case strings.HasPrefix(p, "/shorts/") || strings.HasPrefix(p, "/live/") || strings.HasPrefix(p, "/embed/"):
			id := strings.Split(strings.Trim(p, "/"), "/")
			if len(id) >= 2 && reYTID.MatchString(id[1]) {
				return Link{Source: SourceYouTube, Type: TypeVideo, ID: id[1], URL: "https://www.youtube.com/watch?v=" + id[1]}
			}
		case p == "/playlist" && q.Get("list") != "":
			list := q.Get("list")
			return Link{Source: SourceYouTube, Type: TypePlaylist, ID: list, URL: "https://www.youtube.com/playlist?list=" + list}
		default:
			if m := reChannelPath.FindStringSubmatch(p); m != nil {
				return Link{Source: SourceYouTube, Type: TypeChannel, ID: m[1], URL: "https://www.youtube.com/" + m[1]}
			}
		}
		return Link{Source: SourceYouTube, Type: TypeUnknown, URL: raw}
	case "open.spotify.com", "play.spotify.com":
		if m := reSpotifyPath.FindStringSubmatch(u.Path); m != nil {
			kind := m[1]
			norm := "https://open.spotify.com/" + kind + "/" + m[2]
			switch kind {
			case "track":
				return Link{Source: SourceSpotify, Type: TypeTrack, ID: m[2], URL: norm}
			case "album":
				return Link{Source: SourceSpotify, Type: TypeAlbum, ID: m[2], URL: norm}
			case "playlist":
				return Link{Source: SourceSpotify, Type: TypePlaylist, ID: m[2], URL: norm}
			}
			return Link{Source: SourceSpotify, Type: TypeUnknown, URL: norm}
		}
		return Link{Source: SourceSpotify, Type: TypeUnknown, URL: raw}
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return Link{Source: SourceOther, Type: TypeVideo, URL: u.String()}
	}
	return Link{Type: TypeUnknown, URL: raw}
}

// SplitLinks splits pasted text into individual URLs (one per line, or separated by spaces).
func SplitLinks(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range strings.Fields(text) {
		f = strings.Trim(f, `"'<>(),`)
		if f == "" || seen[f] {
			continue
		}
		if strings.Contains(f, ".") && (strings.Contains(f, "://") || strings.Contains(f, "youtu") || strings.Contains(f, "spotify")) {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}
