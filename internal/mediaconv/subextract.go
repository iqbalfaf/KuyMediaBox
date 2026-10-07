package mediaconv

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"kuymediabox/internal/ffmpeg"
)

// SubExtractOptions come from the Video page's subtitle mode: subtitle tracks inside a
// video saved as their own files.
type SubExtractOptions struct {
	Format string `json:"format"` // original srt vtt ass (picture subtitles are always copied)
	Langs  string `json:"langs"`  // only these languages ("id, en"); empty = every track
	Fonts  bool   `json:"fonts"`  // also save the attached fonts (for ASS)
}

// Normalize fixes unknown values.
func (o *SubExtractOptions) Normalize() {
	switch o.Format {
	case "srt", "vtt", "ass":
	default:
		o.Format = "original"
	}
	o.Langs = strings.TrimSpace(o.Langs)
}

// langAliases joins ISO 639-1 codes with the 639-2 codes Matroska uses.
var langAliases = [][]string{
	{"id", "ind", "indonesian", "indonesia"}, {"en", "eng", "english", "inggris"}, {"ja", "jpn", "japanese", "jepang"},
	{"ko", "kor", "korean", "korea"}, {"zh", "chi", "zho", "chinese", "mandarin"}, {"ms", "may", "msa", "malay", "melayu"},
	{"ar", "ara", "arabic", "arab"}, {"es", "spa", "spanish", "spanyol"}, {"fr", "fre", "fra", "french", "prancis"},
	{"de", "ger", "deu", "german", "jerman"}, {"pt", "por", "portuguese"}, {"ru", "rus", "russian", "rusia"},
	{"it", "ita", "italian", "italia"}, {"th", "tha", "thai"}, {"vi", "vie", "vietnamese"}, {"hi", "hin", "hindi"},
	{"tr", "tur", "turkish", "turki"}, {"nl", "dut", "nld", "dutch", "belanda"}, {"pl", "pol", "polish"},
	{"sv", "swe", "swedish"}, {"tl", "tgl", "fil", "tagalog", "filipino"}, {"jv", "jav", "javanese", "jawa"},
	{"su", "sun", "sundanese", "sunda"}, {"uk", "ukr", "ukrainian"}, {"he", "heb", "hebrew"}, {"el", "gre", "ell", "greek"},
}

// langSet returns every code a language tag or user word may stand for.
func langSet(s string) []string {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, a := range langAliases {
		for _, c := range a {
			if c == s {
				return a
			}
		}
	}
	return []string{s}
}

// MatchLangs keeps the tracks in one of the languages listed in langs (separated by commas
// or spaces). A word also matches a whole word of a track's title ("signs", "english").
// Empty langs keeps all.
func MatchLangs(tracks []ffmpeg.SubTrack, langs string) []ffmpeg.SubTrack {
	words := strings.FieldsFunc(strings.ToLower(langs), func(r rune) bool { return r == ',' || r == ';' || r == ' ' })
	if len(words) == 0 {
		return tracks
	}
	var out []ffmpeg.SubTrack
	for _, t := range tracks {
		if matchTrack(t, words) {
			out = append(out, t)
		}
	}
	return out
}

