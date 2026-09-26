package downloader

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
	"kuymediabox/internal/queue"
)

// X (Twitter) posts and profiles are read with gallery-dl: it lists every photo and video of a
// post. Photos are fetched straight from the CDN; videos are downloaded by yt-dlp from the post,
// so they can also be saved as audio. Posts are public; profiles need a login (cookies).

var (
	reXStatus  = regexp.MustCompile(`^/(?:i/web|i|([A-Za-z0-9_]{1,20}))/status(?:es)?/(\d+)`)
	reXProfile = regexp.MustCompile(`^/([A-Za-z0-9_]{1,20})(?:/media)?/?$`)
	reXMediaID = regexp.MustCompile(`/(?:ext_tw_video|amplify_video)/(\d+)/`)
	reXTco     = regexp.MustCompile(`\s*https?://t\.co/\S+`)
)

var xHosts = map[string]bool{
	"x.com": true, "twitter.com": true, "mobile.x.com": true, "mobile.twitter.com": true,
	"fxtwitter.com": true, "vxtwitter.com": true, "fixupx.com": true, "fixvx.com": true,
}

// First path segments that are X pages, not user names.
var xReserved = map[string]bool{
	"home": true, "explore": true, "search": true, "i": true, "notifications": true, "messages": true,
	"settings": true, "hashtag": true, "compose": true, "login": true, "logout": true, "signup": true,
	"tos": true, "privacy": true, "jobs": true, "account": true, "intent": true, "share": true,
}

// How many photos and videos a profile may list at most.
const xLimitProfile = 500

// detectX recognises x.com / twitter.com posts and profiles (and the fx/vx share mirrors).
func detectX(host string, u *url.URL) (Link, bool) {
	if !xHosts[host] {
		return Link{}, false
	}
	p := u.Path
	if m := reXStatus.FindStringSubmatch(p); m != nil {
		user := m[1]
		if user == "" {
			user = "i"
		}
		return Link{Source: SourceX, Type: TypePost, ID: m[2], Photo: true, URL: "https://x.com/" + user + "/status/" + m[2]}, true
	}
	if m := reXProfile.FindStringSubmatch(p); m != nil && !xReserved[strings.ToLower(m[1])] {
		return Link{Source: SourceX, Type: TypeProfile, ID: m[1], URL: "https://x.com/" + m[1] + "/media"}, true
	}
	return Link{Source: SourceX, Type: TypeUnknown, URL: u.String()}, true
}

// xCollection lists the photos and videos of an X post or profile.
func xCollection(ctx context.Context, env Env, link Link) (*Collection, error) {
	if env.GalleryDL == "" {
		if link.Type != TypePost {
			return nil, queue.Fail(i18n.L("Profil X dibaca dengan gallery-dl. Pasang gallery-dl di Pengaturan › Tools pendukung.", "X profiles are read with gallery-dl. Install gallery-dl in Settings › Helper tools."), "")
		}
		// Videos still work with yt-dlp alone.
		info, err := env.dumpJSON(ctx, link.URL, "--ignore-no-formats-error")
		if err != nil {
			return nil, xYtError(err, env)
		}
		return ytPostFrom(link, info)
	}
	args := append([]string{"--config-ignore", "-j"}, env.cookieArgs()...)
	var ytInfoOf *ytInfo
	var wg sync.WaitGroup
	if link.Type == TypeProfile {
		args = append(args, "--range", "1-"+strconv.Itoa(xLimitProfile))
	} else if env.YtDlp != "" {
		// gallery-dl has no video previews: yt-dlp reads the post at the same time. Its answer
		// is also the fallback when gallery-dl can't read the post.
		wg.Add(1)
		go func() {
			defer wg.Done()
			yctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if info, err := env.dumpJSON(yctx, link.URL, "--ignore-no-formats-error"); err == nil {
				ytInfoOf = info
			}
		}()
	}
	out, err := proc.Output(ctx, env.GalleryDL, append(args, link.URL)...)
	wg.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	files, perr := parseGalleryJSON(out)
	if perr != nil || len(files) == 0 {
		// A text post quoting a video, or a gallery-dl broken by an X change: yt-dlp may still
		// have the video.
		if ytInfoOf != nil && (len(ytInfoOf.Entries) > 0 || len(ytInfoOf.Formats) > 0) {
			return ytPostFrom(link, ytInfoOf)
		}
		if perr != nil || err != nil {
			detail := strings.TrimSpace(out)
			if err != nil {
				detail = err.Error() + "\n" + out
			} else if detail != "" {
				detail = perr.Error() + "\n" + out
			}
			return nil, xError(detail, env)
		}
	}
	return xPostsCollection(link, files, xVideos(ytInfoOf))
}

// xVideos lists the videos yt-dlp found in a post, in their order.
func xVideos(info *ytInfo) []ytInfo {
	switch {
	case info == nil:
		return nil
	case len(info.Entries) > 0:
		return info.Entries
	}
	return []ytInfo{*info}
}

// xYtError maps a yt-dlp error about an X post.
func xYtError(err error, env Env) error {
	detail := err.Error()
	var ue *queue.UserError
	if errors.As(err, &ue) && ue.Detail != "" {
		detail = ue.Detail
	}
	if strings.Contains(strings.ToLower(detail), "no video could be found") {
		return errGalleryMissing()
	}
	return xError(detail, env)
}

