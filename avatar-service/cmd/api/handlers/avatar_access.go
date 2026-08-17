package handlers

import (
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
)

// getActorAccessContext retrieves the ActorAccessContext for the given Viewer
func (h *avatarHandler) getActorAccessContext(actor *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.ActorAccessContext, error) {
	var (
		err error
		ctx *permissions.ActorAccessContext
	)

	ctx = &permissions.ActorAccessContext{
		Viewer: viewer,
		Actor:  actor,
	}

	ctx.ActorViewerRel, err = h.Srv.SocialgraphGRPC.Read(actor.ID, viewer.ID)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}
