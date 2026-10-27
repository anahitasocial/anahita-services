package handlers

import (
	"fmt"

	"anahita-libs/data/entities"
	"anahita-libs/handlers"
	"anahita-libs/permissions"

	"github.com/labstack/echo/v4"
)

// A video post can be a reply, a GIF made into a clip among them: begun as
// an upload with a parent (see video_uploads.go) and made by the worker. A
// reply is judged by the rule for its thread, as a note reply is in
// text-service and a photo reply in photo-service.

// readers are what a thread is read with.
func (h *videoHandler) readers() handlers.ThreadReaders {
	return handlers.ThreadReaders{Media: h.Srv.MediaGRPC, Actors: h.Srv.ActorsGRPC, Socialgraph: h.Srv.SocialgraphGRPC}
}

// mayRead, mayEdit and mayDelete judge a video post by the rule for video
// posts, and one that is a reply by the rule for its thread.
func (h *videoHandler) mayRead(c echo.Context, medium *entities.AnahitaNode, viewer *entities.Viewer, ctx *permissions.MediumAccessContext) (bool, error) {
	if medium.RootID > 0 {
		return h.judgedByThread(c, medium, viewer, permissions.CanSeeThread)
	}
	return h.Perms.Media.CanRead(ctx)
}

func (h *videoHandler) mayEdit(c echo.Context, medium *entities.AnahitaNode, viewer *entities.Viewer, ctx *permissions.MediumAccessContext) (bool, error) {
	if medium.RootID > 0 {
		return h.judgedByThread(c, medium, viewer, permissions.CanEditReply)
	}
	return h.Perms.Media.CanEdit(ctx)
}

func (h *videoHandler) mayDelete(c echo.Context, medium *entities.AnahitaNode, viewer *entities.Viewer, ctx *permissions.MediumAccessContext) (bool, error) {
	if medium.RootID > 0 {
		return h.judgedByThread(c, medium, viewer, permissions.CanDeleteReply)
	}
	return h.Perms.Media.CanDelete(ctx)
}

func (h *videoHandler) judgedByThread(c echo.Context, reply *entities.AnahitaNode, viewer *entities.Viewer, may func(*permissions.ReplyAccessContext) bool) (bool, error) {
	if _, status := handlers.ReplyRuling(c, h.readers(), reply, viewer, may); status != 0 {
		return false, fmt.Errorf("reply %d: refused by its thread (%d)", reply.ID, status)
	}
	return true, nil
}
