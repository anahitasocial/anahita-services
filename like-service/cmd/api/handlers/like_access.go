package handlers

import (
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
)

// getCommentAccessContext retrieves the CommentAccessContext for the given CommentAccessContext
func (h *likeHandler) getCommentAccessContext(node *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.CommentAccessContext, error) {
	var (
		err error
		ctx *permissions.CommentAccessContext
	)

	ctx = &permissions.CommentAccessContext{
		Viewer:  viewer,
		Comment: node,
	}

	actorIds := []int64{node.GetOwnerID(), node.GetAuthorID()}
	relationships, err := h.Srv.SocialgraphGRPC.Browse(actorIds, viewer.ID)
	if err != nil {
		return nil, err
	}

	ctx.AuthorViewerRel = relationships[node.GetAuthorID()]

	return ctx, nil
}

// getMediumAccessContext retrieves the MediumAccessContext for the given CommentAccessContext
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
