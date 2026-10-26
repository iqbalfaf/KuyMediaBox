package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
)

// Stream is a direct address of an online video or song, for the trim preview.
type Stream struct {
	URL      string
	Headers  map[string]string // what yt-dlp would send (user agent, referer, cookies)
	Protocol string            // https, m3u8_native, …
	Ext      string
	Video    bool
	Audio    bool
}

// Playable reports whether the app's browser can play the stream itself (one http file in
// a container it knows); otherwise the preview shows frames made with FFmpeg.
func (s Stream) Playable() bool {
	return strings.HasPrefix(s.Protocol, "http") && !strings.Contains(s.Protocol, "dash") &&
		(s.Ext == "mp4" || s.Ext == "webm" || s.Ext == "m4a" || s.Ext == "mp3" || s.Ext == "ogg" || s.Ext == "opus")
}

// Small single files first (quick to load, with sound), then anything yt-dlp can give.
const previewFormat = "b[height<=480][protocol^=http][vcodec!=none][acodec!=none]" +
	"/b[height<=720][protocol^=http][vcodec!=none][acodec!=none]" +
	"/bv*[height<=480][protocol^=http]/ba[protocol^=http]/b[height<=720]/b"

type streamInfo struct {
	URL      string            `json:"url"`
	Protocol string            `json:"protocol"`
	Ext      string            `json:"ext"`
	VCodec   string            `json:"vcodec"`
	ACodec   string            `json:"acodec"`
	Headers  map[string]string `json:"http_headers"`
	Cookies  string            `json:"cookies"`
	Formats  []streamInfo      `json:"requested_formats"`
}

// PreviewStream asks yt-dlp for a stream of url the trim dialog can show.
func PreviewStream(ctx context.Context, env Env, url string) (Stream, error) {
	if env.YtDlp == "" {
		return Stream{}, errors.New(i18n.L("yt-dlp belum terpasang", "yt-dlp is not installed"))
	}
	args := append(env.commonArgs(), "-J", "--no-playlist", "-f", previewFormat, "--", url)
	out, err := proc.Output(ctx, env.YtDlp, args...)
	if err != nil {
		if ctx.Err() != nil {
			return Stream{}, ctx.Err()
		}
		return Stream{}, friendlyYtError(err.Error())
	}
	var info streamInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		return Stream{}, fmt.Errorf(i18n.L("jawaban yt-dlp tidak bisa dibaca: %w", "yt-dlp response can't be read: %w"), err)
	}
	return pickStream(info)
}

func pickStream(info streamInfo) (Stream, error) {
	f := info
	if f.URL == "" && len(info.Formats) > 0 {
		// A merged pick (video+audio): the video part is enough for the picture.
		f = info.Formats[0]
		for _, x := range info.Formats {
			if x.VCodec != "none" && x.VCodec != "" {
				f = x
				break
			}
		}
	}
	if f.URL == "" {
		return Stream{}, errors.New(i18n.L("tidak ada stream yang bisa dipratinjau", "no stream to preview"))
	}
	// Only web addresses: the URL goes to FFmpeg, which would open local files too.
	if u := strings.ToLower(f.URL); !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		return Stream{}, errors.New(i18n.L("alamat stream tidak didukung", "unsupported stream address"))
	}
	s := Stream{URL: f.URL, Protocol: f.Protocol, Ext: f.Ext, Video: f.VCodec != "none", Audio: f.ACodec != "none", Headers: map[string]string{}}
	for k, v := range f.Headers {
		s.Headers[k] = v
	}
	if f.Headers == nil {
		for k, v := range info.Headers {
			s.Headers[k] = v
		}
	}
	if c := cookieHeader(firstNonEmpty(f.Cookies, info.Cookies)); c != "" {
		s.Headers["Cookie"] = c
	}
	return s, nil
}

// cookieHeader turns yt-dlp's cookie field ("a=1; Domain=.x.com; Path=/; Secure; b=2; …")
// into a Cookie header ("a=1; b=2").
func cookieHeader(s string) string {
	var out []string
	for _, part := range strings.Split(s, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || k == "" {
			continue
		}
		switch strings.ToLower(k) {
		case "domain", "path", "expires", "max-age", "samesite", "priority":
			continue
		}
		out = append(out, k+"="+v)
	}
	return strings.Join(out, "; ")
}
