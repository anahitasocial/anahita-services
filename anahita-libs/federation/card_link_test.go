package federation

import "testing"

// The link a post's card is of: the first that is not a mention or a
// hashtag, written as a link or bare. Moved here from activitypub-service,
// whose wrapper of it went (roadmap row 170).
func TestTheLinkACardIsOf(t *testing.T) {
	for body, want := range map[string]string{
		"Read [this](https://www.bbc.com/news/1) now":                                                "https://www.bbc.com/news/1",
		"[@alice@mastodon.example](https://mastodon.example/@alice) see https://www.bbc.com/news/1.": "https://www.bbc.com/news/1",
		"[#news](https://mastodon.example/tags/news) and nothing else":                               "",
		"#beans @ana no link here":                                                                   "",
		"[https://www.bbc.com/news/1](https://www.bbc.com/news/1)":                                   "https://www.bbc.com/news/1",
	} {
		if got := CardLink(body); got != want {
			t.Errorf("%q: %q, want %q", body, got, want)
		}
	}
}