func matchTrack(t ffmpeg.SubTrack, words []string) bool {
	title := strings.FieldsFunc(strings.ToLower(t.Title), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for _, w := range words {
		for _, c := range langSet(w) {
			if t.Lang != "" && c == t.Lang {
				return true
			}
			for _, tw := range title {
				if tw == c {
					return true
				}
			}
		}
	}
	return false
}

// SubOutput tells how a subtitle track is saved: the file extension, the encoder ("copy"
// keeps it as is) and the muxer to force ("" = from the extension). ok is false for
// subtitles that can't be saved on their own (teletext, closed captions, …).
func SubOutput(codec, format string) (ext, encoder, muxer string, ok bool) {
	switch codec {
	case "hdmv_pgs_subtitle":
		return "sup", "copy", "sup", true
	case "dvd_subtitle", "dvb_subtitle":
		return "mks", "copy", "matroska", true
	}
	if !textSubs[codec] {
		return "", "", "", false
	}
	if format == "original" || format == "" {
		switch codec {
		case "ass", "ssa":
			format = "ass"
		case "webvtt":
			format = "vtt"
		default:
			format = "srt"
		}
	}
	switch format {
	case "ass":
		if codec == "ass" || codec == "ssa" {
			return "ass", "copy", "", true
		}
		return "ass", "ass", "", true
	case "vtt":
		if codec == "webvtt" {
			return "vtt", "copy", "", true
		}
		return "vtt", "webvtt", "", true
	default:
		if codec == "subrip" || codec == "srt" {
			return "srt", "copy", "", true
		}
		return "srt", "srt", "", true
	}
}

// SubTarget is a subtitle track and how it is saved.
type SubTarget struct {
	ffmpeg.SubTrack
	Ext     string // file extension
	Encoder string // "copy" or the subtitle encoder
	Muxer   string // forced output format ("" = from the extension)
	Path    string // output file, set by the caller
}

// SubTargets plans how each track is saved in format; tracks that can't be saved on their
// own are left out.
func SubTargets(tracks []ffmpeg.SubTrack, format string) []SubTarget {
	var out []SubTarget
	for _, t := range tracks {
		if ext, enc, mux, ok := SubOutput(t.Codec, format); ok {
			out = append(out, SubTarget{SubTrack: t, Ext: ext, Encoder: enc, Muxer: mux})
		}
	}
	return out
}

// CanExtract counts the tracks that can be saved as files.
func CanExtract(tracks []ffmpeg.SubTrack) int {
	return len(SubTargets(tracks, "original"))
}

// SubSuffixes names each target's file so players pick it up next to the video: "" for
// movie.srt, ".eng" for movie.eng.srt, ".eng.forced"; targets that would get the same name
// get their number in front (".2.eng").
func SubSuffixes(ts []SubTarget) []string {
	out := make([]string, len(ts))
	count := map[string]int{}
	for i, t := range ts {
		if tag := safeTag(t.Lang); tag != "" {
			out[i] = "." + tag
		}
		if t.Forced {
			out[i] += ".forced"
		}
		count[out[i]+"|"+t.Ext]++
	}
	for i, t := range ts {
		if count[out[i]+"|"+t.Ext] > 1 {
			out[i] = "." + strconv.Itoa(i+1) + out[i]
		}
	}
	return out
}

// safeTag keeps a language tag usable in a file name.
func safeTag(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return -1
	}, strings.ToLower(s))
}

// FontNames gives every attached font a safe, unique file name (attachment names come
// from the file and may hold folder parts or characters Windows doesn't allow).
func FontNames(fonts []ffmpeg.Attachment) []string {
	out := make([]string, len(fonts))
	used := map[string]bool{}
	for i, f := range fonts {
		name := filepath.Base(strings.ReplaceAll(f.Name, `\`, "/"))
		name = strings.Map(func(r rune) rune {
			if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
				return '_'
			}
			return r
		}, name)
		name = strings.Trim(name, ". ")
		if name == "" {
			name = "font" + strconv.Itoa(i+1) + ".ttf"
		}
		stem, ext := strings.TrimSuffix(name, filepath.Ext(name)), filepath.Ext(name)
		for k := 2; used[strings.ToLower(name)]; k++ {
			name = stem + " (" + strconv.Itoa(k) + ")" + ext
		}
		used[strings.ToLower(name)] = true
		out[i] = name
	}
	return out
}

// SubExtractArgs saves every target to its Path and fonts[i] to fontOuts[i] in one ffmpeg run.
func SubExtractArgs(in string, subs []SubTarget, fonts []ffmpeg.Attachment, fontOuts []string) []string {
	var args []string
	for i, f := range fonts {
		args = append(args, "-dump_attachment:"+strconv.Itoa(f.Index), fontOuts[i])
	}
	args = append(args, "-i", in)
	for _, t := range subs {
		args = append(args, "-map", "0:"+strconv.Itoa(t.Index), "-c:s", t.Encoder)
		if t.Muxer != "" {
			args = append(args, "-f", t.Muxer)
		}
		args = append(args, t.Path)
	}
	return args
}

var reFontTag = regexp.MustCompile(`(?i)</?font[^>]*>`)

// CleanSRT removes the <font> tags ffmpeg writes when it turns ASS into SRT: their sizes are
// in the ASS script's units and make the text huge in many players. <b>, <i> and <u> stay.
func CleanSRT(data []byte) []byte {
	return reFontTag.ReplaceAll(data, nil)
}
