package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/permissions"
)

// getMediumAccessContext retrieves the MediumAccessContext for a post
func (h *likeHandler) getMediumAccessContext(medium *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.MediumAccessContext, error) {
	var (
		err error
		ctx *permissions.MediumAccessContext
	)

	ctx = &permissions.MediumAccessContext{
		Viewer: viewer,
		Medium: medium,
	}

	actorIds := []int64{medium.GetOwnerID(), medium.GetAuthorID()}
	relationships, err := h.Srv.SocialgraphGRPC.Browse(actorIds, viewer.ID)
	if err != nil {
		return nil, err
	}

	ctx.OwnerViewerRel = relationships[medium.GetOwnerID()]
	ctx.AuthorViewerRel = relationships[medium.GetAuthorID()]

	return ctx, nil
}
