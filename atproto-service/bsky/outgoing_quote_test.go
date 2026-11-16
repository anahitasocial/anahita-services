package bsky

import "testing"

// A post that quotes ends with "↪" and the quoted post's address, the
// address alone a link (roadmap row 156).
func TestAPostEndsWithWhatItQuotes(t *testing.T) {
	p := &OutPost{Text: "Look"}
	p.EndWithQuote("↪ ", "https://banu.io/photos/185877")
	if p.Text != "Look\n\n↪ banu.io/photos/185877" {
		t.Fatalf("text %q", p.Text)
	}
	if len(p.Facets) != 1 {
		t.Fatalf("facets %+v", p.Facets)
	}
	link := p.Facets[0]
	if got := p.Text[link.Index.ByteStart:link.Index.ByteEnd]; got != "banu.io/photos/185877" || link.Features[0]["uri"] != "https://banu.io/photos/185877" {
		t.Errorf("the link covers %q, to %q", got, link.Features[0]["uri"])
	}

	alone := &OutPost{}
	alone.EndWithQuote("↪ ", "https://banu.io/notes/1")
	if alone.Text != "↪ banu.io/notes/1" || alone.Text[alone.Facets[0].Index.ByteStart:] != "banu.io/notes/1" {
		t.Errorf("a quote with no words: %q %+v", alone.Text, alone.Facets)
	}
}
