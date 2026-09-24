package tools

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"2026.09.20", "2026.08.30", true},
		{"2026.08.30", "2026.08.30", false},
		{"v4.2.11", "4.2.10", true},
		{"4.2.10", "4.2.11", false},
		{"2026.09.20", "", false},
		{"7.1", "7.1.1", false},
	}
	for _, c := range cases {
		if got := newer(c.a, c.b); got != c.want {
			t.Errorf("newer(%q,%q)=%v", c.a, c.b, got)
		}
	}
}
