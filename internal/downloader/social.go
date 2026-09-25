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
	"regexp"
	"strconv"
	"strings"
	"time"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/naming"
	"kuymediabox/internal/proc"
	"kuymediabox/internal/queue"
)

// Entry kinds of social posts.
const (
	KindVideo = "video"
	KindAudio = "audio"
	KindImage = "image"
)

var imageExts = map[string]bool{"jpg": true, "jpeg": true, "png": true, "webp": true, "heic": true, "heif": true, "avif": true, "gif": true, "bmp": true}

// AnalyzeSocial reads a TikTok, Instagram or Facebook post (or a TikTok profile).
//
// Videos and sound come from yt-dlp. Instagram pictures come from yt-dlp too (a post without
// video formats is a picture, its best "thumbnail" is the full-size photo). TikTok photo
// posts and Facebook photos are read with gallery-dl.
func AnalyzeSocial(ctx context.Context, env Env, link Link) (*Collection, error) {
	if env.YtDlp == "" {
		return nil, errors.New(i18n.L("yt-dlp belum terpasang. Buka Pengaturan untuk mengunduhnya.", "yt-dlp is not installed. Open Settings to download it."))
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if link.Short {
		if final, err := resolveRedirect(ctx, link.URL); err == nil {
			if l := Detect(final); l.Source == link.Source && l.Type != TypeUnknown {
				l.Photo = l.Photo || (link.Photo && link.Source != SourceInstagram)
				link = l
			}
		}
	}
	if link.Type == TypeUnknown {
		return nil, unsupportedSocial(link.Source)
	}
	archived := readArchive(env.ArchivePath)
	var col *Collection
	var err error
	switch {
	case link.Type == TypeProfile:
		col, err = tiktokProfile(ctx, env, link)
	case link.Source == SourceTikTok && link.Photo:
		col, err = tiktokPhotos(ctx, env, link)
	case link.Source == SourceFacebook && link.Photo:
		col, err = galleryPost(ctx, env, link)
		if err != nil { // a "photo" share can still be a video post
			if c, yerr := ytPost(ctx, env, link); yerr == nil {
				col, err = c, nil
			}
		}
	default:
		col, err = ytPost(ctx, env, link)
		// A TikTok /video/ link of a photo post only has the sound: read the pictures too.
		if err == nil && link.Source == SourceTikTok && len(col.Entries) == 1 && col.Entries[0].Kind == KindAudio {
			if c, gerr := tiktokPhotos(ctx, env, link); gerr == nil {
				col = c
			}
		}
	}
	if err != nil {
		return nil, err
	}
	for i := range col.Entries {
		e := &col.Entries[i]
		e.Index = i + 1
		e.Archived = e.archiveKey != "" && archived[e.archiveKey]
	}
	if col.Thumbnail == "" && len(col.Entries) > 0 {
		col.Thumbnail = col.Entries[0].Thumbnail
	}
	return col, nil
}

// ytPost reads a single post (or an Instagram carousel) with yt-dlp.
func ytPost(ctx context.Context, env Env, link Link) (*Collection, error) {
	info, err := env.dumpJSON(ctx, link.URL, "--ignore-no-formats-error")
	if err != nil {
		return nil, socialError(err)
	}
	items := []ytInfo{*info}
	if len(info.Entries) > 0 {
		items = info.Entries
	}
	uploader := firstNonEmpty(info.Uploader, info.Channel, info.UploaderID, firstUploader(items))
	postID := firstNonEmpty(link.ID, info.ID)
	col := &Collection{Source: link.Source, Type: TypePost, URL: link.URL, postID: postID, uploader: uploader}
	col.Title = cleanCaption(link.Source, info.Title, info.Description, uploader)
	if col.Title == "" && len(items) > 0 {
		col.Title = cleanCaption(link.Source, items[0].Title, items[0].Description, uploader)
	}
	if col.Title == "" {
		col.Title = postID
	}
	col.Subtitle = uploader
	multi := len(info.Entries) > 0
	for i, it := range items {
		e := Entry{ID: firstNonEmpty(it.ID, postID+"-"+strconv.Itoa(i+1)), Title: col.Title, Thumbnail: it.Thumbnail, uploader: uploader}
		if it.Duration != nil {
			e.Duration = *it.Duration
		}
		switch mediaKind(it) {
		case KindImage:
			if it.Thumbnail == "" {
				continue
			}
			e.Kind = KindImage
			e.URL = it.Thumbnail
			e.Title = i18n.F("Foto %d", "Photo %d", i+1)
		case KindAudio:
			e.Kind = KindAudio
			e.URL = firstNonEmpty(it.WebpageURL, link.URL)
			e.Title = i18n.L("Musik", "Sound")
			e.archiveKey = archiveKey(it, *info)
		default:
			e.Kind = KindVideo
			e.URL = firstNonEmpty(it.WebpageURL, link.URL)
			if multi {
				e.Title = i18n.F("Video %d", "Video %d", i+1)
			}
			e.archiveKey = archiveKey(it, *info)
		}
		if multi {
			e.URL = link.URL
			if e.Kind == KindImage {
				e.URL = it.Thumbnail
			}
			e.item = firstPositive(it.PlaylistIdx, i+1)
		}
		col.Entries = append(col.Entries, e)
	}
	if len(col.Entries) == 0 {
		return nil, queue.Fail(i18n.L("Tidak ada video atau foto yang bisa diunduh di post ini", "This post has no video or picture to download"), "")
	}
	if len(col.Entries) == 1 && col.Entries[0].Kind != KindAudio {
		col.Entries[0].Title = col.Title
	}
	return col, nil
}

// tiktokPhotos reads a TikTok photo post: pictures from gallery-dl, the sound from yt-dlp.
func tiktokPhotos(ctx context.Context, env Env, link Link) (*Collection, error) {
	id := link.ID
	user := "@_"
	if m := reTikTokPost.FindStringSubmatch(strings.TrimPrefix(link.URL, "https://www.tiktok.com")); m != nil {
		user, id = m[1], m[3]
	}
	photoURL := "https://www.tiktok.com/" + user + "/photo/" + id
	col, err := galleryPost(ctx, env, Link{Source: SourceTikTok, Type: TypePost, URL: photoURL, ID: id})
	if err != nil {
		return nil, err
	}
	if col.hasSound {
		col.Entries = append(col.Entries, Entry{
			ID: id + "-sound", Kind: KindAudio, Title: i18n.L("Musik slide", "Slideshow sound"),
			URL: "https://www.tiktok.com/" + user + "/video/" + id, Thumbnail: col.Thumbnail,
			archiveKey: "tiktok " + id, uploader: col.uploader,
		})
	}
	return col, nil
}

// galleryFile is one "Url" message of gallery-dl --dump-json.
type galleryFile struct {
	URL  string
	Meta map[string]any
}

// galleryPost lists the pictures of a post with gallery-dl.
func galleryPost(ctx context.Context, env Env, link Link) (*Collection, error) {
	if env.GalleryDL == "" {
		return nil, errGalleryMissing()
	}
	args := append([]string{"--config-ignore", "-j"}, env.cookieArgs()...)
	out, err := proc.Output(ctx, env.GalleryDL, append(args, link.URL)...)
	files, perr := parseGalleryJSON(out)
	if perr != nil {
		detail := out
		if err != nil {
			detail = err.Error() + "\n" + out
		}
		return nil, friendlySocialError(detail)
	}
	if err != nil && len(files) == 0 {
		return nil, friendlySocialError(err.Error() + "\n" + out)
	}
	return galleryCollection(link, files, out)
}

// galleryCollection turns gallery-dl's file list into a collection of pictures.
func galleryCollection(link Link, files []galleryFile, out string) (*Collection, error) {
	col := &Collection{Source: link.Source, Type: TypePost, URL: link.URL, postID: link.ID}
	n := 0
	for _, f := range files {
		ext := strings.ToLower(metaString(f.Meta, "extension"))
		if !imageExts[ext] {
			if ext == "mp3" || ext == "m4a" {
				col.hasSound = true
			}
			continue
		}
		n++
		if col.uploader == "" {
			col.uploader = galleryUploader(f.Meta)
		}
		if col.Title == "" {
			col.Title = cleanCaption(link.Source, metaString(f.Meta, "title"), firstNonEmpty(metaString(f.Meta, "desc"), metaString(f.Meta, "description"), metaString(f.Meta, "caption")), col.uploader)
		}
		if col.postID == "" {
			col.postID = metaString(f.Meta, "id")
		}
		col.Entries = append(col.Entries, Entry{
			ID: fmt.Sprintf("%s-%d", firstNonEmpty(col.postID, "img"), n), Kind: KindImage, URL: f.URL, Thumbnail: f.URL,
			Title: i18n.F("Foto %d", "Photo %d", n), ext: ext,
		})
	}
	if len(col.Entries) == 0 {
		return nil, queue.Fail(i18n.L("Tidak ada foto yang bisa diunduh di post ini", "This post has no picture to download"), out)
	}
	col.Subtitle = col.uploader
	if col.Title == "" {
		col.Title = firstNonEmpty(col.postID, i18n.L("Post", "Post"))
	}
	for i := range col.Entries {
		col.Entries[i].uploader = col.uploader
		if len(col.Entries) == 1 {
			col.Entries[i].Title = col.Title
		}
	}
	return col, nil
}

// parseGalleryJSON reads gallery-dl's --dump-json output: [[2, meta], [3, url, meta], [-1, error]].
func parseGalleryJSON(out string) ([]galleryFile, error) {
	out = strings.TrimSpace(out)
	if i := strings.Index(out, "["); i > 0 {
		out = out[i:]
	}
	if out == "" {
		return nil, errors.New("empty")
	}
	var msgs []json.RawMessage
	if err := json.Unmarshal([]byte(out), &msgs); err != nil {
		return nil, err
	}
	var files []galleryFile
	for _, raw := range msgs {
		var m []json.RawMessage
		if json.Unmarshal(raw, &m) != nil || len(m) < 2 {
			continue
		}
		var kind int
		if json.Unmarshal(m[0], &kind) != nil {
			continue
		}
		switch kind {
		case 3:
			var u string
			var meta map[string]any
			if json.Unmarshal(m[1], &u) != nil || len(m) < 3 || json.Unmarshal(m[2], &meta) != nil {
				continue
			}
			files = append(files, galleryFile{URL: u, Meta: meta})
		case -1:
			if len(files) == 0 {
				var e struct {
					Error   string `json:"error"`
					Message string `json:"message"`
				}
				_ = json.Unmarshal(m[1], &e)
				return nil, errors.New(e.Error + ": " + e.Message)
			}
		}
	}
	return files, nil
}

// tiktokProfile lists the posts of a TikTok profile.
func tiktokProfile(ctx context.Context, env Env, link Link) (*Collection, error) {
	info, err := env.dumpJSON(ctx, link.URL, "--flat-playlist")
	if err != nil {
		var ue *queue.UserError
		if errors.As(err, &ue) && strings.Contains(strings.ToLower(ue.Detail), "secondary user id") {
			return nil, queue.Fail(i18n.L("Profil TikTok ini tidak bisa dibaca (dibatasi TikTok). Tempel link videonya satu per satu.", "This TikTok profile can't be read (TikTok restricts it). Paste the video links one by one."), ue.Detail)
		}
		return nil, socialError(err)
	}
	col := &Collection{Source: SourceTikTok, Type: TypeProfile, URL: link.URL, Title: link.ID, postID: strings.TrimPrefix(link.ID, "@")}
	col.Subtitle = firstNonEmpty(info.Uploader, info.Channel)
	col.uploader = strings.TrimPrefix(link.ID, "@")
	for _, en := range info.Entries {
		e, ok := flatEntry(en, "")
		if !ok {
			continue
		}
		if e.Thumbnail == "" && len(en.Thumbnails) > 0 {
			e.Thumbnail = en.Thumbnails[len(en.Thumbnails)-1].URL
		}
		if e.Title == "" {
			e.Title = e.ID
		}
		e.Title = shorten(e.Title, 120)
		e.Kind = KindVideo
		e.archiveKey = "tiktok " + e.ID
		e.uploader = col.uploader
		col.Entries = append(col.Entries, e)
	}
	if len(col.Entries) == 0 {
		return nil, queue.Fail(i18n.L("Tidak ada video yang bisa dibaca di profil ini", "No videos could be read from this profile"), "")
	}
	return col, nil
}

// errGalleryMissing asks the user to install gallery-dl (needed for TikTok/Facebook pictures).
func errGalleryMissing() error {
	return queue.Fail(i18n.L("Post ini berisi foto. Pasang gallery-dl di Pengaturan › Tools pendukung untuk mengunduhnya.", "This post holds pictures. Install gallery-dl in Settings › Helper tools to download them."), "")
}

func unsupportedSocial(source string) error {
	switch source {
	case SourceInstagram:
		return queue.Fail(i18n.L("Link Instagram ini belum didukung. Gunakan link post atau reel (story & profil butuh login).", "This Instagram link isn't supported yet. Use a post or reel link (stories & profiles need a login)."), "")
	case SourceFacebook:
		return queue.Fail(i18n.L("Link Facebook ini belum didukung. Gunakan link video, reel, atau foto.", "This Facebook link isn't supported yet. Use a video, reel or photo link."), "")
	}
	return queue.Fail(i18n.L("Link TikTok ini belum didukung. Gunakan link video, foto, atau profil.", "This TikTok link isn't supported yet. Use a video, photo or profile link."), "")
}

// socialError re-maps a yt-dlp error with the social-media specific messages.
func socialError(err error) error {
	var ue *queue.UserError
	if errors.As(err, &ue) && ue.Detail != "" {
		return friendlySocialError(ue.Detail)
	}
	return err
}

// friendlySocialError maps yt-dlp/gallery-dl output about social posts to a short message.
func friendlySocialError(out string) error {
	detail := strings.TrimSpace(out)
	low := strings.ToLower(detail)
	var msg string
	switch {
	case strings.Contains(low, "ip address is blocked"):
		msg = i18n.L("TikTok menolak akses ke post ini (mungkin sudah dihapus, privat, atau diblokir di jaringan Anda)", "TikTok refused access to this post (it may be deleted, private or blocked on your network)")
	case strings.Contains(low, "login") || strings.Contains(low, "log in") || strings.Contains(low, "rate-limit") ||
		strings.Contains(low, "private") || strings.Contains(low, "requested content is not available") || strings.Contains(low, "authentication"):
		msg = i18n.L("Post ini privat, dibatasi, atau butuh login — belum didukung", "This post is private, restricted or needs a login — not supported yet")
	case strings.Contains(low, "unsupported url"):
		msg = i18n.L("Link ini tidak didukung", "This link isn't supported")
	case strings.Contains(low, "404") || strings.Contains(low, "not found") || strings.Contains(low, "has been removed") || strings.Contains(low, "no longer available") || strings.Contains(low, "video unavailable"):
		msg = i18n.L("Post tidak ditemukan (mungkin sudah dihapus)", "Post not found (it may have been deleted)")
	}
	if msg == "" {
		return friendlyYtError(detail)
	}
	return queue.Fail(msg, detail)
}

// mediaKind tells a video, a sound-only item (TikTok photo post) and a picture apart.
func mediaKind(it ytInfo) string {
	if len(it.Formats) == 0 {
		return KindImage
	}
	for _, f := range it.Formats {
		if f.VCodec != "none" {
			return KindVideo
		}
	}
	return KindAudio
}

func archiveKey(it, parent ytInfo) string {
	ext := strings.ToLower(firstNonEmpty(it.Extractor, parent.Extractor))
	if ext == "" || it.ID == "" {
		return ""
	}
	return ext + " " + it.ID
}

func firstUploader(items []ytInfo) string {
	for _, it := range items {
		if s := firstNonEmpty(it.Uploader, it.Channel, it.UploaderID); s != "" {
			return s
		}
	}
	return ""
}

func firstPositive(v ...int) int {
	for _, n := range v {
		if n > 0 {
			return n
		}
	}
	return 0
}

func metaString(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	}
	return ""
}

