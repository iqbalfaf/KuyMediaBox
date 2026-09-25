package downloader

import "strings"

// Entry is one downloadable item of a collection.
type Entry struct {
	ID          string  `json:"id"`
	URL         string  `json:"url"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album"`
	Duration    float64 `json:"duration"`
	Date        string  `json:"date"`  // YYYYMMDD when known
	Index       int     `json:"index"` // 1-based position in the list
	Thumbnail   string  `json:"thumbnail"`
	Tab         string  `json:"tab"`  // channel only: videos | shorts | streams
	Kind        string  `json:"kind"` // video | audio | image (social posts); empty = video
	Archived    bool    `json:"archived"`
	Unavailable bool    `json:"unavailable"`
	Source      string  `json:"source"` // Spotify: YouTube link chosen by hand ("" = automatic match)

	song       *spotifySong // Spotify metadata used for tagging
	archiveKey string       // yt-dlp archive line ("<extractor> <id>")
	item       int          // position inside the post for yt-dlp --playlist-items / gallery-dl --range
	uploader   string
	ext        string // picture extension when known
}

// Collection is what a pasted link contains.
type Collection struct {
	Key       string         `json:"key"`
	Source    string         `json:"source"`
	Type      string         `json:"type"`
	URL       string         `json:"url"`
	Title     string         `json:"title"`
	Subtitle  string         `json:"subtitle"`
	Thumbnail string         `json:"thumbnail"`
	Entries   []Entry        `json:"entries"`
	TabCounts map[string]int `json:"tabCounts"`

	postID   string // social posts: used in file and folder names
	uploader string
	hasSound bool // TikTok photo post with a soundtrack
}

// Env holds the tool paths a download needs.
type Env struct {
	YtDlp       string
	FFmpeg      string
	SpotDL      string
	GalleryDL   string
	JSKind      string // deno | node
	JSPath      string
	ArchivePath string
	TempDir     string

	CookiesBrowser string // read the login cookies of this browser (chrome, edge, firefox, …)
	CookiesFile    string // or a cookies.txt file
	SpotifyAuth    bool   // spotDL --user-auth: log in to Spotify (private playlists)
	NameTemplate   string // YouTube & other sites: {title} {uploader} {date} {id} … ("" = built-in)
	SpotifyTpl     string // Spotify: {artist} {title} {album} {year} {index}
	RateLimitKB    int    // speed limit of one download in KB/s (0 = none)
}

// Options are the per-link download settings from the UI.
type Options struct {
	Mode         string `json:"mode"`         // video | audio
	Quality      string `json:"quality"`      // best | 2160 | 1440 | 1080 | 720 | 480
	Container    string `json:"container"`    // mp4 | mkv
	AudioFormat  string `json:"audioFormat"`  // mp3 | m4a | opus | flac | wav
	AudioQuality string `json:"audioQuality"` // auto (best VBR) or a bitrate in kbps: 96 128 160 192 256 320
	Embed        bool   `json:"embed"`
	SkipExisting bool   `json:"skipExisting"`
	Numbering    bool   `json:"numbering"`   // prefix list position
	ImageFormat  string `json:"imageFormat"` // original | jpg (pictures from social posts)

	Subtitles    string `json:"subtitles"`    // none | file | embed
	SubLangs     string `json:"subLangs"`     // e.g. "id,en"
	SectionStart string `json:"sectionStart"` // download only part of a video ("" = from the start)
	SectionEnd   string `json:"sectionEnd"`   // ("" = to the end)
	SponsorBlock string `json:"sponsorBlock"` // off | mark | remove (YouTube)
	Playlist     bool   `json:"playlist"`     // write an .m3u8 playlist for albums/playlists
}

// Normalize fills defaults.
func (o *Options) Normalize(source string) {
	if source == SourceSpotify {
		o.Mode = "audio"
	}
	if o.Mode != "audio" {
		o.Mode = "video"
	}
	switch o.Quality {
	case "best", "2160", "1440", "1080", "720", "480":
	default:
		o.Quality = "1080"
	}
	if o.Container != "mkv" {
		o.Container = "mp4"
	}
	switch o.AudioFormat {
	case "mp3", "m4a", "opus", "flac", "wav":
	default:
		o.AudioFormat = "mp3"
	}
	if source == SourceSpotify && o.AudioFormat == "flac" {
		o.AudioFormat = "mp3"
	}
	if o.ImageFormat != "jpg" {
		o.ImageFormat = "original"
	}
	switch o.AudioQuality {
	case "auto", "96", "128", "160", "192", "256", "320":
	default:
		o.AudioQuality = "auto"
	}
	// Lossless formats have no bitrate to choose.
	if o.AudioFormat == "flac" || o.AudioFormat == "wav" {
		o.AudioQuality = "auto"
	}
	switch o.Subtitles {
	case "file", "embed":
	default:
		o.Subtitles = "none"
	}
	o.SubLangs = cleanLangs(o.SubLangs)
	switch o.SponsorBlock {
	case "mark", "remove":
	default:
		o.SponsorBlock = "off"
	}
	if source != SourceYouTube {
		o.SponsorBlock = "off"
	}
	o.SectionStart, o.SectionEnd = strings.TrimSpace(o.SectionStart), strings.TrimSpace(o.SectionEnd)
}

// cleanLangs keeps a comma-separated list of language codes ("id,en", "en.*").
func cleanLangs(s string) string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		ok := p != ""
		for _, r := range p {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.' || r == '*' || r == '_') {
				ok = false
			}
		}
		if ok {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return "id,en"
	}
	return strings.Join(out, ",")
}

// Ext returns the final file extension for the options.
func (o Options) Ext() string {
	if o.Mode == "audio" {
		return o.AudioFormat
	}
	return o.Container
}
