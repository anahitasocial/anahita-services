package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"syndication/data/repositories"
	"syndication/websub"

	"github.com/labstack/echo/v4"
)

// Where a WebSub hub calls back for a website: to check that a subscription
// was asked for, and to say that its feed changed. See package websub. Not
// signed in: a hub has no account here. The address's signature is what
// says the call is about this website.
type websubHandler struct {
	*Env
}

// WebSubRequest is a hub's call.
type WebSubRequest struct {
	// The website's id, and the signature that came with it.
	ID        int64  `param:"id"`
	Signature string `param:"signature"`
	// What the hub checks: `subscribe`, `unsubscribe`, or `denied`.
	Mode string `query:"hub.mode"`
	// What to answer with, to show the subscription was asked for here.
	Challenge string `query:"hub.challenge"`
	// How long, in seconds, the hub will call back.
	LeaseSeconds int64 `query:"hub.lease_seconds"`
}

func (h *websubHandler) websiteOf(c echo.Context, req *WebSubRequest) (string, int) {
	// Read from the address alone: what a hub sends with a change is the
	// feed, in whatever it is written in, and is not looked at.
	req.ID, _ = strconv.ParseInt(c.Param("id"), 10, 64)
	req.Signature = c.Param("signature")
	req.Mode = c.QueryParam("hub.mode")
	req.Challenge = c.QueryParam("hub.challenge")
	req.LeaseSeconds, _ = strconv.ParseInt(c.QueryParam("hub.lease_seconds"), 10, 64)
	if req.ID <= 0 {
		return "", http.StatusNotFound
	}
	if !websub.Signed(req.ID, h.Config.GetSecureCookieKey(), req.Signature) {
		return "", http.StatusNotFound
	}
	_, hubURL, err := h.Srv.Websites.Feed(c.Request().Context(), req.ID)
	if errors.Is(err, repositories.ErrNotFound) || (err == nil && hubURL == "") {
		return "", http.StatusNotFound
	}
	if err != nil {
		log.Println(err)
		return "", http.StatusInternalServerError
	}
	return hubURL, 0
}

// Verify answers a hub checking a subscription.
//
// GET /websites/_websub/:id/:signature
func (h *websubHandler) Verify(c echo.Context) error {
	req := &WebSubRequest{}
	if _, status := h.websiteOf(c, req); status != 0 {
		return c.NoContent(status)
	}

	switch req.Mode {
	case "subscribe":
		if req.Challenge == "" {
			return c.NoContent(http.StatusBadRequest)
		}
		lease := time.Duration(req.LeaseSeconds) * time.Second
		if lease <= 0 || lease > websub.Lease {
			lease = websub.Lease
		}
		if err := h.Srv.Websites.HubVerified(c.Request().Context(), req.ID, time.Now().Add(lease)); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		return c.String(http.StatusOK, req.Challenge)
	case "unsubscribe":
		return c.String(http.StatusOK, req.Challenge)
	case "denied":
		// The hub will not: the feed is read every half hour as before.
		return c.NoContent(http.StatusOK)
	}
	return c.NoContent(http.StatusBadRequest)
}

// Notify takes a hub's word that the feed changed: it is read now, from the
// site, as any time it is read. What the hub sends is not read.
//
// POST /websites/_websub/:id/:signature
func (h *websubHandler) Notify(c echo.Context) error {
	req := &WebSubRequest{}
	if _, status := h.websiteOf(c, req); status != 0 {
		return c.NoContent(status)
	}
	if err := h.Srv.Websites.ReadSoon(c.Request().Context(), req.ID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusAccepted)
}
