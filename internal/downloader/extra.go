package downloader

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"kuymediabox/internal/naming"
)

// ParseClock reads "90", "1:30", "01:02:03.5" as seconds (ok=false for bad input, "" = 0).
func ParseClock(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return 0, true
	}
	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, false
	}
	total := 0.0
	for i, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || v < 0 || (i > 0 && v >= 60) {
			return 0, false
		}
		total = total*60 + v
	}
	return total, true
}

// sectionArg builds yt-dlp's --download-sections value ("*10-95.5", "*10-inf") or "".
func sectionArg(start, end string) string {
	s, ok1 := ParseClock(start)
	e, ok2 := ParseClock(end)
	if !ok1 || !ok2 || (s == 0 && e == 0) || (e > 0 && e <= s) {
		return ""
	}
	to := "inf"
	if e > 0 {
		to = strconv.FormatFloat(e, 'f', -1, 64)
	}
	return "*" + strconv.FormatFloat(s, 'f', -1, 64) + "-" + to
}

var reToken = regexp.MustCompile(`\{([a-z]+)\}`)

// Template tokens for YouTube and other sites, mapped to yt-dlp fields.
var ytTokens = map[string]string{
	"title":    "%(title).150B",
	"uploader": "%(uploader,channel|)s",
	"channel":  "%(channel,uploader|)s",
	"date":     "%(upload_date>%Y-%m-%d|)s",
	"year":     "%(upload_date>%Y|)s",
	"id":       "%(id)s",
	"playlist": "%(playlist_title|)s",
	"res":      "%(height|)sp",
}

// YtTemplate turns a name template ("{uploader} - {title}") into a yt-dlp output template.
// Unknown tokens stay as literal text; {index} is the position in the list.
func YtTemplate(tpl string, index int) string {
	var b strings.Builder
	last := 0
	for _, m := range reToken.FindAllStringSubmatchIndex(tpl, -1) {
		b.WriteString(escapeTemplate(tpl[last:m[0]]))
		key := tpl[m[2]:m[3]]
		switch {
		case key == "index":
			b.WriteString(fmt.Sprintf("%02d", index))
		case ytTokens[key] != "":
			b.WriteString(ytTokens[key])
		default:
			b.WriteString(escapeTemplate(tpl[m[0]:m[1]]))
		}
		last = m[1]
	}
	b.WriteString(escapeTemplate(tpl[last:]))
	return b.String()
}

// FillTemplate replaces {tokens} with values (unknown tokens are kept).
func FillTemplate(tpl string, vals map[string]string) string {
	out := reToken.ReplaceAllStringFunc(tpl, func(t string) string {
		if v, ok := vals[t[1:len(t)-1]]; ok {
			return v
		}
		return t
	})
	out = strings.Join(strings.Fields(out), " ")
	return strings.Trim(out, " -_.")
}

func firstOf(list []string) string {
	if len(list) > 0 {
		return list[0]
	}
	return ""
}

// sortArtist orders an artist's songs by album (newest first as spotDL lists them), then track.
func sortArtist(songs []spotifySong) {
	order := map[string]int{}
	for _, s := range songs {
		if _, ok := order[s.AlbumName]; !ok {
			order[s.AlbumName] = len(order)
		}
	}
	sort.SliceStable(songs, func(i, j int) bool {
		a, b := songs[i], songs[j]
		if order[a.AlbumName] != order[b.AlbumName] {
			return order[a.AlbumName] < order[b.AlbumName]
		}
		if a.DiscNumber != b.DiscNumber {
			return a.DiscNumber < b.DiscNumber
		}
		return a.TrackNumber < b.TrackNumber
	})
}

// PlaylistItem is one line of an .m3u8 playlist.
type PlaylistItem struct {
	Index    int
	Path     string
	Title    string
	Duration float64
}

// WriteM3U writes an extended M3U playlist (UTF-8, paths relative to the playlist).
func WriteM3U(path string, items []PlaylistItem) error {
	sort.SliceStable(items, func(i, j int) bool { return items[i].Index < items[j].Index })
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	dir := filepath.Dir(path)
	for _, it := range items {
		rel, err := filepath.Rel(dir, it.Path)
		if err != nil {
			rel = it.Path
		}
		dur := int(it.Duration + 0.5)
		if dur <= 0 {
			dur = -1
		}
		fmt.Fprintf(&b, "#EXTINF:%d,%s\n%s\n", dur, strings.ReplaceAll(it.Title, "\n", " "), rel)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// PlaylistName is the file name of a collection's playlist.
func PlaylistName(col *Collection) string {
	return naming.SanitizeFileName(firstNonEmpty(col.Title, "playlist")) + ".m3u8"
}