func galleryUploader(m map[string]any) string {
	if s := firstNonEmpty(metaString(m, "username"), metaString(m, "user_name")); s != "" {
		return s
	}
	for _, k := range []string{"author", "user"} {
		if sub, ok := m[k].(map[string]any); ok {
			if s := firstNonEmpty(metaString(sub, "uniqueId"), metaString(sub, "name"), metaString(sub, "nickname")); s != "" {
				return s
			}
		}
	}
	return ""
}

var (
	reFBStats   = regexp.MustCompile(`^(?:[\d.,]+[KMB]?\s+(?:views?|reactions?|comments?|shares?|plays?)(?:\s*·\s*)?)+\s*\|\s*`)
	reGenericIG = regexp.MustCompile(`^(?:Video|Photo|Post) by \S+$`)
	reSpaces    = regexp.MustCompile(`\s+`)
)

// cleanCaption turns a post title/description into a short one-line caption.
func cleanCaption(source, title, desc, uploader string) string {
	t := strings.TrimSpace(title)
	if source == SourceFacebook {
		t = reFBStats.ReplaceAllString(t, "")
		if uploader != "" {
			t = strings.TrimSuffix(strings.TrimSpace(t), "| "+uploader)
			t = strings.TrimSuffix(strings.TrimSpace(t), "|")
		}
	}
	if t == "" || reGenericIG.MatchString(t) || (uploader != "" && strings.EqualFold(t, uploader)) {
		if d := strings.TrimSpace(desc); d != "" {
			t = d
		}
	}
	t = reSpaces.ReplaceAllString(strings.TrimSpace(t), " ")
	return shorten(t, 120)
}

