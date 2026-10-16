package handlers

import (
	"anahita-libs/responses"
	"log"
	"net/http"
	"time"

	"recommendation/data/repositories"

	"github.com/labstack/echo/v4"
)

// The hashtags somebody who has just joined is offered to pick from, in the
// Interests step of signing up: the ones on the most public posts lately.
// What is picked is followed as feeds, and those feeds are what this
// service then recommends by (the followed-hashtags signal).

// How far back is looked, in turn, until enough hashtags are found: the
// last three months, then the year, then all of it. A site that has been
// quiet lately has hashtags still worth offering, and old and useful
// counts as much as new.
var interestsWindows = []time.Duration{
	90 * 24 * time.Hour,
	365 * 24 * time.Hour,
	0, // all time
}

const (
	// Fewer than this found, and the next window is tried. The web app
	// leaves the step out with fewer than three.
	interestsEnough = 8
	// A hashtag on fewer posts than this is not offered: one post is not
	// an interest anybody shares.
	interestsFewestPosts = 2
	interestsDefault     = 20
	interestsMost        = 40
)

type interestsHandler struct {
	*Env
}

// InterestsRequest asks for the hashtags offered when signing up.
type InterestsRequest struct {
	// The most to send: 20 unless asked, 40 at most.
	Limit int `query:"limit" validate:"omitempty,min=1,max=40"`
}

// Browse lists them, the most used first.
func (h *interestsHandler) Browse(c echo.Context) error {
	if !signedIn(getViewer(c)) {
		return c.NoContent(http.StatusForbidden)
	}

	req := &InterestsRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	limit := req.Limit
	if limit == 0 {
		limit = interestsDefault
	}
	if limit > interestsMost {
		limit = interestsMost
	}

	var found []repositories.Interest
	for _, window := range interestsWindows {
		since := time.Time{}
		if window > 0 {
			since = time.Now().Add(-window)
		}
		var err error
		found, err = h.Repos.Signals.Interests(since, interestsFewestPosts, limit)
		if err != nil {
			log.Println("recommendations: interests:", err)
			return c.NoContent(http.StatusInternalServerError)
		}
		if len(found) >= interestsEnough || len(found) >= limit {
			break
		}
	}
	if found == nil {
		found = []repositories.Interest{}
	}
	return c.JSON(http.StatusOK, responses.Unpaged[repositories.Interest]{Data: found})
}
