package handlers

import (
	"context"
	"testing"

	"anahita-libs/data/entities"
	"anahita-libs/linkpreviews"
)

type fakeCards struct{ asked []string }

func (f *fakeCards) Preview(_ context.Context, address string) *linkpreviews.Preview {
	f.asked = append(f.asked, address)
	if address != "https://www.bbc.com/news/1" {
		return nil
	}
	return &linkpreviews.Preview{URL: address, Title: "Bumble shares soar", SiteName: "BBC News", ImageKey: "abc.jpg", Status: linkpreviews.StatusOK}
}

func TestTheLinkACardIsOf(t *testing.T) {
	for body, want := range map[string]string{
		"Read [this](https://www.bbc.com/news/1) now":                                                "https://www.bbc.com/news/1",
		"[@alice@mastodon.example](https://mastodon.example/@alice) see https://www.bbc.com/news/1.": "https://www.bbc.com/news/1",
		"[#news](https://mastodon.example/tags/news) and nothing else":                               "",
		"#beans @ana no link here":                                                                   "",
		"[https://www.bbc.com/news/1](https://www.bbc.com/news/1)":                                   "https://www.bbc.com/news/1",
	} {
		if got := cardLink(body); got != want {
			t.Errorf("%q: %q, want %q", body, got, want)
		}
	}
}

// A post from elsewhere with a link in it is kept with the card this
// installation made of the page, and one without is kept without.
func TestAPostFromElsewhereIsGivenItsCard(t *testing.T) {
	w, _ := settledWorld()
	cards := &fakeCards{}
	w.h.Cards = cards

	w.fromAlice(received(alicesNote, `<p>Bumble <a href=\"https://www.bbc.com/news/1\">bbc.com/news/1</a></p>`, ""))
	kept, _ := w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == alicesNote })
	if kept == nil {
		t.Fatal("her note was not kept")
	}
	card := linkpreviews.FromMeta(kept.Meta)
	if card == nil || card.Title != "Bumble shares soar" || card.ImageKey != "abc.jpg" {
		t.Errorf("kept with %+v (asked %v)", card, cards.asked)
	}

	other := "https://mastodon.example/users/alice/statuses/5"
	w.fromAlice(received(other, `<p>No link at all</p>`, ""))
	plain, _ := w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == other })
	if plain == nil || linkpreviews.FromMeta(plain.Meta) != nil {
		t.Errorf("a post with no link: %+v", plain)
	}
}
