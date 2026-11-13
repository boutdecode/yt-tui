package yt

import "testing"

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"2026.07.04", "2026.08.01", true},
		{"2026.08.01", "2026.07.04", false},
		{"2026.07.04", "2026.07.04", false},
		{"2026.07.04", "2026.07.04.1", true},
		{"2025.12.31", "2026.01.01", true},
	}
	for _, c := range cases {
		if got := versionLess(c.a, c.b); got != c.want {
			t.Errorf("versionLess(%q,%q)=%v", c.a, c.b, got)
		}
	}
}
