package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/queue"
)

// Reddit pictures and galleries. Reddit refuses anonymous API clients (gallery-dl gets
// "blocked by network security"), but answers a post's .json to a browser that first
// visited old.reddit.com for its cookies — the same way yt-dlp reads Reddit videos.

type redditPost struct {
	ID               string                 `json:"id"`
	Title            string                 `json:"title"`
	Author           string                 `json:"author"`
	Subreddit        string                 `json:"subreddit"`
	URL              string                 `json:"url"`
	URLOverridden    string                 `json:"url_overridden_by_dest"`
	PostHint         string                 `json:"post_hint"`
	IsGallery        bool                   `json:"is_gallery"`
	IsVideo          bool                   `json:"is_video"`
	Thumbnail        string                 `json:"thumbnail"`
	GalleryData      *redditItems           `json:"gallery_data"`
	MediaMetadata    map[string]redditMedia `json:"media_metadata"`
	CrosspostParents []redditPost           `json:"crosspost_parent_list"`
}

type redditItems struct {
	Items []struct {
		MediaID string `json:"media_id"`
		Caption string `json:"caption"`
	} `json:"items"`
}

type redditMedia struct {
	Status string `json:"status"`
	E      string `json:"e"` // Image | AnimatedImage
	M      string `json:"m"` // image/png
	S      struct {
		U   string `json:"u"`
		Gif string `json:"gif"`
		MP4 string `json:"mp4"`
	} `json:"s"`
}

func redditClient(ctx context.Context) (*http.Client, error) {
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 30 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://old.reddit.com/", nil)
	req.Header.Set("User-Agent", browserUA)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	return c, nil
}

// redditJSONURL is the .json address of a post link.
func redditJSONURL(link Link) string {
	if link.ID != "" {
		return "https://www.reddit.com/comments/" + link.ID + "/.json?raw_json=1"
	}
	u := strings.SplitN(link.URL, "?", 2)[0]
	return strings.TrimSuffix(u, "/") + "/.json?raw_json=1"
}

// redditPictures reads the pictures of a Reddit post (one image or a gallery).
func redditPictures(ctx context.Context, link Link) (*Collection, error) {
	c, err := redditClient(ctx)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, redditJSONURL(link), nil)
	req.Header.Set("User-Agent", browserUA)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit: %s", resp.Status)
	}
	var listing []struct {
		Data struct {
			Children []struct {
				Data redditPost `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&listing); err != nil {
		return nil, err
	}
	if len(listing) == 0 || len(listing[0].Data.Children) == 0 {
		return nil, errors.New("reddit: post not found")
	}
	return redditCollection(link, listing[0].Data.Children[0].Data)
}

// redditCollection turns a post into picture entries.
func redditCollection(link Link, p redditPost) (*Collection, error) {
	src := p
	if !p.IsGallery && p.PostHint != "image" && len(p.CrosspostParents) > 0 {
		src = p.CrosspostParents[0] // a crosspost shows the original's pictures
	}
	col := &Collection{Source: SourceReddit, Type: TypePost, URL: link.URL, postID: p.ID, uploader: "u/" + p.Author,
		Title: strings.TrimSpace(p.Title), Subtitle: "r/" + p.Subreddit}
	add := func(url, ext string) {
		n := len(col.Entries) + 1
		col.Entries = append(col.Entries, Entry{ID: fmt.Sprintf("%s-%d", p.ID, n), Kind: KindImage, URL: url, Thumbnail: url,
			Title: i18n.F("Foto %d", "Photo %d", n), ext: ext, uploader: col.uploader})
	}
	switch {
	case src.IsGallery && src.GalleryData != nil:
		for _, it := range src.GalleryData.Items {
			m, ok := src.MediaMetadata[it.MediaID]
			if !ok || (m.Status != "" && m.Status != "valid") {
				continue
			}
			ext := strings.TrimPrefix(m.M, "image/")
			switch {
			case m.E == "AnimatedImage" && m.S.MP4 != "":
				add(m.S.MP4, "mp4")
			case m.E == "AnimatedImage" && m.S.Gif != "":
				add(m.S.Gif, "gif")
			case ext != "":
				if ext == "jpeg" {
					ext = "jpg"
				}
				add("https://i.redd.it/"+it.MediaID+"."+ext, ext)
			case m.S.U != "":
				add(m.S.U, "")
			}
		}
	case src.PostHint == "image" || isPictureURL(firstNonEmpty(src.URLOverridden, src.URL)):
		u := firstNonEmpty(src.URLOverridden, src.URL)
		add(u, extFromURL(u))
	}
	if len(col.Entries) == 0 {
		return nil, queue.Fail(i18n.L("Post Reddit ini tidak berisi video atau foto", "This Reddit post has no video or picture"), "")
	}
	if col.Title == "" {
		col.Title = p.ID
	}
	if len(col.Entries) == 1 {
		col.Entries[0].Title = col.Title
	}
	return col, nil
}

func isPictureURL(u string) bool {
	switch extFromURL(u) {
	case "jpg", "jpeg", "png", "gif", "webp":
		return true
	}
	return false
}
