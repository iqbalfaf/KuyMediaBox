package downloader

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type nopReporter struct{}

func (nopReporter) Progress(float64)        {}
func (nopReporter) Message(string)          {}
func (nopReporter) SetOutput(string, int64) {}

// KMB_LIVE=1: download "Me at the zoo" as MP3 128 kbps and WAV with the real tools.
func TestAudioDownloadLive(t *testing.T) {
	if os.Getenv("KMB_LIVE") == "" {
		t.Skip()
	}
	bin := filepath.Join(os.Getenv("LOCALAPPDATA"), "KuyMediaBox", "bin")
	node, _ := exec.LookPath("node")
	env := Env{YtDlp: filepath.Join(bin, "yt-dlp.exe"), FFmpeg: filepath.Join(bin, "ffmpeg.exe"), JSKind: "node", JSPath: node, TempDir: t.TempDir()}
	entry := Entry{URL: "https://www.youtube.com/watch?v=jNQXAC9IVRw"}
	for _, c := range []struct{ format, rate, want string }{{"mp3", "128", "128"}, {"wav", "auto", "pcm_s16le"}} {
		o := Options{Mode: "audio", AudioFormat: c.format, AudioQuality: c.rate, Embed: true}
		o.Normalize(SourceYouTube)
		out, err := DownloadYouTube(context.Background(), env, entry, t.TempDir(), "", false, o, nopReporter{})
		if err != nil {
			t.Fatalf("%s: %v", c.format, err)
		}
		probe, _ := exec.Command(filepath.Join(bin, "ffprobe.exe"), "-v", "error", "-select_streams", "a:0",
			"-show_entries", "stream=codec_name,bit_rate,sample_rate", "-show_entries", "format_tags=title", "-of", "compact", out).Output()
		t.Logf("%s → %s\n%s", c.format, filepath.Base(out), probe)
		if !strings.HasSuffix(out, "."+c.format) || !strings.Contains(string(probe), c.want) {
			t.Fatalf("%s: unexpected result %s", c.format, probe)
		}
	}
}
