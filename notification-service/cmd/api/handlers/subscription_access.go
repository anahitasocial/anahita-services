package handlers

import (
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
)

// getActorAccessContext retrieves the ActorAccessContext for the given Viewer
func (h *subscriptionHandler) getActorAccessContext(actor *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.ActorAccessContext, error) {
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

// getMediumAccessContext retrieves the MediumAccessContext for the given CommentAccessContext
func (h *subscriptionHandler) getMediumAccessContext(medium *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.MediumAccessContext, error) {
	var (
		err error
		ctx *permissions.MediumAccessContext
	)

	ctx = &permissions.MediumAccessContext{
		Viewer: viewer,
		Medium: medium,
	}

	actorIDs := []int64{medium.GetOwnerID(), medium.GetAuthorID()}
	relationships, err := h.Srv.SocialgraphGRPC.Browse(actorIDs, viewer.ID)
	if err != nil {
		return nil, err
	}

	ctx.OwnerViewerRel = relationships[medium.GetOwnerID()]
	ctx.AuthorViewerRel = relationships[medium.GetAuthorID()]

	return ctx, nil
}
