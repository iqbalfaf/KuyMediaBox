package downloader

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

// More sites with their own tab: SoundCloud (music), Twitch (VODs & clips), Reddit (video,
// pictures, galleries) and Bilibili. yt-dlp reads all of them; Reddit pictures come from
// gallery-dl.
const (
	SourceSoundCloud = "soundcloud"
	SourceTwitch     = "twitch"
	SourceReddit     = "reddit"
	SourceBilibili   = "bilibili"
)

var (
	reBiliVideo   = regexp.MustCompile(`^/(?:[a-z]{2}/)?video/((?:BV|bv)[0-9A-Za-z]{10}|av\d+)`)
	reTwitchVOD   = regexp.MustCompile(`^/(?:[^/]+/)?(?:videos|v)/(\d+)`)
	reTwitchClip  = regexp.MustCompile(`^/[^/]+/clip/([A-Za-z0-9_-]+)`)
	reRedditPost  = regexp.MustCompile(`^/(?:r|u|user)/[^/]+/comments/([a-z0-9]+)`)
	reRedditShare = regexp.MustCompile(`^/r/[^/]+/s/[A-Za-z0-9]+`)
)

// twitchReserved are Twitch paths that aren't channels.
var twitchReserved = map[string]bool{"directory": true, "videos": true, "settings": true, "search": true, "downloads": true, "p": true, "jobs": true, "subscriptions": true, "inventory": true, "wallet": true, "drops": true}

// soundcloudReserved are SoundCloud paths that aren't users.
var soundcloudReserved = map[string]bool{"discover": true, "search": true, "stream": true, "upload": true, "you": true, "charts": true, "pages": true, "terms-of-use": true, "settings": true, "messages": true, "notifications": true}

// detectMore recognises the sites of this file.
func detectMore(host string, u *url.URL) (Link, bool) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	first := ""
	if len(parts) > 0 {
		first = parts[0]
	}
	switch host {
	case "soundcloud.com":
		if first == "" || soundcloudReserved[first] {
			return Link{Source: SourceSoundCloud, Type: TypeUnknown, URL: u.String()}, true
		}
		base := "https://soundcloud.com/" + first
		switch {
		case len(parts) >= 3 && parts[1] == "sets":
			return Link{Source: SourceSoundCloud, Type: TypePlaylist, ID: parts[2], URL: base + "/sets/" + parts[2]}, true
		case len(parts) == 1 || (len(parts) == 2 && (parts[1] == "tracks" || parts[1] == "popular-tracks")):
			return Link{Source: SourceSoundCloud, Type: TypePlaylist, ID: first, URL: base + "/tracks"}, true
		case len(parts) == 2 && (parts[1] == "sets" || parts[1] == "albums" || parts[1] == "likes" || parts[1] == "reposts"):
			return Link{Source: SourceSoundCloud, Type: TypeUnknown, URL: u.String()}, true
		case len(parts) >= 2:
			return Link{Source: SourceSoundCloud, Type: TypeVideo, ID: parts[1], URL: base + "/" + parts[1]}, true
		}
	case "on.soundcloud.com":
		return Link{Source: SourceSoundCloud, Type: TypeVideo, URL: u.String(), Short: true}, true
	case "twitch.tv", "clips.twitch.tv":
		if host == "clips.twitch.tv" && first != "" {
			return Link{Source: SourceTwitch, Type: TypeVideo, ID: first, URL: "https://clips.twitch.tv/" + first}, true
		}
		if m := reTwitchVOD.FindStringSubmatch(u.Path); m != nil {
			return Link{Source: SourceTwitch, Type: TypeVideo, ID: m[1], URL: "https://www.twitch.tv/videos/" + m[1]}, true
		}
		if m := reTwitchClip.FindStringSubmatch(u.Path); m != nil {
			return Link{Source: SourceTwitch, Type: TypeVideo, ID: m[1], URL: "https://clips.twitch.tv/" + m[1]}, true
		}
		if first != "" && !twitchReserved[first] {
			// A channel: its past broadcasts (clips with /clips).
			if len(parts) >= 2 && parts[1] == "clips" {
				return Link{Source: SourceTwitch, Type: TypePlaylist, ID: first, URL: "https://www.twitch.tv/" + first + "/clips?filter=clips&range=all"}, true
			}
			return Link{Source: SourceTwitch, Type: TypePlaylist, ID: first, URL: "https://www.twitch.tv/" + first + "/videos?filter=archives&sort=time"}, true
		}
		return Link{Source: SourceTwitch, Type: TypeUnknown, URL: u.String()}, true
	case "reddit.com", "old.reddit.com", "new.reddit.com", "np.reddit.com":
		if m := reRedditPost.FindStringSubmatch(u.Path); m != nil {
			return Link{Source: SourceReddit, Type: TypePost, ID: m[1], URL: "https://www.reddit.com" + strings.TrimSuffix(u.Path, "/") + "/", Photo: true}, true
		}
		if len(parts) == 2 && parts[0] == "gallery" && parts[1] != "" {
			return Link{Source: SourceReddit, Type: TypePost, ID: parts[1], URL: "https://www.reddit.com/gallery/" + parts[1], Photo: true}, true
		}
		if reRedditShare.MatchString(u.Path) {
			return Link{Source: SourceReddit, Type: TypePost, URL: "https://www.reddit.com" + u.Path, Photo: true, Short: true}, true
		}
		return Link{Source: SourceReddit, Type: TypeUnknown, URL: u.String()}, true
	case "redd.it":
		if first != "" {
			return Link{Source: SourceReddit, Type: TypePost, ID: first, URL: "https://www.reddit.com/comments/" + first + "/", Photo: true}, true
		}
	case "v.redd.it":
		if first != "" {
			return Link{Source: SourceReddit, Type: TypePost, ID: first, URL: "https://v.redd.it/" + first, Short: true}, true
		}
	case "bilibili.com", "bilibili.tv":
		if m := reBiliVideo.FindStringSubmatch(u.Path); m != nil {
			v := url.URL{Scheme: "https", Host: "www." + host, Path: "/video/" + m[1]}
			if p := u.Query().Get("p"); p != "" {
				v.RawQuery = "p=" + url.QueryEscape(p)
			}
			return Link{Source: SourceBilibili, Type: TypeVideo, ID: m[1], URL: v.String()}, true
		}
		return Link{Source: SourceBilibili, Type: TypeUnknown, URL: u.String()}, true
	case "space.bilibili.com":
		if first != "" {
			return Link{Source: SourceBilibili, Type: TypePlaylist, ID: first, URL: "https://space.bilibili.com/" + first + "/video"}, true
		}
	case "b23.tv":
		return Link{Source: SourceBilibili, Type: TypeVideo, URL: u.String(), Short: true}, true
	}
	return Link{}, false
}

// ResolveRedirect follows a short link (b23.tv, on.soundcloud.com, …) to its page.
func ResolveRedirect(ctx context.Context, url string) (string, error) {
	return resolveRedirect(ctx, url)
}
