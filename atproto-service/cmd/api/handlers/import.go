package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

// Following here whoever one follows on Bluesky already.
//
// Somebody who connects their account has, on Bluesky, a list of accounts
// they follow, and here follows none of them: their feed here begins
// empty. This reads that list and follows each of them here, once, when
// they ask for it. Nothing is written to Bluesky by it: every one of them
// is followed there already, which is what is found when the follow is
// about to be told (out.go).
//
// It is done after the answer, one account at a time: each is read, kept,
// and has its recent posts read, and a list of hundreds takes minutes.

const (
	// The most accounts followed in one go, and how many when no number
	// is asked for.
	importMost    = 500
	importDefault = 25
	// A pause between one and the next, so that the reading of each one's
	// posts is spread out.
	importPause = 300 * time.Millisecond
)

// How often somebody may ask for it.
var importLimit = ratelimit.Limit{Max: 3, Window: time.Hour}

const errImportNotConnected = "bluesky_not_connected"

// ImportPreview says what following them here would do, before it is
// done: how many accounts the viewer follows on Bluesky, and the most that
// are followed here in one go. Whoever asks is shown this first, since
// what it does is not undone in one step.
func (h *Handlers) ImportPreview(c echo.Context) error {
	viewer := getViewer(c)
	if !h.canWrite() || h.FollowEdges == nil || h.Relations == nil {
		return refuse(c, http.StatusNotFound, errConnectOff)
	}
	account := h.connected(viewer.ID)
	if account == nil {
		return refuse(c, http.StatusConflict, errImportNotConnected)
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*bsky.Timeout)
	defer cancel()

	profile, err := h.Bluesky.Profile(ctx, account.DID)
	if err != nil {
		log.Printf("how many %s follows on Bluesky: %v", account.Handle, err)
		return refuse(c, http.StatusBadGateway, errConnectFailed)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"following":     profile.FollowsCount,
		"most":          importMost,
		"posts_of_each": backfillPosts,
	})
}

type importRequest struct {
	// How many accounts to follow here, beginning with the ones followed
	// most lately.
	Limit int `json:"limit"`
}

// ImportFollows follows here the accounts the viewer follows on Bluesky.
func (h *Handlers) ImportFollows(c echo.Context) error {
	viewer := getViewer(c)
	if !h.canWrite() || h.FollowEdges == nil || h.Relations == nil {
		return refuse(c, http.StatusNotFound, errConnectOff)
	}
	account := h.connected(viewer.ID)
	if account == nil {
		return refuse(c, http.StatusConflict, errImportNotConnected)
	}
	if h.Limiter != nil && ratelimit.Exceeded(h.Limiter, "atproto_import", fmt.Sprint(viewer.ID), importLimit) {
		return refuse(c, http.StatusTooManyRequests, errConnectLimit)
	}

	req := &importRequest{}
	_ = c.Bind(req)
	limit := req.Limit
	if limit <= 0 {
		limit = importDefault
	}
	limit = min(limit, importMost)

	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*bsky.Timeout)
	defer cancel()

	// The first page is read now, to say whether there is anything to do.
	first, next, err := h.Bluesky.Following(ctx, account.DID, "")
	if err != nil {
		log.Printf("who %s follows on Bluesky: %v", account.Handle, err)
		return refuse(c, http.StatusBadGateway, errConnectFailed)
	}

	h.later(func() { h.importFollows(account, first, next, limit) })
	return c.JSON(http.StatusAccepted, map[string]any{"started": true, "limit": limit})
}

// importFollows follows here each account of the list, page by page, the
// ones followed most lately first, and no more than `most` of them.
func (h *Handlers) importFollows(account *repositories.Account, page []bsky.Profile, next string, most int) {
	person, err := h.Nodes.Read(account.PersonID)
	if err != nil || person == nil || person.ID == 0 {
		return
	}

	done, followed := 0, 0
	for {
		for i := range page {
			if done >= most {
				log.Printf("%s now follows %d more accounts of Bluesky here, of the first %d followed there", account.Handle, followed, done)
				return
			}
			done++
			if h.importFollow(person, &page[i]) {
				followed++
				if h.Settle > 0 {
					time.Sleep(importPause)
				}
			}
		}
		if next == "" {
			break
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*bsky.Timeout)
		page, next, err = h.Bluesky.Following(ctx, account.DID, next)
		cancel()
		if err != nil {
			log.Printf("who %s follows on Bluesky, further on: %v", account.Handle, err)
			break
		}
	}
	log.Printf("%s now follows %d more accounts of Bluesky here, of %d followed there", account.Handle, followed, done)
}

// importFollow follows one account here, and says whether it did.
func (h *Handlers) importFollow(person *entities.AnahitaNode, profile *bsky.Profile) bool {
	if !bsky.ValidDID(profile.DID) || h.refused(strings.ToLower(profile.Handle)) {
		return false
	}
	node, err := h.keep(profile)
	if err != nil || node == nil {
		// Somebody here who connected that account, or one that could
		// not be kept.
		return false
	}

	relation, err := h.Relations.Read(person.ID, node.ID)
	if err != nil || (relation != nil && (relation.IsFollowing || relation.IsBlocking)) {
		return false
	}

	viewer := entities.NewViewer()
	viewer.ID = person.ID
	add := services.NewSocialgraphFollowRequest().Add
	add.Source, add.Target, add.Viewer = person, node, viewer
	if err := h.FollowEdges.Add(add); err != nil {
		log.Printf("following %s here for %s: %v", node.Alias, person.Alias, err)
		return false
	}
	return true
}
