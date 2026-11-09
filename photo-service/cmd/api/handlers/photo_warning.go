package handlers

import (
	"log"
	"net/http"

	"anahita-libs/data/entities"
	"anahita-libs/handlers"
	"anahita-libs/requests"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// EditWarning gives a photo post a content warning, changes it, or takes it
// off (anahita-services row 122). Whoever may edit the post may; a reply is
// changed by its author alone. A warning a moderator gave stays as it is:
// its author is answered `warning_not_allowed`.
//
// PATCH /photos/:id/warning
func (h *photoHandler) EditWarning(c echo.Context) error {
	var (
		req    = &requests.MediumEditWarningRequest{}
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

	medium, err := h.Srv.MediaGRPC.Read(req.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.mayEdit(c, medium, viewer, mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	valid, allowed := handlers.SetWarning(medium, req.Warning, req.WarningText)
	if !valid {
		return handlers.RefuseWarning(c)
	}
	if !allowed {
		return handlers.RefuseWarningNotAllowed(c)
	}
	medium.ModifiedBy = viewer.ID

	if medium, err = h.Srv.MediaGRPC.Edit(medium, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err = h.onAfterFetched(c, medium); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{medium})

	mediaResp := &responses.MediumNodeResponse{}

	return c.JSON(http.StatusOK, mediaResp.MapMediumNode(medium))
}
