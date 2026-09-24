//go:build integration

// Package integration runs real conversions and downloads with the actual tools.
// Run with: go test -tags "integration nodynamic" ./internal/integration -v -timeout 60m
package integration

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/config"
	"kuymediabox/internal/downloader"
	"kuymediabox/internal/ffmpeg"
	"kuymediabox/internal/imageconv"
	"kuymediabox/internal/mediaconv"
	"kuymediabox/internal/proc"
	"kuymediabox/internal/queue"
	"kuymediabox/internal/tools"
)

var (
	setupOnce sync.Once
	tm        *tools.Manager
)

func setup(t *testing.T) *tools.Manager {
	t.Helper()
	setupOnce.Do(func() {
		cfg := config.Load()
		tm = tools.New(cfg, nil)
		ctx := context.Background()
		tm.Detect(ctx)
		for _, id := range []string{tools.FFmpeg, tools.YtDlp, tools.SpotDL} {
			if tm.Path(id) == "" {
				t.Logf("installing %s …", id)
				start := time.Now()
				if err := tm.Install(ctx, id); err != nil {
					t.Fatalf("install %s: %v", id, err)
				}
				t.Logf("installed %s in %s", id, time.Since(start).Round(time.Second))
			}
		}
		if tm.Path(tools.JSRuntime) == "" {
			if err := tm.Install(ctx, tools.JSRuntime); err != nil {
				t.Fatalf("install js runtime: %v", err)
			}
		}
		tm.CheckUpdates(ctx)
		for _, s := range tm.List() {
			t.Logf("%-10s found=%v v=%s src=%s latest=%s path=%s", s.ID, s.Found, s.Version, s.Source, s.Latest, s.Path)
		}
	})
	if tm == nil || tm.Path(tools.FFmpeg) == "" {
		t.Fatal("tools not ready")
	}
	return tm
}

type rep struct {
	t    *testing.T
	last float64
	out  string
}

func (r *rep) Progress(p float64)             { r.last = p }
func (r *rep) Message(m string)               {}
func (r *rep) SetOutput(p string, size int64) { r.out = p }

