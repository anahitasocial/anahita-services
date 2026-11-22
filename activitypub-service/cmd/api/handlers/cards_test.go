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

// THE RULE THIS GUARDS (roadmap row 170): the server that sent a post is
// answered before its link's card is read, which can take seconds; the
// card is added after, and not to a post changed since.
func TestAPostIsKeptBeforeItsCard(t *testing.T) {
	w, _ := settledWorld()
	w.h.Cards = &fakeCards{}
	var later []func()
	w.h.Later = func(work func()) { later = append(later, work) }

	w.fromAlice(received(alicesNote, `<p>Bumble <a href=\"https://www.bbc.com/news/1\">bbc.com/news/1</a></p>`, ""))
	kept, _ := w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == alicesNote })
	if kept == nil || linkpreviews.FromMeta(kept.Meta) != nil {
		t.Fatalf("kept with its card, or not kept: %+v", kept)
	}
	for _, work := range later {
		work()
	}
	kept, _ = w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == alicesNote })
	if card := linkpreviews.FromMeta(kept.Meta); card == nil || card.Title != "Bumble shares soar" {
		t.Errorf("no card after: %+v", card)
	}

	// Changed before its card was read: the card of what it was is not
	// put on what it is.
	other := "https://mastodon.example/users/alice/statuses/6"
	later = nil
	w.fromAlice(received(other, `<p>Bumble <a href=\"https://www.bbc.com/news/1\">bbc.com/news/1</a></p>`, ""))
	for _, node := range w.nodes.people {
		if node.URI == other {
			node.Body = "Changed there"
		}
	}
	for _, work := range later {
		work()
	}
	changed, _ := w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == other })
	if linkpreviews.FromMeta(changed.Meta) != nil {
		t.Error("the card of what it was was put on what it is")
	}
}
