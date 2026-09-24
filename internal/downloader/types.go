package downloader

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
}

// Options are the per-link download settings from the UI.
type Options struct {
	Mode         string `json:"mode"`         // video | audio
	Quality      string `json:"quality"`      // best | 1080 | 720 | 480
	Container    string `json:"container"`    // mp4 | mkv
	AudioFormat  string `json:"audioFormat"`  // mp3 | m4a | opus | flac
	AudioQuality string `json:"audioQuality"` // auto | 192 | 320 (Spotify)
	Embed        bool   `json:"embed"`
	SkipExisting bool   `json:"skipExisting"`
	Numbering    bool   `json:"numbering"`   // prefix list position
	ImageFormat  string `json:"imageFormat"` // original | jpg (pictures from social posts)
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
	case "best", "1080", "720", "480":
	default:
		o.Quality = "1080"
	}
	if o.Container != "mkv" {
		o.Container = "mp4"
	}
	switch o.AudioFormat {
	case "mp3", "m4a", "opus", "flac":
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
	case "auto", "192", "320":
	default:
		o.AudioQuality = "auto"
	}
}

// Ext returns the final file extension for the options.
func (o Options) Ext() string {
	if o.Mode == "audio" {
		return o.AudioFormat
	}
	return o.Container
}
