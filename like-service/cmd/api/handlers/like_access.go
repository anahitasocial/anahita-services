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

	// The viewer's relationship to the owner of the post the comment sits
	// under — the profile whose owner and administrators may delete it. It
	// was never set, so the delete flag in a like response always said no
	// to them. readLikeTarget loads the parent; without it the id is 0.
	var parentOwnerID int64
	if parent := node.GetParent(); parent != nil {
		parentOwnerID = parent.GetOwnerID()
	}

	actorIds := []int64{parentOwnerID, node.GetAuthorID()}
	relationships, err := h.Srv.SocialgraphGRPC.Browse(actorIds, viewer.ID)
	if err != nil {
		return nil, err
	}

	ctx.ParentOwnerViewerRel = relationships[parentOwnerID]
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
