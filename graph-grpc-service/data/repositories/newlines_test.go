package repositories

import "testing"

func TestNewlines(t *testing.T) {
	cases := map[string]string{
		"one\r\ntwo\r\n\r\nthree": "one\ntwo\n\nthree",
		"old\rmac":                "old\nmac",
		"func() {\r\n\tx\r\n}":    "func() {\n\tx\n}",
		"already\nfine":           "already\nfine",
		"":                        "",
	}
	for in, want := range cases {
		if got := newlines(in); got != want {
			t.Errorf("newlines(%q) = %q, want %q", in, got, want)
		}
	}
}
