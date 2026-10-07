package mediaconv

import (
	"reflect"
	"strings"
	"testing"

	"kuymediabox/internal/ffmpeg"
)

func TestMatchLangs(t *testing.T) {
	tracks := []ffmpeg.SubTrack{
		{Index: 2, Codec: "ass", Lang: "ind"},
		{Index: 3, Codec: "subrip", Lang: "eng", Title: "English Signs"},
		{Index: 4, Codec: "subrip"},
		{Index: 5, Codec: "subrip", Lang: "fre", Title: "French"},
		{Index: 6, Codec: "subrip", Title: "Indonesian (full)"},
		{Index: 7, Codec: "subrip", Lang: "spa", Title: "Commentary / Standard"},
	}
	idx := func(ts []ffmpeg.SubTrack) []int {
		var out []int
		for _, t := range ts {
			out = append(out, t.Index)
		}
		return out
	}
	for _, c := range []struct {
		langs string
		want  []int
	}{
		{"", []int{2, 3, 4, 5, 6, 7}},
		{"id", []int{2, 6}}, // tag, or "Indonesian" in the title
		{"IND", []int{2, 6}},
		{"en", []int{3}}, // not "French", "Commentary"
		{"ar", nil},      // not "Standard"
		{"en, fr", []int{3, 5}},
		{"signs", []int{3}},
		{"ja", nil},
	} {
		if got := idx(MatchLangs(tracks, c.langs)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %v, want %v", c.langs, got, c.want)
		}
	}
}

func TestSubOutput(t *testing.T) {
	for _, c := range []struct {
		codec, format, ext, enc, mux string
		ok                           bool
	}{
		{"ass", "original", "ass", "copy", "", true},
		{"ass", "srt", "srt", "srt", "", true},
		{"subrip", "original", "srt", "copy", "", true},
		{"subrip", "ass", "ass", "ass", "", true},
		{"subrip", "vtt", "vtt", "webvtt", "", true},
		{"webvtt", "original", "vtt", "copy", "", true},
		{"mov_text", "original", "srt", "srt", "", true},
		{"hdmv_pgs_subtitle", "srt", "sup", "copy", "sup", true},
		{"dvd_subtitle", "original", "mks", "copy", "matroska", true},
		{"dvb_teletext", "original", "", "", "", false},
	} {
		ext, enc, mux, ok := SubOutput(c.codec, c.format)
		if ext != c.ext || enc != c.enc || mux != c.mux || ok != c.ok {
			t.Errorf("%s→%s: got %s %s %s %v", c.codec, c.format, ext, enc, mux, ok)
		}
	}
}

func TestSubTargets(t *testing.T) {
	tracks := []ffmpeg.SubTrack{{Index: 2, Codec: "ass"}, {Index: 3, Codec: "dvb_teletext"}, {Index: 4, Codec: "hdmv_pgs_subtitle"}}
	got := SubTargets(tracks, "srt")
	if len(got) != 2 || got[0].Ext != "srt" || got[0].Encoder != "srt" || got[1].Index != 4 || got[1].Muxer != "sup" {
		t.Fatalf("%+v", got)
	}
	if n := CanExtract(tracks); n != 2 {
		t.Errorf("CanExtract = %d", n)
	}
}

func TestSubSuffixes(t *testing.T) {
	target := func(lang, ext string, forced bool) SubTarget {
		return SubTarget{SubTrack: ffmpeg.SubTrack{Lang: lang, Forced: forced}, Ext: ext}
	}
	got := SubSuffixes([]SubTarget{target("eng", "ass", false), target("eng", "ass", false), target("ind", "srt", false), target("ind", "srt", true), target("", "srt", false)})
	want := []string{".1.eng", ".2.eng", ".ind", ".ind.forced", ""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	// Same language but different file types don't clash.
	if got := SubSuffixes([]SubTarget{target("eng", "ass", false), target("eng", "sup", false)}); !reflect.DeepEqual(got, []string{".eng", ".eng"}) {
		t.Errorf("got %q", got)
	}
	// A tag with no usable characters counts as no language (never a lone ".").
	if got := SubSuffixes([]SubTarget{target("日本", "srt", false), target("", "srt", false)}); !reflect.DeepEqual(got, []string{".1", ".2"}) {
		t.Errorf("got %q", got)
	}
}

func TestFontNames(t *testing.T) {
	got := FontNames([]ffmpeg.Attachment{{Name: "arial.ttf"}, {Name: `..\..\evil.ttf`}, {Name: "ARIAL.ttf"}, {Name: ""}, {Name: "a?b.otf"}})
	want := []string{"arial.ttf", "evil.ttf", "ARIAL (2).ttf", "font4.ttf", "a_b.otf"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubExtractArgs(t *testing.T) {
	subs := SubTargets([]ffmpeg.SubTrack{{Index: 2, Codec: "ass"}, {Index: 5, Codec: "hdmv_pgs_subtitle"}}, "srt")
	subs[0].Path, subs[1].Path = "o.srt", "o.sup"
	fonts := []ffmpeg.Attachment{{Index: 7, Name: "a.ttf"}}
	got := strings.Join(SubExtractArgs("in.mkv", subs, fonts, []string{"f/a.ttf"}), " ")
	want := "-dump_attachment:7 f/a.ttf -i in.mkv -map 0:2 -c:s srt o.srt -map 0:5 -c:s copy -f sup o.sup"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestCleanSRT(t *testing.T) {
	in := "1\n00:00:05,080 --> 00:00:07,920\n<font face=\"Commie Sans\" size=\"52\"><b>Hei, Tweety.</b></FONT>\n"
	want := "1\n00:00:05,080 --> 00:00:07,920\n<b>Hei, Tweety.</b>\n"
	if got := string(CleanSRT([]byte(in))); got != want {
		t.Errorf("got %q", got)
	}
}
