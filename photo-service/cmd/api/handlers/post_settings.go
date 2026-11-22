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

func (h *photoHandler) settings() an_handlers.PostSettings {
	return an_handlers.PostSettings{
		Media:       h.Srv.MediaGRPC,
		Socialgraph: h.Srv.SocialgraphGRPC,
		Perms:       h.Perms.Media,
		Thread:      h.readers(),
		Answer:      h.answerPost,
	}
}

func (h *photoHandler) EditAccess(c echo.Context) error { return h.settings().EditAccess(c) }
func (h *photoHandler) EditCommentStatus(c echo.Context) error {
	return h.settings().EditCommentStatus(c)
}
func (h *photoHandler) EditReplyAccess(c echo.Context) error { return h.settings().EditReplyAccess(c) }
func (h *photoHandler) EditQuotePolicy(c echo.Context) error { return h.settings().EditQuotePolicy(c) }
func (h *photoHandler) EditWarning(c echo.Context) error     { return h.settings().EditWarning(c) }
func (h *photoHandler) Pin(c echo.Context) error             { return h.settings().Pin(c) }
func (h *photoHandler) Unpin(c echo.Context) error           { return h.settings().Unpin(c) }

// answerPost answers with a post after it was changed: what the viewer may
// do with it, and its pictures signed.
func (h *photoHandler) answerPost(c echo.Context, post *entities.AnahitaNode) error {
	if err := h.onAfterFetched(c, post); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	batchPresignPaths(h.Env, []*entities.AnahitaNode{post})
	mediaResp := &responses.MediumNodeResponse{}
	return c.JSON(http.StatusOK, mediaResp.MapMediumNode(post))
}

// getMediumAccessContext is who the viewer is to a post's owner and author.
func (h *photoHandler) getMediumAccessContext(post *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.MediumAccessContext, error) {
	return an_handlers.AccessContext(h.Srv.SocialgraphGRPC, post, viewer)
}
