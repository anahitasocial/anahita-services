package handlers

import "anahita-libs/federation"

// The card for a link in a post from another server: made as it is for a
// post from any protocol, in anahita-libs/federation.

// Cards finds the preview of a link, or nil when there is nothing to show.
type Cards = federation.Cards

// NewCards is what asks link-preview-service at `base`.
func NewCards(base string) Cards { return federation.NewCards(base) }

func cardLink(markdown string) string { return federation.CardLink(markdown) }

// withCard gives a post from elsewhere the card of the link in it.
func (h *Handlers) withCard(body string) []byte {
	return federation.CardMeta(h.Cards, body, "")
}
