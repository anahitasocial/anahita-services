package handlers

import (
	"net/http"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"

	"github.com/labstack/echo/v4"
)

// ErrQuotePolicyInvalid is the error a request gets when the value it sends
// for who may quote is not one.
const ErrQuotePolicyInvalid = "quote_policy_invalid"

// ErrQuoteNotAllowed is the error a note gets when it may not quote what it
// asks to. It comes with a reason: see permissions.QuoteRefused*.
const ErrQuoteNotAllowed = "quote_not_allowed"

// RequestQuotePolicy reads who may quote from a request. Not given comes
// back as "" and true.
func RequestQuotePolicy(raw string) (string, bool) {
	return constants.NormalizeQuotePolicy(raw)
}

// RefuseQuotePolicy answers a request whose value for who may quote is not
// one: 400, saying which field.
func RefuseQuotePolicy(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, map[string]string{
		"error":       ErrQuotePolicyInvalid,
		"description": "quote_policy must be \"anyone\", \"followers\" or \"nobody\"",
	})
}

// RefuseQuote answers a note that may not quote what it asks to: 403, with
// why, so the app can say "this person does not allow quotes" and not just
// "no".
func RefuseQuote(c echo.Context, reason string) error {
	return c.JSON(http.StatusForbidden, map[string]string{
		"error":  ErrQuoteNotAllowed,
		"reason": reason,
	})
}

// QuoteReader is what SetQuotes reads the quoted posts with: a post, as a
// viewer, through the gate that opening it goes through.
type QuoteReader interface {
	Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error)
}

var _ QuoteReader = services.MediaGRPCInterface(nil)

// SetQuotes gives every note that quotes a post that post, as this viewer
// may see it, or the reason it is not shown.
//
// The quoted post is read AS THE VIEWER, through the same gate as opening it
// would go through. That is the whole of what keeps a quote from showing
// somebody a post they may not see: the note is theirs to read, and what it
// carries is asked about separately, for each reader, every time.
//
// One level only. A quoted note that itself quotes something is shown
// without what it quotes.
//
// It returns the quoted posts it attached, for the caller to make their
// images ready the way it does for its own.
func SetQuotes(media QuoteReader, nodes []*entities.AnahitaNode, viewer *entities.Viewer) []*entities.AnahitaNode {
	attached := []*entities.AnahitaNode{}
	read := map[int64]*entities.AnahitaNode{}
	gone := map[int64]bool{}

	for _, node := range nodes {
		if node == nil || node.QuoteID <= 0 {
			continue
		}

		// Taken out by its author. Not looked up at all.
		if node.QuoteDetached {
			node.QuoteState = constants.QuoteStateDetached
			continue
		}

		if gone[node.QuoteID] {
			node.QuoteState = constants.QuoteStateUnavailable
			continue
		}

		quoted, ok := read[node.QuoteID]
		if !ok {
			var err error
			quoted, err = media.Read(node.QuoteID, viewer)
			if err != nil || quoted == nil || quoted.DeletedAt != nil || quoted.RootID > 0 {
				gone[node.QuoteID] = true
				node.QuoteState = constants.QuoteStateUnavailable
				continue
			}
			read[node.QuoteID] = quoted
			attached = append(attached, quoted)
		}

		node.Quote = quoted
	}

	return attached
}