// xError turns gallery-dl/yt-dlp output about X into a short message.
func xError(detail string, env Env) error {
	low := strings.ToLower(detail)
	switch {
	case strings.Contains(low, "authrequired") || strings.Contains(low, "authenticat") || strings.Contains(low, "log in") || strings.Contains(low, "login"):
		return needLogin("X", env, detail)
	case strings.Contains(low, "protected") || strings.Contains(low, "suspended"):
		return queue.Fail(i18n.L("Akun ini dikunci atau ditangguhkan, isinya tidak bisa dibaca", "This account is protected or suspended, its posts can't be read"), detail)
	case strings.Contains(low, "'result'") || strings.Contains(low, "could not be found") || strings.Contains(low, "does not exist") || strings.Contains(low, "unavailable"):
		return queue.Fail(i18n.L("Post atau akun X tidak ditemukan (mungkin sudah dihapus)", "X post or account not found (it may have been deleted)"), detail)
	case strings.TrimSpace(detail) == "":
		return queue.Fail(i18n.L("Post ini tidak berisi foto atau video", "This post has no picture or video"), "")
	}
	return friendlySocialError(detail)
}

// xPostsCollection turns gallery-dl's list into entries: photos from the CDN, videos by post.
// vids is yt-dlp's list of the post's videos (previews and IDs), when it was read.
func xPostsCollection(link Link, files []galleryFile, vids []ytInfo) (*Collection, error) {
	col := &Collection{Source: SourceX, Type: link.Type, URL: link.URL, postID: link.ID}
	nth := map[string]int{}
	seenURL := map[string]bool{}
	seenID := map[string]bool{}
	for _, f := range files {
		m := f.Meta
		tweet := metaString(m, "tweet_id")
		ext := strings.ToLower(metaString(m, "extension"))
		video := xIsVideo(f)
		if (!video && !imageExts[ext]) || seenURL[f.URL] {
			continue
		}
		seenURL[f.URL] = true
		user := xUser(m)
		if col.uploader == "" {
			col.uploader = user
		}
		caption := cleanCaption(SourceX, reXTco.ReplaceAllString(metaString(m, "content"), ""), "", user)
		id := tweet
		if n := metaString(m, "count"); n != "" && n != "1" {
			id = tweet + "-" + metaString(m, "num")
		}
		e := Entry{ID: uniqueID(seenID, id), Title: caption, uploader: user}
		if video {
			e.Kind = KindVideo
			e.URL = "https://x.com/" + firstNonEmpty(user, "i") + "/status/" + tweet
			e.Duration = metaFloat(m, "duration")
			// yt-dlp sees only the videos of a post (and those of a quoted post after them):
			// always pick one, --no-playlist would still download them all.
			nth[tweet]++
			e.item = nth[tweet]
			mediaID := ""
			if mid := reXMediaID.FindStringSubmatch(f.URL); mid != nil {
				mediaID = mid[1]
			}
			if link.Type == TypePost {
				e.Thumbnail, mediaID = xVideoInfo(vids, mediaID, e.item)
			}
			if mediaID != "" {
				e.archiveKey = "twitter " + mediaID
			}
		} else {
			e.Kind = KindImage
			e.URL = f.URL
			e.ext = ext
			e.Thumbnail = strings.Replace(f.URL, "name=orig", "name=small", 1)
		}
		if e.Title == "" {
			e.Title = "Post " + tweet
		}
		col.Entries = append(col.Entries, e)
	}
	if len(col.Entries) == 0 {
		return nil, queue.Fail(i18n.L("Tidak ada foto atau video yang bisa diunduh di link X ini", "This X link has no picture or video to download"), "")
	}
	if link.Type == TypeProfile {
		col.Title = "@" + link.ID
		col.Subtitle = i18n.L("Media", "Media")
		if len(col.Entries) >= xLimitProfile {
			col.Subtitle += i18n.F(" (%d item terbaru)", " (latest %d items)", xLimitProfile)
		}
		return col, nil
	}
	col.Title = col.Entries[0].Title
	if col.postID == "" {
		col.postID = metaString(files[0].Meta, "tweet_id")
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
	col.Subtitle = col.uploader
	return col, nil
}

// xIsVideo tells videos and GIFs (saved as MP4) apart from photos.
func xIsVideo(f galleryFile) bool {
	t := metaString(f.Meta, "type")
	return t == "video" || t == "animated_gif" || strings.Contains(f.URL, "video.twimg.com")
}

// xUser is the post author's @name (the profile owner for retweets is "user").
func xUser(m map[string]any) string {
	for _, k := range []string{"author", "user"} {
		if u, ok := m[k].(map[string]any); ok {
			if s := metaString(u, "name"); s != "" {
				return s
			}
		}
	}
	return ""
}

// xVideoInfo finds the preview and media ID of the n-th video of a post in yt-dlp's list: by
// media ID when gallery-dl's link has one, else by position (GIFs).
func xVideoInfo(vids []ytInfo, mediaID string, n int) (thumb, id string) {
	for _, v := range vids {
		if mediaID != "" && v.ID == mediaID {
			return v.Thumbnail, mediaID
		}
	}
	if mediaID == "" && n >= 1 && n <= len(vids) {
		return vids[n-1].Thumbnail, vids[n-1].ID
	}
	return "", mediaID
}
