package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/permissions"
)

// getActorAccessContext retrieves the ActorAccessContext for the given Viewer
func (h *adminHandler) getActorAccessContext(actor *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.ActorAccessContext, error) {
	var (
		err error
		ctx *permissions.ActorAccessContext
	)

	ctx = &permissions.ActorAccessContext{
		Viewer: viewer,
		Actor:  actor,
	}

	ctx.ActorViewerRel, err = h.Srv.SocialgraphGRPC.Read(actor.GetOwnerID(), viewer.GetID())
	if err != nil {
		return nil, err
	}

	return ctx, nil
}
