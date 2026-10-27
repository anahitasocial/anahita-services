package handlers

import (
	"anahita-libs/responses"
	"net/http"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/permissions"

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
	return c.JSON(http.StatusBadRequest, responses.Refusal{
		Error:       ErrQuotePolicyInvalid,
		Description: "quote_policy must be \"anyone\", \"followers\" or \"nobody\"",
	})
}

// RefuseQuote answers a note that may not quote what it asks to: 403, with
// why, so the app can say "this person does not allow quotes" and not just
// "no".
func RefuseQuote(c echo.Context, reason string) error {
	return c.JSON(http.StatusForbidden, responses.Refusal{
		Error:  ErrQuoteNotAllowed,
		Reason: reason,
	})
}

// QuoteReader reads a post for SetQuotes. It is NOT a check: it returns the
// post to whoever asks.
type QuoteReader interface {
	Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error)
}

var (
	_ QuoteReader                  = services.MediaGRPCInterface(nil)
	_ permissions.VisibilityFilter = services.GraphNodeGRPCInterface(nil)
)

// SetQuotes gives every note that quotes a post that post, as this viewer
// may see it, or the reason it is not shown.
//
// WHETHER THE VIEWER MAY SEE EACH QUOTED POST IS ASKED FIRST, of the filter,
// for all of them at once, and only the ones they may see are read. Reading
// a post does not check anything: the read hands it to whoever asks. The
// first version of this read and trusted the read, and would have shown a
// post made private after it was quoted, or a post by somebody who had
// blocked the reader, to anybody reading the note. The note is theirs to
// read; what it carries is asked about separately, for each reader, every
// time.
//
// If the filter cannot be asked, nothing is shown.
//
// One level only. A quoted note that itself quotes something is shown
// without what it quotes.
//
// It returns the quoted posts it attached, for the caller to make their
// images ready the way it does for its own.
func SetQuotes(media QuoteReader, filter permissions.VisibilityFilter, nodes []*entities.AnahitaNode, viewer *entities.Viewer) []*entities.AnahitaNode {
	attached := []*entities.AnahitaNode{}

	// The quoted posts to ask about: each once, and not one its author
	// took out of the quote, which is not looked up at all.
	ids := []int64{}
	asked := map[int64]bool{}
	for _, node := range nodes {
		if node == nil || node.QuoteID <= 0 {
			continue
		}
		if node.QuoteDetached {
			node.QuoteState = constants.QuoteStateDetached
			continue
		}
		if !asked[node.QuoteID] {
			asked[node.QuoteID] = true
			ids = append(ids, node.QuoteID)
		}
	}
	if len(ids) == 0 {
		return attached
	}

	seen := map[int64]bool{}
	if visible, err := filter.FilterVisible(ids, viewer); err == nil {
		for _, id := range visible {
			seen[id] = true
		}
	}

	read := map[int64]*entities.AnahitaNode{}
	for _, id := range ids {
		if !seen[id] {
			continue
		}
		quoted, err := media.Read(id, viewer)
		if err != nil || quoted == nil || quoted.DeletedAt != nil || quoted.RootID > 0 {
			continue
		}
		read[id] = quoted
		attached = append(attached, quoted)
	}

	for _, node := range nodes {
		if node == nil || node.QuoteID <= 0 || node.QuoteDetached {
			continue
		}
		if quoted, ok := read[node.QuoteID]; ok {
			node.Quote = quoted
		} else {
			node.QuoteState = constants.QuoteStateUnavailable
		}
	}

	return attached
}
