package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"activitypub/remote"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

// Somebody from another server, on a page here (roadmap row 134): given
// their handle, the page on their own server where they follow a profile
// from here, or answer, like or repost a post from here. Asked by the web
// app for a visitor who is not signed in, so it needs no token.

// Why there is nowhere to send them.
const (
	// The handle is not one: @name@server.
	errIntentHandle = "intent_handle"
	// What they would act on is not on the fediverse.
	errIntentNotOut = "intent_not_out"
	// Their server publishes no such page, does not answer, or is one this
	// site refuses: they are told to search on it for the address instead.
	errIntentNone = "intent_none"
	// Too many asked from one address.
	errIntentLimit = "too_many_intents"
)

var intentLimit = ratelimit.Limit{Max: 30, Window: 10 * time.Minute}

// IntentRequest is a visitor's handle and what they would act on.
type IntentRequest struct {
	// Their handle, @name@server, the first @ or not.
	Handle string `query:"handle"`
	// The profile or the post here.
	NodeID string `query:"node_id"`
}

// Intent is where they are sent, or, when there is nowhere, the address
// to search for on their server.
type Intent struct {
	// Their server's own page for it.
	URL string `json:"url,omitempty"`
	// What they would act on, as other servers know it: to search for when
	// there is no page to send them to.
	Address string `json:"address"`
	// Why there is no page, when there is none.
	Error string `json:"error,omitempty"`
}

// Intent answers GET /activitypub/intent?handle=…&node_id=….
func (h *Handlers) Intent(c echo.Context) error {
	if ok, reason := h.siteFederates(); !ok {
		return refuse(c, http.StatusConflict, reason)
	}
	if h.Limiter != nil && ratelimit.Exceeded(h.Limiter, "activitypub_intent", c.RealIP(), intentLimit) {
		return refuse(c, http.StatusTooManyRequests, errIntentLimit)
	}

	id, _ := strconv.ParseInt(c.QueryParam("node_id"), 10, 64)
	address, follow := h.actedOn(id)
	if address == "" {
		return refuse(c, http.StatusNotFound, errIntentNotOut)
	}

	handle := strings.TrimPrefix(strings.TrimSpace(c.QueryParam("handle")), "@")
	mentions := entities.ParseMentions("@" + handle)
	if handle == "" || len(mentions) != 1 || mentions[0].Host == "" || mentions[0].End != len(handle)+1 {
		return refuse(c, http.StatusBadRequest, errIntentHandle)
	}
	if mentions[0].IsLocal(h.Site.Domain) {
		return refuse(c, http.StatusBadRequest, errIntentHandle)
	}

	none := &Intent{Address: address, Error: errIntentNone}
	// A server this site refuses is answered as one that does not answer.
	if h.Blocks != nil && h.Blocks.IsBlocked(mentions[0].Host) {
		return c.JSON(http.StatusOK, none)
	}
	if h.Fetcher == nil {
		return c.JSON(http.StatusOK, none)
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()
	page, err := remote.Intent(ctx, h.Fetcher, mentions[0].User, mentions[0].Host, address, follow)
	if err != nil {
		log.Printf("activitypub intent: %s: %v", mentions[0].Host, err)
	}
	if page == "" {
		return c.JSON(http.StatusOK, none)
	}
	return c.JSON(http.StatusOK, &Intent{URL: page, Address: address})
}

// actedOn is the address other servers know a profile or a post here by,
// and whether it is followed rather than answered: only something that is
// on the fediverse, a profile that chose it, a post that went out.
func (h *Handlers) actedOn(id int64) (string, bool) {
	if id <= 0 || h.Nodes == nil {
		return "", false
	}
	node, err := h.Nodes.Read(id)
	if err != nil || node == nil || node.ID == 0 || node.IsRemote() {
		return "", false
	}
	if isFederatingType(node.Type) {
		if h.federating(node) == nil || node.UUID == "" {
			return "", false
		}
		return h.addresses.Actor(node.UUID), true
	}
	if strings.HasPrefix(node.Type, constants.NodeMediumPrefixEntityType) {
		if author, _ := h.goesOut(node); author == nil {
			return "", false
		}
		return h.addresses.Note(node.UUID), false
	}
	return "", false
}