func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}

// SocialFileName is the file name (without extension) of one item of a social post.
func SocialFileName(col *Collection, e Entry, width int) string {
	caption := shorten(strings.TrimSuffix(col.Title, "…"), 60)
	caption = strings.TrimSuffix(caption, "…")
	name := caption
	if u := firstNonEmpty(e.uploader, col.uploader); u != "" {
		name = u + " - " + name
	}
	if col.Type == TypeProfile {
		name = firstNonEmpty(e.uploader, col.uploader) + " - " + strings.TrimSuffix(shorten(e.Title, 60), "…")
	}
	id := col.postID
	if col.Type == TypeProfile {
		id = e.ID
	}
	if col.Type == TypePost && len(col.Entries) > 1 {
		// Items of a multi-item post sit in a folder named after the caption: keep the file
		// names short so paths stay well below Windows' 260-character limit.
		name = firstNonEmpty(e.uploader, col.uploader, strings.TrimSuffix(shorten(caption, 30), "…"))
	}
	if id != "" {
		name += " [" + id + "]"
	}
	if col.Type == TypePost && len(col.Entries) > 1 {
		switch e.Kind {
		case KindAudio:
			name += " " + i18n.L("musik", "sound")
		default:
			name += fmt.Sprintf(" %0*d", width, e.Index)
		}
	}
	return naming.SanitizeFileName(name)
}

