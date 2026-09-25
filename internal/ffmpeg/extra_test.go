package ffmpeg

import "testing"

func TestParseSilence(t *testing.T) {
	log := `[silencedetect @ 0x1] silence_start: 0
[silencedetect @ 0x1] silence_end: 1.52 | silence_duration: 1.52
[silencedetect @ 0x1] silence_start: 30.1
[silencedetect @ 0x1] silence_end: 31 | silence_duration: 0.9
[silencedetect @ 0x1] silence_start: 58.25`
	s, e, err := parseSilence(log, 60)
	if err != nil || s != 1.52 || e != 58.25 {
		t.Fatalf("got %v %v %v", s, e, err)
	}
	s, e, _ = parseSilence("silence_start: 10\nsilence_end: 12", 60)
	if s != 0 || e != 0 {
		t.Fatalf("middle silence only: %v %v", s, e)
	}
}
