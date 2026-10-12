package store

import (
	"testing"
	"unicode/utf8"
)

func TestClipKeepsCharactersWhole(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"short", 10, "short"},
		{"abcdef", 3, "abc"},
		{"ééé", 3, "é"},
		{"a\xffb", 10, "ab"},
	}
	for _, c := range cases {
		got := clip(c.in, c.n)
		if got != c.want || !utf8.ValidString(got) {
			t.Errorf("clip(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}
