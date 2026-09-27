package main

import "testing"

func TestLinksFromArgs(t *testing.T) {
	got := linksFromArgs([]string{
		"C:\file.mp4",
		"kuymediabox://download?url=https%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3DdQw4w9WgXcQ%26t%3D5",
		"kuymediabox:https://soundcloud.com/forss/flickermood",
		"kuymediabox://download?url=javascript%3Aalert(1)",
		"KUYMEDIABOX://x?url=http://example.com/a",
	})
	want := []string{"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=5", "https://soundcloud.com/forss/flickermood", "http://example.com/a"}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: %q, want %q", i, got[i], want[i])
		}
	}
}
