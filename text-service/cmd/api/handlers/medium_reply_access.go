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

// EditReplyAccess changes who may reply to an article, a note or a topic: anyone, nobody, or any of
// the people who follow its author, the people its author follows, and the
// people it mentions. Whoever may edit the post may change it.
//
// PATCH /{articles,notes,topics}/:id/reply-access
//
// A route of its own, as the audience and the older switch for replies have,
// and for the same reason: an edit form that did not send it must not change
// it.
func (h *mediumHandler) EditReplyAccess(c echo.Context) error {
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

	replyAccess, valid := an_handlers.RequestReplyAccess(req.ReplyAccess)
	if !valid || replyAccess == "" {
		return an_handlers.RefuseReplyAccess(c)
	}

	medium, err := h.Srv.MediaGRPC.Read(req.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// A reply has no setting of its own: it follows the post at the top of
	// its thread.
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

	an_handlers.SetReplyAccess(medium, replyAccess)
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
