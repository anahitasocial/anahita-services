package responses

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// QuoteEmbed is what a note that quotes a post carries about it: the post,
// when this viewer may see it, or why it is not shown.
type QuoteEmbed struct {
	// "unavailable" when the viewer may not see the quoted post or it is
	// gone; "detached" when its author took it out of this quote. Left out
	// when the post is there.
	State string `json:"state,omitempty"`
	// The quoted post, with its author, the profile it is on and its date.
	Post *Parent `json:"post,omitempty"`
}

// NewQuoteEmbed is nil for a note that quotes nothing.
//
// It is nil too for a note whose handler did not look the quoted post up:
// the post is never sent on the strength of the note alone, and saying
// "unavailable" would be a guess. A response like that, the one a like
// answers with for instance, is laid over what the app already has, and
// must not wipe out a quote that was looked up properly a moment before.
func NewQuoteEmbed(node *entities.AnahitaNode) *QuoteEmbed {
	if node == nil || node.QuoteID <= 0 {
		return nil
	}

	if node.QuoteDetached {
		return &QuoteEmbed{State: constants.QuoteStateDetached}
	}

	if node.QuoteState != "" {
		return &QuoteEmbed{State: node.QuoteState}
	}

	if node.Quote != nil {
		return &QuoteEmbed{Post: NewRoot(node.Quote)}
	}

	return nil
}
