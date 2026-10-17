package bsky

import (
	"strings"
	"testing"
	"time"
)

func TestWhatIsWrittenHereAsABlueskyPost(t *testing.T) {
	at := time.Date(2026, 10, 13, 6, 0, 0, 0, time.UTC)
	post := NewOutPost("Café **news**: [the story](https://example.com/a?b=1) and https://example.org/x. See #garden, not a#b\n\n> quoted\\_words", "en", at)

	want := "Café news: the story and https://example.org/x. See #garden, not a#b\n\nquoted_words"
	if post.Text != want {
		t.Fatalf("text %q\nwant %q", post.Text, want)
	}
	if post.Type != "app.bsky.feed.post" || post.CreatedAt != "2026-10-13T06:00:00.000Z" || len(post.Langs) != 1 || post.Langs[0] != "en" {
		t.Errorf("%+v", post)
	}

	// Each stretch is where its words are, counted in bytes.
	marked := map[string]string{}
	for _, facet := range post.Facets {
		words := post.Text[facet.Index.ByteStart:facet.Index.ByteEnd]
		for _, feature := range facet.Features {
			marked[words] = feature["uri"] + feature["tag"]
		}
	}
	for words, is := range map[string]string{
		"the story":             "https://example.com/a?b=1",
		"https://example.org/x": "https://example.org/x",
		"#garden":               "garden",
	} {
		if marked[words] != is {
			t.Errorf("%q marked as %q, want %q (%v)", words, marked[words], is, marked)
		}
	}
	if len(post.Facets) != 3 {
		t.Errorf("%d stretches marked", len(post.Facets))
	}
}

func TestOnlyTenHashtagsAreMarked(t *testing.T) {
	post := NewOutPost("#a1 #a2 #a3 #a4 #a5 #a6 #a7 #a8 #a9 #a10 #A1 #a11", "", time.Now())

	if post.Text != "#a1 #a2 #a3 #a4 #a5 #a6 #a7 #a8 #a9 #a10 #A1 #a11" {
		t.Fatalf("text %q", post.Text)
	}
	// #A1 is #a1 again, and marked; #a11 is the eleventh, and a word.
	if len(post.Facets) != 11 {
		t.Errorf("%d marked", len(post.Facets))
	}
	last := post.Facets[len(post.Facets)-1]
	if words := post.Text[last.Index.ByteStart:last.Index.ByteEnd]; words != "#A1" {
		t.Errorf("last marked %q", words)
	}
}

func TestAPostTooLongIsCut(t *testing.T) {
	long := strings.Repeat("é", 290) + " [a link at the end](https://example.com/far)"
	post := NewOutPost(long, "und", time.Now())

	if got := len([]rune(post.Text)); got != PostMax || !strings.HasSuffix(post.Text, "…") {
		t.Errorf("%d letters, ending %q", got, post.Text[len(post.Text)-8:])
	}
	// The link the cut ran through is words, and no language is claimed
	// that is not known.
	if len(post.Facets) != 0 || post.Langs != nil {
		t.Errorf("facets %v, langs %v", post.Facets, post.Langs)
	}

	short := NewOutPost("[all of it](https://example.com)", "", time.Now())
	if len(short.Facets) != 1 || short.Text != "all of it" {
		t.Errorf("%+v", short)
	}
}

func TestNamesOfRecordsSortAsTheyWereMade(t *testing.T) {
	before := ""
	for i := 0; i < 200; i++ {
		name := NewTID()
		if len(name) != 13 || !ValidRkey(name) || name <= before {
			t.Fatalf("%q after %q", name, before)
		}
		before = name
	}
}
