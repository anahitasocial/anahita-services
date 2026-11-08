package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"anahita-libs/ratelimit"

	"activitypub/fetch"
	"activitypub/remote"

	"github.com/labstack/echo/v4"
)

// How long what another server said of somebody is believed before it is
// asked again, when their profile is opened here: an hour (the user,
// 2026-11-07), so that a change there shows almost as soon as anybody
// looks. At most one fetch an hour for a profile, and only when it is
// opened, which costs their server nothing beside what its followers'
// servers ask.
const refreshAfter = time.Hour

// Refreshed is whether somebody's profile was read again from their server.
type Refreshed struct {
	// True when it was, so that what was shown is to be read again here.
	Refreshed bool `json:"refreshed"`
}

// RefreshPerson reads somebody on another server again, when what was kept
// of them is more than an hour old: opening their profile asks for it, so
// that whether they approve their followers, their name and their words
// are as their server says now (roadmap row 130). Nobody has to know to
// look them up again. Nothing is said of a failure: what was kept stands.
func (h *Handlers) RefreshPerson(c echo.Context) error {
	viewer := getViewer(c)

	if ok, reason := h.siteFederates(); !ok {
		return refuse(c, http.StatusConflict, reason)
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return refuse(c, http.StatusNotFound, errProfileNotKept)
	}
	kept, err := h.Actors.ReadByNode(id)
	if err != nil || kept == nil {
		return refuse(c, http.StatusNotFound, errProfileNotKept)
	}

	if time.Since(kept.FetchedAt) < refreshAfter {
		return c.JSON(http.StatusOK, Refreshed{})
	}
	if h.Limiter != nil && ratelimit.Exceeded(h.Limiter, "activitypub_refresh", fmt.Sprint(viewer.ID), lookupLimit) {
		return c.JSON(http.StatusOK, Refreshed{})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*fetch.Timeout)
	defer cancel()
	if err := h.refreshed(ctx, &remote.Actor{ID: kept.URI}); err != nil {
		log.Println("activitypub refresh:", kept.URI, ":", err)
		return c.JSON(http.StatusOK, Refreshed{})
	}
	return c.JSON(http.StatusOK, Refreshed{Refreshed: true})
}
