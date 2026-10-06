package handlers

import (
	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/constants"
	"anahita-libs/responses"
	"log"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// Saved posts: a person's own list of posts to find again.
//
// PRIVATE. Every route here is about the person asking and nobody else:
// there is no way to ask what somebody else saved, or who saved a post, and
// the author of a post is not told that it was. Nothing is counted on the
// post. Served under /bookmarks/, for posts of any kind, a photo included,
// as replies are served under /notes/ whatever they answer.

type savedBrowseRequest struct {
	Offset uint64 `query:"start"`
	Limit  uint64 `query:"limit" validate:"omitempty,max=100"`
}

// Saved returns what the viewer saved, most recently saved first.
//
// GET /bookmarks/
//
// As the viewer may see each post now. One saved while it was public and
// since made private, or whose author has since blocked them, is left out:
// saving a post does not keep it open. It comes back if it is opened again.
func (h *mediumHandler) Saved(c echo.Context) error {
	var (
		req    = &savedBrowseRequest{Limit: constants.RequestLimitDefault}
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	type Response struct {
		Data       []*responses.MediumNodeItem `json:"data"`
		Pagination responses.Pagination        `json:"pagination"`
	}
	resp := &Response{Data: []*responses.MediumNodeItem{}}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	media, total, err := h.Srv.MediaGRPC.Saved(req.Limit, req.Offset, viewer)
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

// Save saves a post for the viewer, and Unsave takes it out of what they
// saved.
//
// PUT and DELETE /bookmarks/:id
//
// Saving needs the post to be one the viewer may see. Taking one out does
// not: somebody can always clear out their own list, whatever has become of
// the post since.
func (h *mediumHandler) Save(c echo.Context) error {
	return h.setSaved(c, true)
}

func (h *mediumHandler) Unsave(c echo.Context) error {
	return h.setSaved(c, false)
}

func (h *mediumHandler) setSaved(c echo.Context, saved bool) error {
	viewer := getViewer(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	// Asked of the gate, not of the read, which hands a post to anybody.
	if saved && !an_handlers.MaySee(h.Srv.GraphNodeGRPC, id, viewer) {
		return c.NoContent(http.StatusNotFound)
	}

	if err := h.Srv.MediaGRPC.SetSaved(id, saved, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"id":                 id,
		"is_saved_by_viewer": saved,
	})
}
