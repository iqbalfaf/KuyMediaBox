//go:build integration

package integration

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"math"
	"os"
	"path/filepath"
	"testing"

	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/mediaconv"
	"kuymediabox/internal/proc"
	"kuymediabox/internal/tools"
)

func TestCueSplitAndContactSheet(t *testing.T) {
	m := setup(t)
	ff, fp := m.Path(tools.FFmpeg), m.Path(tools.FFprobe)
	ctx := context.Background()
	dir := t.TempDir()

	album := filepath.Join(dir, "Band - Album.flac")
	if _, err := proc.Output(ctx, ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=500:duration=20", "-c:a", "flac", album); err != nil {
		t.Fatal(err)
	}
	sheet := "PERFORMER \"Band\"\nTITLE \"Album\"\nFILE \"Band - Album.flac\" WAVE\n" +
		"TRACK 01 AUDIO\nTITLE \"One\"\nINDEX 01 00:00:00\n" +
		"TRACK 02 AUDIO\nTITLE \"Two\"\nINDEX 01 00:05:00\n" +
		"TRACK 03 AUDIO\nTITLE \"Three\"\nINDEX 01 00:12:37\n"
	_ = os.WriteFile(filepath.Join(dir, "Band - Album.cue"), []byte(sheet), 0o644)

	cue, err := mediaconv.ParseCue(mediaconv.FindCue(album))
	if err != nil {
		t.Fatal(err)
	}
	info := probe(t, fp, album)
	want := []float64{5, 7.5067, 20 - 12.4933}
	for i := range cue.Tracks {
		o := mediaconv.AudioOptions{Format: "mp3", Bitrate: 128}
		o.TrimStart, o.TrimEnd = cue.TrackRange(i)
		out := filepath.Join(dir, fmt.Sprintf("%02d.mp3", i+1))
		p, err := mediaconv.AudioPlan(album, out, info, o, cue.TrackTags(i), 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		runPlan(t, ff, p)
		got := probe(t, fp, out)
		if math.Abs(got.Duration-want[i]) > 0.15 {
			t.Errorf("track %d: %.2f s, want %.2f", i+1, got.Duration, want[i])
		}
		if got.Tags["title"] != cue.Tracks[i].Title || got.Tags["album"] != "Album" || got.Tags["artist"] != "Band" {
			t.Errorf("track %d tags %v", i+1, got.Tags)
		}
	}

	// Contact sheet from a test video.
	clip := filepath.Join(dir, "clip.mp4")
	if _, err := proc.Output(ctx, ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=25:duration=8", "-c:v", "libx264", "-pix_fmt", "yuv420p", clip); err != nil {
		t.Fatal(err)
	}
	var cells []imageconv.SheetCell
	for i := 0; i < 6; i++ {
		pic := filepath.Join(dir, fmt.Sprintf("f%d.png", i))
		if err := ffmpeg.Run(ctx, ff, []string{"-ss", fmt.Sprint(float64(i) + 0.5), "-i", clip, "-frames:v", "1", "-vf", "scale=240:-2", pic}, 0, nil); err != nil {
			t.Fatal(err)
		}
		cells = append(cells, imageconv.SheetCell{Path: pic, Label: fmt.Sprintf("0:0%d", i)})
	}
	out := filepath.Join(dir, "sheet.jpg")
	if err := imageconv.ContactSheet(cells, []string{"clip.mp4", "0:08 · 640×360 · H264"}, 3, 240, out, "jpg"); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 3*240+4*6 || cfg.Height < 2*135 {
		t.Errorf("sheet %dx%d", cfg.Width, cfg.Height)
	}
}
