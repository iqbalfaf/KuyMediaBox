package downloader

import (
	"net/url"
	"strings"
)

// detectSocial recognises TikTok, Instagram and Facebook links. ok is false for other hosts.
func detectSocial(host string, u *url.URL, raw string) (Link, bool) {
	p := u.Path
	switch host {
	case "tiktok.com":
		if m := reTikTokPost.FindStringSubmatch(p); m != nil {
			user := m[1]
			if user == "@" {
				user = "@_"
			}
			return Link{Source: SourceTikTok, Type: TypePost, ID: m[3], Photo: m[2] == "photo",
				URL: "https://www.tiktok.com/" + user + "/" + m[2] + "/" + m[3]}, true
		}
		if strings.HasPrefix(p, "/t/") {
			return Link{Source: SourceTikTok, Type: TypePost, Short: true, URL: raw}, true
		}
		if m := reTikTokUser.FindStringSubmatch(p); m != nil {
			return Link{Source: SourceTikTok, Type: TypeProfile, ID: m[1], URL: "https://www.tiktok.com/" + m[1]}, true
		}
		return Link{Source: SourceTikTok, Type: TypeUnknown, URL: raw}, true
	case "vm.tiktok.com", "vt.tiktok.com":
		return Link{Source: SourceTikTok, Type: TypePost, Short: true, URL: raw}, true
	case "tiktokv.com":
		if m := reTikTokShare.FindStringSubmatch(p); m != nil {
			return Link{Source: SourceTikTok, Type: TypePost, ID: m[1], URL: "https://www.tiktok.com/@_/video/" + m[1]}, true
		}
		return Link{Source: SourceTikTok, Type: TypeUnknown, URL: raw}, true
	case "instagram.com", "instagr.am":
		if m := reIGPost.FindStringSubmatch(p); m != nil {
			kind := m[1]
			if kind == "reels" {
				kind = "reel"
			}
			return Link{Source: SourceInstagram, Type: TypePost, ID: m[2], Photo: kind == "p",
				URL: "https://www.instagram.com/" + kind + "/" + m[2] + "/"}, true
		}
		if strings.HasPrefix(p, "/share/") {
			return Link{Source: SourceInstagram, Type: TypePost, Short: true, Photo: true, URL: raw}, true
		}
		return Link{Source: SourceInstagram, Type: TypeUnknown, URL: raw}, true
	case "facebook.com", "web.facebook.com", "mbasic.facebook.com", "fb.com":
		switch {
		case strings.HasPrefix(p, "/share/"):
			// Share links redirect to the real post; photo shares (/share/p/) may hold pictures.
			return Link{Source: SourceFacebook, Type: TypePost, Short: true, Photo: strings.HasPrefix(p, "/share/p/"), URL: raw}, true
		case reFBVideo.MatchString(p):
			return Link{Source: SourceFacebook, Type: TypePost, ID: fbID(u), URL: fbURL(u)}, true
		case reFBPhoto.MatchString(p):
			return Link{Source: SourceFacebook, Type: TypePost, ID: fbID(u), Photo: true, URL: fbURL(u)}, true
		}
		return Link{Source: SourceFacebook, Type: TypeUnknown, URL: raw}, true
	case "fb.watch":
		return Link{Source: SourceFacebook, Type: TypePost, Short: true, URL: raw}, true
	}
	return Link{}, false
}

// fbURL keeps only the query parameters Facebook needs to find the post.
func fbURL(u *url.URL) string {
	q := u.Query()
	keep := url.Values{}
	for _, k := range []string{"v", "fbid", "set", "story_fbid", "id"} {
		if v := q.Get(k); v != "" {
			keep.Set(k, v)
		}
	}
	out := "https://www.facebook.com" + u.Path
	if len(keep) > 0 {
		out += "?" + keep.Encode()
	}
	return out
}

func fbID(u *url.URL) string {
	q := u.Query()
	for _, k := range []string{"v", "fbid", "story_fbid"} {
		if v := q.Get(k); v != "" {
			return v
		}
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	return parts[len(parts)-1]
}

// hasKnownDomain lets SplitLinks accept links pasted without "https://".
func hasKnownDomain(s string) bool {
	low := strings.ToLower(s)
	for _, p := range []string{"x.com/", "www.x.com/", "mobile.x.com/"} {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	for _, d := range []string{"youtu", "spotify", "tiktok", "instagram.com", "instagr.am", "facebook.com", "fb.watch", "fb.com", "pinterest.", "pin.it", "twitter.com", "fxtwitter.com", "vxtwitter.com", "fixupx.com", "fixvx.com"} {
		if strings.Contains(low, d) {
			return true
		}
	}
	return false
}
