package mediaconv

import (
	"strings"
	"testing"

	"kuymediabox/internal/ffmpeg"
)

func joined(a []string) string { return strings.Join(a, " ") }

func TestScaleFilter(t *testing.T) {
	cases := []struct {
		w, h, target int
		want         string
	}{
		{3840, 2160, 720, "scale=-2:720"},
		{1170, 2532, 720, "scale=720:-2"},
		{1280, 720, 720, ""},
		{640, 360, 720, ""},
		{1920, 1080, 0, ""},
	}
	for _, c := range cases {
		if got := ScaleFilter(c.w, c.h, c.target); got != c.want {
			t.Errorf("%dx%d→%d: got %q want %q", c.w, c.h, c.target, got, c.want)
		}
	}
}

func TestOutputSize(t *testing.T) {
	w, h := OutputSize(1170, 2532, VideoOptions{Format: "mp4", Codec: "h264", Resolution: "720"})
	if w != 720 || h != 1558 {
		t.Fatalf("got %dx%d", w, h)
	}
	if w, h := OutputSize(720, 1280, VideoOptions{Format: "mp4", Codec: "h264", Resolution: "480"}); w != 480 || h != 854 {
		t.Fatalf("got %dx%d", w, h)
	}
}

func TestVideoArgsH264(t *testing.T) {
	info := ffmpeg.Info{HasVideo: true, HasAudio: true, Width: 3840, Height: 2160, CoverIndex: -1}
	args, err := VideoArgs("in.mov", "out.mp4", info, VideoOptions{Format: "mp4", Codec: "h264", Quality: "seimbang", Resolution: "720"}, map[string]bool{"libx264": true})
	if err != nil {
		t.Fatal(err)
	}
	s := joined(args)
	for _, want := range []string{"-c:v libx264", "-crf 23", "-preset medium", "-vf scale=-2:720", "-c:a aac", "+faststart", "-map 0:a:0?"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	if args[len(args)-1] != "out.mp4" {
		t.Error("output must be last")
	}
}

func TestVideoArgsInvalidComboNormalized(t *testing.T) {
	info := ffmpeg.Info{HasVideo: true, Width: 1920, Height: 1080}
	args, err := VideoArgs("a.mp4", "b.webm", info, VideoOptions{Format: "webm", Codec: "h264"}, map[string]bool{"libvpx-vp9": true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(joined(args), "libvpx-vp9") {
		t.Fatalf("webm must switch to vp9: %s", joined(args))
	}
	if strings.Contains(joined(args), "-c:a") {
		t.Fatal("no audio stream, no audio codec expected")
	}
}

func TestVideoArgsCopyAndGif(t *testing.T) {
	info := ffmpeg.Info{HasVideo: true, HasAudio: true, Width: 1920, Height: 1080, VideoCodec: "h264", AudioCodec: "aac"}
	args, err := VideoArgs("a.mkv", "b.mp4", info, VideoOptions{Format: "mp4", Codec: "copy"}, nil)
	if err != nil || !strings.Contains(joined(args), "-c:v copy") || !strings.Contains(joined(args), "-c:a copy") || strings.Contains(joined(args), "-vf") {
		t.Fatalf("copy args: %s %v", joined(args), err)
	}
	if _, err := VideoArgs("a.mp4", "b.webm", info, VideoOptions{Format: "webm", Codec: "copy"}, nil); err == nil || !strings.Contains(err.Error(), "tidak bisa disalin") {
		t.Fatalf("h264 → webm copy must be refused with a clear message, got %v", err)
	}
	opusInfo := info
	opusInfo.AudioCodec = "opus"
	args, _ = VideoArgs("a.mkv", "b.avi", opusInfo, VideoOptions{Format: "avi", Codec: "copy"}, nil)
	if !strings.Contains(joined(args), "-c:v copy") || !strings.Contains(joined(args), "libmp3lame") {
		t.Fatalf("incompatible audio must be re-encoded while video is copied: %s", joined(args))
	}
	args, _ = VideoArgs("a.mkv", "b.gif", info, VideoOptions{Format: "gif"}, nil)
	s := joined(args)
	if !strings.Contains(s, "palettegen") || !strings.Contains(s, "scale=-2:480") {
		t.Fatalf("gif args: %s", s)
	}
}

func TestAV1Fallback(t *testing.T) {
	if e := PickEncoder("av1", map[string]bool{"libaom-av1": true}); e != "libaom-av1" {
		t.Fatalf("got %s", e)
	}
	if e := PickEncoder("av1", map[string]bool{}); e != "" {
		t.Fatalf("got %s", e)
	}
	info := ffmpeg.Info{HasVideo: true, Width: 100, Height: 100}
	if _, err := VideoArgs("a", "b.mp4", info, VideoOptions{Format: "mp4", Codec: "av1"}, map[string]bool{}); err == nil {
		t.Fatal("expected missing encoder error")
	}
}

func TestAudioArgs(t *testing.T) {
	flac := ffmpeg.Info{HasAudio: true, CoverIndex: 1, CoverCodec: "mjpeg", BitsPerSample: 24}
	args, err := AudioArgs("a.flac", "a.mp3", flac, AudioOptions{Format: "mp3", Bitrate: 320, KeepMetadata: true, Channels: "source", SampleRate: "source"})
	if err != nil {
		t.Fatal(err)
	}
	s := joined(args)
	for _, want := range []string{"-map 0:a:0", "-map 0:1", "attached_pic", "-b:a 320k", "libmp3lame", "-map_metadata 0", "-id3v2_version 3"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}

	args, _ = AudioArgs("a.flac", "a.opus", flac, AudioOptions{Format: "opus", Bitrate: 320, SampleRate: "44100", Channels: "mono"})
	s = joined(args)
	if !strings.Contains(s, "-b:a 256k") || strings.Contains(s, "-ar 44100") || strings.Contains(s, "attached_pic") || !strings.Contains(s, "-ac 1") {
		t.Errorf("opus args wrong: %s", s)
	}

	args, _ = AudioArgs("a.flac", "a.wav", flac, AudioOptions{Format: "wav"})
	if !strings.Contains(joined(args), "pcm_s24le") {
		t.Errorf("24-bit source should stay 24-bit: %s", joined(args))
	}

	video := ffmpeg.Info{HasVideo: true, HasAudio: true, CoverIndex: -1}
	args, _ = AudioArgs("v.mp4", "v.m4a", video, AudioOptions{Format: "m4a", Bitrate: 192, KeepMetadata: true})
	if !strings.Contains(joined(args), "-vn") {
		t.Errorf("video source must drop the video stream: %s", joined(args))
	}

	if _, err := AudioArgs("x", "y.mp3", ffmpeg.Info{HasVideo: true}, AudioOptions{}); err == nil {
		t.Error("expected error for file without audio")
	}
}