// PostFolderName names the subfolder of a multi-item post or a TikTok profile.
func PostFolderName(col *Collection) string {
	if col.Type == TypeProfile {
		return naming.SanitizeFileName("TikTok " + col.Title)
	}
	name := strings.TrimSuffix(shorten(col.Title, 60), "…")
	if col.uploader != "" {
		name = col.uploader + " - " + name
	}
	if col.postID != "" {
		name += " [" + col.postID + "]"
	}
	return naming.SanitizeFileName(name)
}

// DownloadSocial downloads one item of a social post into dir as name.<ext>.
func DownloadSocial(ctx context.Context, env Env, e Entry, dir, name string, o Options, r queue.Reporter) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", queue.Fail(i18n.L("Tidak bisa membuat folder tujuan", "Can't create the destination folder"), err.Error())
	}
	if e.Kind == KindImage {
		return downloadImage(ctx, env, e, dir, name, o, r)
	}
	if e.Kind == KindAudio {
		o.Mode = "audio"
	}
	phases := 1
	if o.Mode == "video" {
		phases = 2
	}
	r.Message(i18n.L("Menyiapkan…", "Preparing…"))
	r.Progress(-1)
	job := ytJob{URL: e.URL, Dir: dir, Template: escapeTemplate(name), Opts: o, Archive: o.SkipExisting && e.archiveKey != "", Phases: phases, Item: e.item}
	out, err := env.runYtDlp(ctx, job, r, 1)
	if out != "" {
		if st, statErr := os.Stat(out); statErr == nil {
			r.SetOutput(out, st.Size())
		}
	}
	if err != nil {
		return out, socialError(err)
	}
	return out, nil
}

