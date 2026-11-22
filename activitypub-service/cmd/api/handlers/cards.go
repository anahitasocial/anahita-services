package handlers

import (
	"log"

	"anahita-libs/data/entities"
	"anahita-libs/federation"
	"anahita-libs/videos"
)

// The card for a link in a post from another server: made as it is for a
// post from any protocol, in anahita-libs/federation.

// Cards finds the preview of a link, or nil when there is nothing to show.
type Cards = federation.Cards

// NewCards is what asks link-preview-service at `base`.
func NewCards(base string) Cards { return federation.NewCards(base) }

// withCard gives a post from elsewhere the card of the link in it.
func (h *Handlers) withCard(body string) []byte {
	return federation.CardMeta(h.Cards, body, "")
}

// cardsAtOnce is how many links' pages are read at once: a burst of posts
// from elsewhere waits its turn, and does not ask link-preview-service for
// hundreds of pages together.
var cardsAtOnce = make(chan struct{}, 4)

// addCard gives a post kept from elsewhere the card of the link in it,
// once it is kept and the server that sent it answered. Only the post as
// it was kept: one changed since is given its own card by its change.
func (h *Handlers) addCard(saved, node *entities.AnahitaNode, words string, elsewhere *videos.Elsewhere) {
	cardsAtOnce <- struct{}{}
	card := h.withCard(words)
	<-cardsAtOnce
	if card == nil {
		return
	}
	now, err := h.Nodes.ReadByURI(node.URI)
	if err != nil || now == nil || now.ID != saved.ID || now.Body != saved.Body {
		return
	}
	node.Meta = card
	if elsewhere != nil {
		node.Meta = videos.WithElsewhere(card, elsewhere)
	}
	if _, _, err := h.Nodes.UpsertRemote(node); err != nil {
		log.Printf("%s: its link's card: %v", node.URI, err)
	}
}
