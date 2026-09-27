package whisper

import (
	"strings"
	"testing"
)

func TestReflow(t *testing.T) {
	srt := "1\n00:00:00,000 --> 00:00:05,280\n Hello everyone! This is a short test of automatic subtitles.\n\n" +
		"2\n00:00:05,280 --> 00:00:09,280\n The quick brown fox jumps over the lazy dog.\n\n" +
		"3\n00:00:09,280 --> 00:00:20,000\n" + strings.Repeat("kata panjang sekali ", 12) + "akhir.\n"
	cues := ParseCues(srt)
	if len(cues) != 3 || cues[1].Start != 5.28 || cues[1].End != 9.28 {
		t.Fatalf("parse: %+v", cues)
	}
	out := Reflow(cues, 42)
	for _, c := range out {
		lines := strings.Split(c.Text, "\n")
		if len(lines) > 2 {
			t.Errorf("more than two lines: %q", c.Text)
		}
		for _, l := range lines {
			if len([]rune(l)) > 42*6/5 { // a folded-in orphan may overflow a little
				t.Errorf("line too long: %q", l)
			}
		}
		if c.End <= c.Start {
			t.Errorf("bad timing %+v", c)
		}
	}
	// The fox sentence (44 chars) stays one cue on two lines: no one-word "dog." cue.
	if lines := strings.Split(out[1].Text, "\n"); len(lines) != 2 || out[1].Start != 5.28 || out[1].End != 9.28 {
		t.Errorf("fox cue %+v", out[1])
	}
	// The long third cue is split and keeps its overall time span.
	var third []Cue
	for _, c := range out {
		if c.Start >= 9.28 {
			third = append(third, c)
		}
	}
	if len(third) < 2 || third[0].Start != 9.28 || third[len(third)-1].End != 20 {
		t.Errorf("split cue %+v", third)
	}
	if s := FormatSRT(out[:1]); !strings.HasPrefix(s, "1\n00:00:00,000 --> 00:00:05,280\n") {
		t.Errorf("srt %q", s)
	}
	if v := FormatVTT(out[:1]); !strings.Contains(v, "00:00:00.000 --> 00:00:05.280") {
		t.Errorf("vtt %q", v)
	}
	// A BOM and CRLF line ends are fine.
	if c := ParseCues(string(rune(0xFEFF)) + "1\r\n00:00:01,500 --> 00:00:02,000\r\nHi\r\n"); len(c) != 1 || c[0].Start != 1.5 {
		t.Errorf("bom/crlf %+v", c)
	}
}
