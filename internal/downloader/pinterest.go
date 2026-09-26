package downloader

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
	"kuymediabox/internal/queue"
)

// Pinterest links are read with gallery-dl (pins, boards, profiles, search results). Pictures
// are fetched straight from the CDN; videos are downloaded by yt-dlp from the pin page, so
// they can also be saved as audio.

var (
	rePinHost = regexp.MustCompile(`(^|\.)pinterest\.[a-z]{2,3}(\.[a-z]{2})?$`)
	rePinID   = regexp.MustCompile(`^/pin/(?:[^/]*--)?([0-9]+|[A-Za-z0-9_-]{8,})`)
)

// First path segments that are Pinterest pages, not user names.
var pinReserved = map[string]bool{
	"pin": true, "search": true, "ideas": true, "today": true, "explore": true, "business": true, "settings": true,
	"resource": true, "login": true, "signup": true, "categories": true, "topics": true, "homefeed": true,
	"news_hub": true, "_": true, "url_shortener": true, "discover": true, "shopping": true,
}

// How many pins a list may hold at most (search results never end).
const (
	pinLimitBoard   = 1000
	pinLimitProfile = 500
	pinLimitSearch  = 200
)

// detectPinterest recognises pinterest.com (and country domains) and pin.it links.
func detectPinterest(host string, u *url.URL, raw string) (Link, bool) {
	if host == "pin.it" {
		return Link{Source: SourcePinterest, Type: TypePost, Short: true, Photo: true, URL: raw}, true
	}
	if !rePinHost.MatchString(host) {
		return Link{}, false
	}
	p := u.Path
	if m := rePinID.FindStringSubmatch(p); m != nil {
		return Link{Source: SourcePinterest, Type: TypePost, ID: m[1], Photo: true, URL: "https://www.pinterest.com/pin/" + m[1] + "/"}, true
	}
	if strings.HasPrefix(p, "/search/") {
		q := strings.TrimSpace(u.Query().Get("q"))
		if q == "" {
			return Link{Source: SourcePinterest, Type: TypeUnknown, URL: raw}, true
		}
		return Link{Source: SourcePinterest, Type: TypeSearch, ID: q, URL: "https://www.pinterest.com/search/pins/?q=" + url.QueryEscape(q)}, true
	}
	var parts []string
	for _, s := range strings.Split(strings.Trim(p, "/"), "/") {
		if s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 || pinReserved[strings.ToLower(parts[0])] {
		return Link{Source: SourcePinterest, Type: TypeUnknown, URL: raw}, true
	}
	user := parts[0]
	switch {
	case len(parts) == 1:
		// A profile page lists boards; "all pins" is what people mean by "everything".
		return Link{Source: SourcePinterest, Type: TypeProfile, ID: user, URL: "https://www.pinterest.com/" + user + "/pins/"}, true
	case len(parts) == 2 && (parts[1] == "pins" || parts[1] == "_created" || parts[1] == "_saved"):
		tab := parts[1]
		if tab == "_saved" {
			tab = "pins"
		}
		return Link{Source: SourcePinterest, Type: TypeProfile, ID: user, URL: "https://www.pinterest.com/" + user + "/" + tab + "/"}, true
	default:
		// /user/board/ or /user/board/section/
		return Link{Source: SourcePinterest, Type: TypeBoard, ID: user + "/" + parts[1],
			URL: "https://www.pinterest.com/" + strings.Join(parts, "/") + "/"}, true
	}
}

// pinterestCollection lists the pins of a Pinterest link.
func pinterestCollection(ctx context.Context, env Env, link Link) (*Collection, error) {
	if env.GalleryDL == "" {
		return nil, queue.Fail(i18n.L("Link Pinterest dibaca dengan gallery-dl. Pasang gallery-dl di Pengaturan › Tools pendukung.", "Pinterest links are read with gallery-dl. Install gallery-dl in Settings › Helper tools."), "")
	}
	args := append([]string{"--config-ignore", "-j"}, env.cookieArgs()...)
	switch link.Type {
	case TypeBoard:
		args = append(args, "--range", "1-"+strconv.Itoa(pinLimitBoard))
	case TypeProfile:
		args = append(args, "--range", "1-"+strconv.Itoa(pinLimitProfile))
	case TypeSearch:
		args = append(args, "--range", "1-"+strconv.Itoa(pinLimitSearch))
	}
	out, err := proc.Output(ctx, env.GalleryDL, append(args, link.URL)...)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	files, perr := parseGalleryJSON(out)
	if perr != nil || (err != nil && len(files) == 0) {
		detail := strings.TrimSpace(out)
		if err != nil {
			detail = err.Error() + "\n" + out
		} else if detail != "" {
			detail = perr.Error() + "\n" + out
		}
		return nil, pinError(detail, env)
	}
	return pinCollection(link, files)
}

// pinError turns gallery-dl output about Pinterest into a short message.
func pinError(detail string, env Env) error {
	low := strings.ToLower(detail)
	switch {
	case strings.Contains(low, "authrequired") || strings.Contains(low, "authenticat") || strings.Contains(low, "login"):
		return needLogin("Pinterest", env, detail)
	case strings.Contains(low, "notfounderror") || strings.Contains(low, "could not be found") || strings.Contains(low, "404"):
		return queue.Fail(i18n.L("Pin, board, atau profil Pinterest tidak ditemukan (mungkin sudah dihapus atau privat)", "Pinterest pin, board or profile not found (it may be deleted or private)"), detail)
	case strings.TrimSpace(detail) == "":
		return queue.Fail(i18n.L("Tidak ada foto atau video yang bisa diunduh di link Pinterest ini", "This Pinterest link has no picture or video to download"), "")
	}
	return friendlySocialError(detail)
}

// pinCollection turns gallery-dl's list into entries: pictures from the CDN, videos by pin page.
func pinCollection(link Link, files []galleryFile) (*Collection, error) {
	col := &Collection{Source: SourcePinterest, Type: link.Type, URL: link.URL, postID: link.ID}
	// yt-dlp reads one video per pin page: a pin with several videos gets its streams directly.
	videos := map[string]int{}
	for _, f := range files {
		if pinIsVideo(f) {
			videos[metaString(f.Meta, "id")]++
		}
	}
	seenURL := map[string]bool{}
	seenID := map[string]bool{}
	firstPin := ""
	for _, f := range files {
		m := f.Meta
		id := metaString(m, "id")
		ext := strings.ToLower(metaString(m, "extension"))
		video := pinIsVideo(f)
		if (!video && !imageExts[ext]) || seenURL[f.URL] {
			continue
		}
		seenURL[f.URL] = true
		if firstPin == "" {
			firstPin = id
		}
		user := pinUser(m)
		caption := cleanCaption(SourcePinterest, firstNonEmpty(metaString(m, "title"), metaString(m, "grid_title")), firstNonEmpty(metaString(m, "description"), metaString(m, "auto_alt_text")), user)
		if col.uploader == "" {
			col.uploader = user
		}
		if col.Title == "" {
			col.Title = pinListTitle(link, m, caption)
		}
		// Several slides of one pin share its ID: entry IDs key the list in the UI.
		e := Entry{ID: uniqueID(seenID, firstNonEmpty(id, "pin")), Title: caption, Thumbnail: pinThumb(m, f.URL), uploader: user}
		if d := metaFloat(m, "duration"); d > 0 {
			e.Duration = d / 1000 // Pinterest gives milliseconds
		}
		if video {
			e.Kind = KindVideo
			e.URL = "https://www.pinterest.com/pin/" + id + "/"
			e.archiveKey = "pinterest " + id
			if videos[id] > 1 || id == "" {
				e.URL = strings.TrimPrefix(f.URL, "ytdl:")
				e.archiveKey = ""
			}
		} else {
			e.Kind = KindImage
			e.URL = f.URL
			e.ext = ext
		}
		if e.Title == "" {
			e.Title = "Pin " + id
		}
		col.Entries = append(col.Entries, e)
	}
	if len(col.Entries) == 0 {
		return nil, queue.Fail(i18n.L("Tidak ada foto atau video yang bisa diunduh di link Pinterest ini", "This Pinterest link has no picture or video to download"), "")
	}
	if link.Type == TypePost {
		if col.postID == "" {
			col.postID = firstPin
		}
		if len(col.Entries) > 1 {
			for i := range col.Entries {
				if col.Entries[i].Kind == KindImage {
					col.Entries[i].Title = i18n.F("Foto %d", "Photo %d", i+1)
				} else {
					col.Entries[i].Title = i18n.F("Video %d", "Video %d", i+1)
				}
			}
		}
	}
	if col.Title == "" {
		col.Title = firstNonEmpty(col.postID, "Pinterest")
	}
	col.Subtitle = col.uploader
	switch link.Type {
	case TypeSearch:
		col.Subtitle = i18n.L("Hasil pencarian", "Search results")
		if len(col.Entries) >= pinLimitSearch {
			col.Subtitle += i18n.F(" (%d pin pertama)", " (first %d pins)", pinLimitSearch)
		}
	case TypeProfile:
		col.Title = "@" + strings.TrimPrefix(link.ID, "@")
	}
	return col, nil
}

// pinIsVideo tells a video (an HLS stream yt-dlp downloads, or an MP4) from a picture.
func pinIsVideo(f galleryFile) bool {
	ext := strings.ToLower(metaString(f.Meta, "extension"))
	return strings.HasPrefix(f.URL, "ytdl:") || ext == "mp4" || ext == "m3u8" || ext == "mov"
}

func pinListTitle(link Link, m map[string]any, caption string) string {
	switch link.Type {
	case TypeBoard:
		if b, ok := m["board"].(map[string]any); ok {
			if name := metaString(b, "name"); name != "" {
				return name
			}
		}
		return link.ID
	case TypeSearch:
		return link.ID
	case TypeProfile:
		return "@" + link.ID
	}
	return caption
}

func pinUser(m map[string]any) string {
	for _, k := range []string{"pinner", "native_creator", "origin_pinner"} {
		if u, ok := m[k].(map[string]any); ok {
			if s := firstNonEmpty(metaString(u, "username"), metaString(u, "full_name")); s != "" {
				return s
			}
		}
	}
	return ""
}

// pinThumb picks a small preview of a pin (the 236/474 px image), else the file itself.
func pinThumb(m map[string]any, fileURL string) string {
	if imgs, ok := m["images"].(map[string]any); ok {
		for _, size := range []string{"236x", "474x", "orig"} {
			if im, ok := imgs[size].(map[string]any); ok {
				if u := metaString(im, "url"); u != "" {
					return u
				}
			}
		}
	}
	if strings.HasPrefix(fileURL, "ytdl:") {
		return ""
	}
	return fileURL
}

func metaFloat(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	}
	return 0
}
