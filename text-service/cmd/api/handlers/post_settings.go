package handlers

import (
	"anahita-libs/data/entities"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/permissions"
	"anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// The settings of a post, each on a route of its own: who may see it, who
// may reply, who may quote it, its content warning and its pin. The same
// for every kind of post, in anahita-libs/handlers (PostSettings).

func (h *mediumHandler) settings() an_handlers.PostSettings {
	return an_handlers.PostSettings{
		Media:       h.Srv.MediaGRPC,
		Socialgraph: h.Srv.SocialgraphGRPC,
		Perms:       h.Perms.Medium,
		Thread:      h.readers(),
		Answer:      h.answerPost,
	}
}

func (h *mediumHandler) EditAccess(c echo.Context) error { return h.settings().EditAccess(c) }
func (h *mediumHandler) EditCommentStatus(c echo.Context) error {
	return h.settings().EditCommentStatus(c)
}
func (h *mediumHandler) EditReplyAccess(c echo.Context) error { return h.settings().EditReplyAccess(c) }
func (h *mediumHandler) EditQuotePolicy(c echo.Context) error { return h.settings().EditQuotePolicy(c) }
func (h *mediumHandler) EditWarning(c echo.Context) error     { return h.settings().EditWarning(c) }
func (h *mediumHandler) Pin(c echo.Context) error             { return h.settings().Pin(c) }
func (h *mediumHandler) Unpin(c echo.Context) error           { return h.settings().Unpin(c) }

// answerPost answers with a post after it was changed: what the viewer may
// do with it, and its pictures signed.
func (h *mediumHandler) answerPost(c echo.Context, post *entities.AnahitaNode) error {
	if err := h.onAfterFetched(c, post); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	batchPresignPaths(h.Env, []*entities.AnahitaNode{post})
	mediaResp := &responses.MediumNodeResponse{}
	return c.JSON(http.StatusOK, mediaResp.MapMediumNode(post))
}

// getMediumAccessContext is who the viewer is to a post's owner and author.
func (h *mediumHandler) getMediumAccessContext(post *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.MediumAccessContext, error) {
	return an_handlers.AccessContext(h.Srv.SocialgraphGRPC, post, viewer)
}
