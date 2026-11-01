package handlers

import (
	"log"
	"net/http"
	"strconv"

	"anahita-libs/responses"

	"activitypub/data/repositories"

	"github.com/labstack/echo/v4"
)

// BrowseKnownProfiles lists who from other networks is kept here, newest
// first, a page at a time, for administrators to see them and refuse any of
// them. Nobody else: these people did not ask to be listed on this site.
func (h *Handlers) BrowseKnownProfiles(c echo.Context) error {
	if !getViewer(c).IsAdmin() {
		return c.NoContent(http.StatusForbidden)
	}

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit < 1 || limit > blocksPageMax {
		limit = blocksPage
	}
	offset, _ := strconv.Atoi(c.QueryParam("start"))
	if offset < 0 {
		offset = 0
	}

	profiles, total, err := h.KnownProfiles.Browse(c.QueryParam("q"), limit, offset)
	if err != nil {
		log.Println("known profiles:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	return c.JSON(http.StatusOK, responses.List[*repositories.KnownProfile]{
		Data: profiles,
		Pagination: responses.Pagination{
			Total:  uint64(total),
			Limit:  uint64(limit),
			Offset: uint64(offset),
		},
	})
}