var imageClient = &http.Client{Timeout: 5 * time.Minute}

// downloadImage saves a picture straight from the CDN, optionally converting it to JPG.
func downloadImage(ctx context.Context, env Env, e Entry, dir, name string, o Options, r queue.Reporter) (string, error) {
	ext := e.ext
	if ext == "" {
		ext = extFromURL(e.URL)
	}
	if ext == "jpeg" {
		ext = "jpg"
	}
	finalExt := ext
	if o.ImageFormat == "jpg" {
		finalExt = "jpg"
	}
	target := filepath.Join(dir, name+"."+finalExt)
	if _, err := os.Stat(target); err == nil {
		if o.SkipExisting {
			r.SetOutput(target, fileSize(target))
			return target, queue.Skip(i18n.L("File sudah ada", "File already exists"))
		}
		target = nextFreePath(dir, name, finalExt)
	}
	r.Message(i18n.L("Mengunduh foto…", "Downloading picture…"))
	r.Progress(0)
	raw := naming.TempPath(filepath.Join(dir, name+"."+ext))
	if err := fetchImage(ctx, e.URL, raw, sourceReferer(e.URL), env.RateLimitKB, r); err != nil {
		_ = os.Remove(raw)
		return "", err
	}
	if finalExt != ext {
		r.Message(i18n.L("Mengubah ke JPG…", "Converting to JPG…"))
		tmp := naming.TempPath(target)
		cerr := imageconv.Convert(ctx, raw, tmp, imageconv.Options{Format: "jpg", Quality: 92, ResizeMode: "original", Background: "#ffffff", AutoRotate: true}, env.FFmpeg, func(float64) {})
		_ = os.Remove(raw)
		if cerr != nil {
			_ = os.Remove(tmp)
			return "", queue.Fail(i18n.L("Foto tidak bisa diubah ke JPG", "The picture couldn't be converted to JPG"), cerr.Error())
		}
		raw = tmp
	}
	if err := naming.Commit(raw, target); err != nil {
		_ = os.Remove(raw)
		return "", queue.Fail(i18n.L("Tidak bisa menyimpan file", "Can't save the file"), err.Error())
	}
	r.Progress(1)
	r.SetOutput(target, fileSize(target))
	return target, nil
}