func sample(t *testing.T, ff, dir string) (video, vertical, flac string) {
	t.Helper()
	ctx := context.Background()
	video = filepath.Join(dir, "sample.mp4")
	vertical = filepath.Join(dir, "vertical.mov")
	flac = filepath.Join(dir, "lagu.flac")
	cover := filepath.Join(dir, "cover.jpg")
	run := func(args ...string) {
		if _, err := proc.Output(ctx, ff, append([]string{"-hide_banner", "-loglevel", "error", "-y"}, args...)...); err != nil {
			t.Fatalf("make sample: %v", err)
		}
	}
	run("-f", "lavfi", "-i", "testsrc2=size=1920x1080:rate=30:duration=4", "-f", "lavfi", "-i", "sine=frequency=440:duration=4",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", video)
	run("-f", "lavfi", "-i", "testsrc2=size=720x1280:rate=30:duration=2", "-c:v", "libx264", "-pix_fmt", "yuv420p", vertical)
	run("-f", "lavfi", "-i", "color=c=orange:size=500x500", "-frames:v", "1", cover)
	run("-f", "lavfi", "-i", "sine=frequency=220:duration=3", "-i", cover, "-map", "0:a", "-map", "1:v", "-c:a", "flac",
		"-sample_fmt", "s32", "-c:v", "copy", "-disposition:v", "attached_pic", "-metadata", "title=Lagu Uji", "-metadata", "artist=Band Uji", flac)
	return
}

func TestConversions(t *testing.T) {
	m := setup(t)
	ff, probe := m.Path(tools.FFmpeg), m.Path(tools.FFprobe)
	dir := t.TempDir()
	video, vertical, flac := sample(t, ff, dir)
	enc, err := ffmpeg.Encoders(context.Background(), ff)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	vinfo, err := ffmpeg.Probe(ctx, probe, video)
	if err != nil || vinfo.Duration < 3.9 || vinfo.Width != 1920 {
		t.Fatalf("probe video: %+v %v", vinfo, err)
	}
	for format, codecs := range mediaconv.VideoFormats {
		for _, codec := range codecs {
			o := mediaconv.VideoOptions{Format: format, Codec: codec, Quality: "hemat", Resolution: "480", Preset: "fast"}
			if codec != "copy" && codec != "gif" && mediaconv.PickEncoder(codec, enc) == "" {
				t.Logf("skip %s/%s: encoder missing", format, codec)
				continue
			}
			t.Run("video_"+format+"_"+codec, func(t *testing.T) {
				out := filepath.Join(dir, "v_"+codec+"."+format)
				args, err := mediaconv.VideoArgs(video, out, vinfo, o, enc)
				if codec == "copy" && !mediaconv.CanCopyVideo(vinfo.VideoCodec, format) {
					if err == nil {
						t.Fatal("expected a clear refusal for an impossible copy")
					}
					t.Logf("refused as expected: %v", err)
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				r := &rep{t: t}
				start := time.Now()
				if err := ffmpeg.Run(ctx, ff, args, vinfo.Duration, func(p float64, s string) { r.Progress(p) }); err != nil {
					t.Fatalf("%v\n%v", err, err.(*queue.UserError).Detail)
				}
				oi, err := ffmpeg.Probe(ctx, probe, out)
				if err != nil {
					t.Fatalf("output unreadable: %v", err)
				}
				wantH := 480
				if codec == "copy" {
					wantH = 1080
				}
				if oi.Height != wantH {
					t.Errorf("height %d want %d", oi.Height, wantH)
				}
				if format != "gif" && !oi.HasAudio {
					t.Error("audio lost")
				}
				if r.last < 0.99 {
					t.Errorf("progress ended at %.2f", r.last)
				}
				t.Logf("ok in %s", time.Since(start).Round(time.Millisecond))
			})
		}
	}

	t.Run("vertical_720", func(t *testing.T) {
		info, _ := ffmpeg.Probe(ctx, probe, vertical)
		out := filepath.Join(dir, "vert.mp4")
		args, _ := mediaconv.VideoArgs(vertical, out, info, mediaconv.VideoOptions{Format: "mp4", Codec: "h264", Resolution: "480"}, enc)
		if err := ffmpeg.Run(ctx, ff, args, info.Duration, nil); err != nil {
			t.Fatal(err)
		}
		oi, _ := ffmpeg.Probe(ctx, probe, out)
		if oi.Width != 480 || oi.Height != 854 {
			t.Fatalf("got %dx%d", oi.Width, oi.Height)
		}
	})

	finfo, err := ffmpeg.Probe(ctx, probe, flac)
	if err != nil || finfo.CoverIndex < 0 {
		t.Fatalf("flac sample: %+v %v", finfo, err)
	}
	for format := range mediaconv.AudioFormats {
		for _, src := range []struct {
			name string
			path string
			info ffmpeg.Info
		}{{"flac", flac, finfo}, {"video", video, vinfo}} {
			t.Run("audio_"+src.name+"_to_"+format, func(t *testing.T) {
				out := filepath.Join(dir, "a_"+src.name+"."+format)
				args, err := mediaconv.AudioArgs(src.path, out, src.info, mediaconv.AudioOptions{Format: format, Bitrate: 320, KeepMetadata: true, Channels: "stereo", SampleRate: "48000"})
				if err != nil {
					t.Fatal(err)
				}
				if err := ffmpeg.Run(ctx, ff, args, src.info.Duration, nil); err != nil {
					t.Fatalf("%v\n%s", err, err.(*queue.UserError).Detail)
				}
				oi, err := ffmpeg.Probe(ctx, probe, out)
				if err != nil || !oi.HasAudio || oi.HasVideo {
					t.Fatalf("bad output %+v %v", oi, err)
				}
				if src.name == "flac" && (format == "mp3" || format == "m4a" || format == "flac") && oi.CoverIndex < 0 {
					t.Error("cover art lost")
				}
			})
		}
	}

	t.Run("image_fallback_ffmpeg", func(t *testing.T) {
		// A TGA file has no Go decoder: the ffmpeg fallback must handle it.
		tga := filepath.Join(dir, "x.tga")
		if _, err := proc.Output(ctx, ff, "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=64x48", "-frames:v", "1", tga); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, "x.webp")
		if err := imageconv.Convert(ctx, tga, out, imageconv.Options{Format: "webp"}, ff, func(float64) {}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("cancel_kills_ffmpeg", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		out := filepath.Join(dir, "cancel.mp4")
		args := []string{"-f", "lavfi", "-i", "testsrc2=size=1920x1080:rate=60:duration=600", "-c:v", "libx264", "-preset", "veryslow", out}
		done := make(chan error, 1)
		go func() { done <- ffmpeg.Run(cctx, ff, args, 600, nil) }()
		time.Sleep(1500 * time.Millisecond)
		cancel()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("expected cancel error")
			}
		case <-time.After(10 * time.Second):
			t.Fatal("ffmpeg not killed")
		}
	})
}

func env(m *tools.Manager) downloader.Env {
	return downloader.Env{
		YtDlp: m.Path(tools.YtDlp), FFmpeg: m.Path(tools.FFmpeg), SpotDL: m.Path(tools.SpotDL), GalleryDL: m.Path(tools.GalleryDL),
		JSKind: m.JSRuntimeKind(), JSPath: m.Path(tools.JSRuntime),
		ArchivePath: filepath.Join(os.TempDir(), "kmb-it-archive.txt"), TempDir: appdir.TempDir(),
	}
}

func TestYouTube(t *testing.T) {
	m := setup(t)
	e := env(m)
	os.Remove(e.ArchivePath)
	ctx := context.Background()
	dir := t.TempDir()

	col, err := downloader.AnalyzeYouTube(ctx, e, downloader.Detect("https://www.youtube.com/watch?v=jNQXAC9IVRw"))
	if err != nil {
		t.Fatalf("analyze video: %v", err)
	}
	t.Logf("video: %s (%d entries) dur=%v", col.Title, len(col.Entries), col.Entries[0].Duration)

	for _, o := range []downloader.Options{
		{Mode: "audio", AudioFormat: "mp3", Embed: true, SkipExisting: true},
		{Mode: "video", Quality: "480", Container: "mp4", Embed: true},
		{Mode: "video", Quality: "480", Container: "mkv", Embed: true},
	} {
		o.Normalize(downloader.SourceYouTube)
		sub := filepath.Join(dir, o.Mode+"_"+o.Ext())
		r := &rep{t: t}
		out, err := downloader.DownloadYouTube(ctx, e, col.Entries[0], sub, "01 - ", false, o, r)
		if err != nil {
			if ue, ok := err.(*queue.UserError); ok {
				t.Fatalf("download %s: %v\n%s", o.Ext(), err, ue.Detail)
			}
			t.Fatalf("download %s: %v", o.Ext(), err)
		}
		info, perr := ffmpeg.Probe(ctx, m.Path(tools.FFprobe), out)
		t.Logf("downloaded %s → %s (progress %.2f) video=%v %dx%d audio=%v cover=%d", o.Ext(), filepath.Base(out), r.last, info.HasVideo, info.Width, info.Height, info.HasAudio, info.CoverIndex)
		if perr != nil || filepath.Ext(out) != "."+o.Ext() {
			t.Fatalf("bad output %s %v", out, perr)
		}
		if o.Mode == "video" && (info.Height > 480 || !info.HasAudio) {
			t.Errorf("unexpected video output %+v", info)
		}
	}

	// Same audio again: the archive must make it a skip.
	o := downloader.Options{Mode: "audio", AudioFormat: "mp3", SkipExisting: true}
	o.Normalize(downloader.SourceYouTube)
	_, err = downloader.DownloadYouTube(ctx, e, col.Entries[0], filepath.Join(dir, "again"), "", false, o, &rep{t: t})
	if err == nil || err.Error() != "Sudah pernah diunduh" {
		t.Errorf("expected archive skip, got %v", err)
	}
	if !downloader.Archived(e.ArchivePath, "jNQXAC9IVRw") {
		t.Error("archive not written")
	}

	pl, err := downloader.AnalyzeYouTube(ctx, e, downloader.Detect("https://www.youtube.com/playlist?list=PLFs4vir_WsTwEd-nJgVJCZPNL3HALHHpF"))
	if err != nil {
		t.Logf("playlist analyze failed (network/YouTube): %v", err)
	} else {
		t.Logf("playlist %q by %q: %d entries, first=%q dur=%v", pl.Title, pl.Subtitle, len(pl.Entries), pl.Entries[0].Title, pl.Entries[0].Duration)
	}

	ch, err := downloader.AnalyzeYouTube(ctx, e, downloader.Detect("https://www.youtube.com/@jawed"))
	if err != nil {
		t.Errorf("channel analyze: %v", err)
	} else {
		t.Logf("channel %q: tabs=%v entries=%d first=%+v", ch.Title, ch.TabCounts, len(ch.Entries), ch.Entries[0])
	}
}

func TestSpotify(t *testing.T) {
	m := setup(t)
	e := env(m)
	ctx := context.Background()
	col, err := downloader.AnalyzeSpotify(ctx, e, downloader.Detect("https://open.spotify.com/album/4aawyAB9vmqN3uQ7FjRGTy"))
	if err != nil {
		if ue, ok := err.(*queue.UserError); ok {
			t.Fatalf("analyze: %v\n%s", err, ue.Detail)
		}
		t.Fatalf("analyze: %v", err)
	}
	t.Logf("album %q by %q: %d tracks, first=%+v", col.Title, col.Subtitle, len(col.Entries), col.Entries[0])
	o := downloader.Options{AudioFormat: "mp3", Numbering: true}
	o.Normalize(downloader.SourceSpotify)
	matcher := downloader.NewMatcher(e, []string{col.Entries[0].URL})
	dir := t.TempDir()
	name := downloader.SpotifyFileName(col.Entries[0], true, 2)
	r := &rep{t: t}
	out, err := downloader.DownloadSpotify(ctx, e, col.Entries[0], dir, name, o, matcher, r)
	if err != nil {
		if ue, ok := err.(*queue.UserError); ok {
			t.Fatalf("download: %v\n%s", err, ue.Detail)
		}
		t.Fatalf("download: %v", err)
	}
	info, _ := ffmpeg.Probe(ctx, m.Path(tools.FFprobe), out)
	t.Logf("spotify → %s audio=%v cover=%d dur=%.0f", filepath.Base(out), info.HasAudio, info.CoverIndex, info.Duration)
	if info.CoverIndex < 0 {
		t.Error("cover missing")
	}
}
