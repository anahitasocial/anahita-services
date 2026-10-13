package federation

import "testing"

func TestAServersNameAsItIsKept(t *testing.T) {
	for typed, want := range map[string]string{
		"Mastodon.Example":                 "mastodon.example",
		" mastodon.example. ":              "mastodon.example",
		"https://mastodon.example/users/a": "mastodon.example",
		"http://mastodon.example:8080/x":   "mastodon.example",
		"@alice@mastodon.example":          "mastodon.example",
		"alice@mastodon.example":           "mastodon.example",
		"":                                 "",
	} {
		if got := CleanDomain(typed); got != want {
			t.Errorf("%q: %q, want %q", typed, got, want)
		}
	}
}