func fetchImage(ctx context.Context, url, dest, referer string, limitKB int, r queue.Reporter) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return queue.Fail(i18n.L("Link foto tidak valid", "Invalid picture link"), err.Error())
	}
	req.Header.Set("User-Agent", browserUA)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := imageClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return queue.Fail(i18n.L("Koneksi bermasalah. Periksa internet lalu coba lagi.", "Connection problem. Check your internet and try again."), err.Error())
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusGone:
		return queue.Fail(i18n.L("Link foto sudah kedaluwarsa. Hapus link lalu periksa lagi.", "The picture link has expired. Remove the link and check it again."), resp.Status)
	case resp.StatusCode != http.StatusOK:
		return queue.Fail(fmt.Sprintf(i18n.L("Foto gagal diunduh (%s)", "Picture download failed (%s)"), resp.Status), resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return queue.Fail(i18n.L("Tidak bisa menyimpan file", "Can't save the file"), err.Error())
	}
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 128*1024)
	started := time.Now()
	for {
		if limitKB > 0 {
			// Wait until the average speed is back under the limit.
			want := time.Duration(float64(done) / float64(limitKB*1024) * float64(time.Second))
			if ahead := want - time.Since(started); ahead > 0 {
				select {
				case <-ctx.Done():
					f.Close()
					return ctx.Err()
				case <-time.After(ahead):
				}
			}
		}
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return queue.Fail(i18n.L("Tidak bisa menyimpan file", "Can't save the file"), werr.Error())
			}
			done += int64(n)
			if total > 0 {
				r.Progress(float64(done) / float64(total) * 0.95)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return queue.Fail(i18n.L("Koneksi bermasalah. Periksa internet lalu coba lagi.", "Connection problem. Check your internet and try again."), rerr.Error())
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	if total > 0 && done != total {
		return queue.Fail(i18n.L("Unduhan foto tidak lengkap", "Incomplete picture download"), "")
	}
	return nil
}

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

func sourceReferer(url string) string {
	switch {
	case strings.Contains(url, "tiktokcdn"):
		return "https://www.tiktok.com/"
	case strings.Contains(url, "cdninstagram") || strings.Contains(url, "instagram."):
		return "https://www.instagram.com/"
	case strings.Contains(url, "fbcdn"):
		return "https://www.facebook.com/"
	}
	return ""
}

func extFromURL(u string) string {
	p := u
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(p), "."))
	if i := strings.Index(ext, "~"); i >= 0 { // TikTok: "....jpeg~tplv-…"
		ext = ext[:i]
	}
	if imageExts[ext] {
		return ext
	}
	return "jpg"
}

func nextFreePath(dir, name, ext string) string {
	for i := 1; i < 10000; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%s (%d).%s", name, i, ext))
		if _, err := os.Stat(p); err != nil {
			return p
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s (%d).%s", name, time.Now().Unix(), ext))
}

func fileSize(p string) int64 {
	if st, err := os.Stat(p); err == nil {
		return st.Size()
	}
	return 0
}

// resolveRedirect follows a short share link (vt.tiktok.com, fb.watch, …) to the real post URL.
func resolveRedirect(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", browserUA)
	resp, err := imageClient.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	return resp.Request.URL.String(), nil
}
