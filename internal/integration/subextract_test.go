//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/mediaconv"
	"kuymediabox/internal/proc"
	"kuymediabox/internal/tools"
)

const testSRT = "1\n00:00:00,500 --> 00:00:02,000\nHalo dunia\n"

const testASS = `[Script Info]
ScriptType: v4.00+

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,2,10,10,10,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:00.50,0:00:02.00,Default,,0,0,0,,{\b1}Hello world
`

// TestSubExtractWithFFmpeg saves the subtitle tracks and fonts of an MKV with the real ffmpeg.
func TestSubExtractWithFFmpeg(t *testing.T) {
	m := setup(t)
	ff, fp := m.Path(tools.FFmpeg), m.Path(tools.FFprobe)
	ctx := context.Background()
	dir := t.TempDir()
	srt, ass := filepath.Join(dir, "id.srt"), filepath.Join(dir, "en.ass")
	os.WriteFile(srt, []byte(testSRT), 0o644)
	os.WriteFile(ass, []byte(testASS), 0o644)
	font := filepath.Join(os.Getenv("WINDIR"), "Fonts", "arial.ttf")
	args := []string{"-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=3", "-i", srt, "-i", ass,
		"-map", "0", "-map", "1", "-map", "2", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:s", "copy",
		"-metadata:s:s:0", "language=ind", "-metadata:s:s:1", "language=eng"}
	hasFont := false
	if _, err := os.Stat(font); err == nil {
		hasFont = true
		args = append(args, "-attach", font, "-metadata:s:t", "mimetype=application/x-truetype-font")
	}
	mkv := filepath.Join(dir, "film.mkv")
	if _, err := proc.Output(ctx, ff, append(args, mkv)...); err != nil {
		t.Fatal(err)
	}
	info := probe(t, fp, mkv)
	if len(info.Subs) != 2 || info.Subs[0].Lang != "ind" || info.Subs[1].Codec != "ass" {
		t.Fatalf("subs %+v", info.Subs)
	}
	if hasFont && len(info.Fonts) != 1 {
		t.Fatalf("fonts %+v", info.Fonts)
	}

	extract := func(t *testing.T, format, langs string, fonts bool) (map[string]string, []string) {
		t.Helper()
		out := t.TempDir()
		subs := mediaconv.SubTargets(mediaconv.MatchLangs(info.Subs, langs), format)
		for i, s := range mediaconv.SubSuffixes(subs) {
			subs[i].Path = filepath.Join(out, "film"+s+"."+subs[i].Ext)
		}
		var att []ffmpeg.Attachment
		var fontOuts []string
		if fonts {
			att = info.Fonts
			for _, n := range mediaconv.FontNames(att) {
				fontOuts = append(fontOuts, filepath.Join(out, n))
			}
		}
		if err := ffmpeg.RunIn(ctx, ff, out, mediaconv.SubExtractArgs(mkv, subs, att, fontOuts), info.Duration, nil); err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, sub := range subs {
			data, err := os.ReadFile(sub.Path)
			if err != nil {
				t.Fatal(err)
			}
			got[filepath.Base(sub.Path)] = string(data)
		}
		return got, fontOuts
	}

	t.Run("original", func(t *testing.T) {
		got, _ := extract(t, "original", "", false)
		if !strings.Contains(got["film.ind.srt"], "Halo dunia") || !strings.Contains(got["film.eng.ass"], `{\b1}Hello world`) {
			t.Fatalf("%q", got)
		}
	})
	t.Run("srt", func(t *testing.T) {
		got, _ := extract(t, "srt", "", false)
		en := got["film.eng.srt"]
		if !strings.Contains(en, "Hello world") || strings.Contains(en, `{\b1}`) || !strings.Contains(en, "-->") {
			t.Fatalf("%q", got)
		}
	})
	t.Run("vtt filtered", func(t *testing.T) {
		got, _ := extract(t, "vtt", "id", false)
		if len(got) != 1 || !strings.HasPrefix(got["film.ind.vtt"], "WEBVTT") {
			t.Fatalf("%q", got)
		}
	})
	t.Run("fonts", func(t *testing.T) {
		if !hasFont {
			t.Skip("no arial.ttf on this PC")
		}
		_, fonts := extract(t, "original", "en", true)
		st, err := os.Stat(fonts[0])
		if err != nil || st.Size() < 10_000 {
			t.Fatalf("font %v %v", st, err)
		}
	})
}
