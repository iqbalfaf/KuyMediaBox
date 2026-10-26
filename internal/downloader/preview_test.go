package downloader

import (
	"encoding/json"
	"testing"
)

func TestPickStream(t *testing.T) {
	var single streamInfo
	_ = json.Unmarshal([]byte(`{"url":"https://cdn/v.mp4","protocol":"https","ext":"mp4","vcodec":"avc1","acodec":"mp4a",
		"http_headers":{"User-Agent":"UA","Referer":"https://site/"},"cookies":"sid=abc; Domain=.site; Path=/; Secure; lang=id; Expires=1"}`), &single)
	s, err := pickStream(single)
	if err != nil || s.URL != "https://cdn/v.mp4" || !s.Video || !s.Audio || !s.Playable() {
		t.Fatalf("single: %+v, %v", s, err)
	}
	if s.Headers["Referer"] != "https://site/" || s.Headers["Cookie"] != "sid=abc; lang=id" {
		t.Errorf("headers: %v", s.Headers)
	}

	var merged streamInfo
	_ = json.Unmarshal([]byte(`{"http_headers":{"User-Agent":"UA"},"requested_formats":[
		{"url":"https://a/audio.m4a","protocol":"https","ext":"m4a","vcodec":"none","acodec":"mp4a"},
		{"url":"https://a/video.webm","protocol":"https","ext":"webm","vcodec":"vp9","acodec":"none"}]}`), &merged)
	s, err = pickStream(merged)
	if err != nil || s.URL != "https://a/video.webm" || s.Audio || s.Headers["User-Agent"] != "UA" {
		t.Fatalf("merged: %+v, %v", s, err)
	}

	hls := Stream{URL: "https://a/x.m3u8", Protocol: "m3u8_native", Ext: "mp4"}
	if hls.Playable() {
		t.Error("HLS can't be played by the webview; it gets frames")
	}
	if _, err := pickStream(streamInfo{}); err == nil {
		t.Error("no url must fail")
	}
	for _, u := range []string{"file:///C:/Windows/win.ini", `C:\secret.mp4`, "concat:a|b"} {
		if _, err := pickStream(streamInfo{URL: u, VCodec: "avc1"}); err == nil {
			t.Errorf("%s must be refused", u)
		}
	}
}
