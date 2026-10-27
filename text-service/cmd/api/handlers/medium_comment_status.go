package handlers

import (
	"anahita-libs/data/entities"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EditCommentStatus opens or closes a medium to comments. It mirrors
// EditAccess: comment status is a single field with its own permission
// question, so it gets its own route rather than riding along on Edit, where a
// form that did not send it would silently close the medium to comments.
// Articles, notes and topics all embed mediumHandler, so this serves all three.
func (h *mediumHandler) EditCommentStatus(c echo.Context) error {
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
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// A reply has no switch for replies of its own to change: it follows the post at the
	// top of its thread.
	if medium.RootID > 0 {
		return c.JSON(http.StatusUnprocessableEntity, responses.Refusal{Error: "reply_follows_its_thread"})
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Medium.CanEdit(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	an_handlers.SetRepliesOpen(medium, req.Status)

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
