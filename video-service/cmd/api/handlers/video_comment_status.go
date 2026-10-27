package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/handlers"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"
	"video/constants"

	"github.com/labstack/echo/v4"
)

// EditCommentStatus opens or closes a video to comments. It mirrors
// EditAccess: comment status is a single field with its own permission
// question, so it gets its own route rather than riding along on Edit, where a
// form that did not send it would silently close the medium to comments.
func (h *videoHandler) EditCommentStatus(c echo.Context) error {
	var (
		req    = requests.NewMediumRequest().EditCommentStatus
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
	if err != nil || medium == nil || medium.Type != constants.NodeMediumVideoType {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// A reply has none of its own: its thread decides.
	if medium.RootID > 0 {
		return c.JSON(http.StatusUnprocessableEntity, responses.Refusal{Error: "reply_follows_its_thread"})
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Media.CanEdit(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	handlers.SetRepliesOpen(medium, req.Status)

	// Changing comment status is a modification, and the edit validator
	// requires a modifier. See person_access.go for what its absence did.
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
	resp := mediaResp.MapMediumNode(medium)

	return c.JSON(http.StatusOK, resp)
}
