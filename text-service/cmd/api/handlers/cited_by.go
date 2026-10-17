package handlers

import (
	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/constants"
	"anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// CitedByRequest is a page of the posts that cite a post.
type CitedByRequest struct {
	// The post cited, of any kind.
	ID int64 `param:"id"`
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send: up to 100.
	Limit uint64 `query:"limit" validate:"omitempty,max=100"`
}

// CitedBy returns the posts that cite a post, newest first: the articles
// and topics that show it within their text. Served under /notes/ for a
// post of any kind, a photo included, as its quotes are.
//
// GET /notes/:id/cited_by
//
// For anybody who can see the post. Each citing post is listed only to
// somebody who may see it, so the list can be shorter than the post's
// citation count; and its author's own are listed, though they are not
// counted.
func (h *mediumHandler) CitedBy(c echo.Context) error {
	var (
		req    = &CitedByRequest{Limit: constants.RequestLimitDefault}
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	id := req.ID
	if id <= 0 {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	// A post the viewer cannot see has no list for them.
	if !an_handlers.MaySee(h.Srv.GraphNodeGRPC, id, viewer) {
		return c.NoContent(http.StatusNotFound)
	}

	resp := &responses.List[*responses.MediumNodeItem]{Data: []*responses.MediumNodeItem{}}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	media, total, err := h.Srv.MediaGRPC.CitedBy(id, req.Limit, req.Offset, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total

	for _, medium := range media {
		if err = h.onAfterFetched(c, medium); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	batchPresignPaths(h.Env, media)

	mediaResp := &responses.MediumNodeResponse{}
	resp.Data = mediaResp.MapMediumNodeItems(media)

	return c.JSON(http.StatusOK, resp)
}
