package handlers

import (
	"anahita-libs/cmd/api/handlers"
	"anahita-libs/data/entities"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"
	"video/constants"

	"github.com/labstack/echo/v4"
)

// EditReplyAccess changes who may reply to a video post: anyone, nobody, or any of
// the people who follow its author, the people its author follows, and the
// people it mentions. Whoever may edit the post may change it.
//
// PATCH /videos/:id/reply-access
//
// A route of its own, as the audience and the older switch for replies have,
// and for the same reason: an edit form that did not send it must not change
// it.
func (h *videoHandler) EditReplyAccess(c echo.Context) error {
	var (
		req    = &requests.MediumEditReplyAccessRequest{}
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

	replyAccess, valid := handlers.RequestReplyAccess(req.ReplyAccess)
	if !valid || replyAccess == "" {
		return handlers.RefuseReplyAccess(c)
	}

	medium, err := h.Srv.MediaGRPC.Read(req.ID, viewer)
	if err != nil || medium == nil || medium.Type != constants.NodeMediumVideoType {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
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

	handlers.SetReplyAccess(medium, replyAccess)
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
