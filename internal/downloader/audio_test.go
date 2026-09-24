package downloader

import (
	"slices"
	"strings"
	"testing"
)

func TestAudioOptions(t *testing.T) {
	o := Options{Mode: "audio", AudioFormat: "wav", AudioQuality: "320", Embed: true}
	o.Normalize(SourceYouTube)
	if o.AudioFormat != "wav" || o.AudioQuality != "auto" {
		t.Fatalf("wav must be lossless without bitrate: %+v", o)
	}
	args := strings.Join(Env{}.ytArgs(ytJob{URL: "u", Opts: o}), " ")
	if !strings.Contains(args, "--audio-format wav") || strings.Contains(args, "--embed-thumbnail") || !strings.Contains(args, "--embed-metadata") {
		t.Fatalf("wav args: %s", args)
	}

	o = Options{Mode: "audio", AudioFormat: "opus", AudioQuality: "128", Embed: true}
	o.Normalize(SourceYouTube)
	a := Env{}.ytArgs(ytJob{URL: "u", Opts: o})
	if i := slices.Index(a, "--audio-quality"); i < 0 || a[i+1] != "128K" || !slices.Contains(a, "--embed-thumbnail") {
		t.Fatalf("opus args: %v", a)
	}

	o = Options{Mode: "audio", AudioFormat: "mp3", AudioQuality: "999"}
	o.Normalize(SourceSpotify)
	if o.AudioQuality != "auto" {
		t.Fatalf("unknown bitrate must fall back to auto: %+v", o)
	}
	o = Options{AudioFormat: "wav"}
	o.Normalize(SourceSpotify)
	if o.AudioFormat != "wav" || o.Mode != "audio" {
		t.Fatalf("spotify wav: %+v", o)
	}
}
