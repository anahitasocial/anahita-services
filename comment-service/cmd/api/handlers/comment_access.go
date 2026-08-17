package handlers

import (
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
)

// getCommentAccessContext retrieves the CommentAccessContext for the given Viewer
func (h *commentHandler) getCommentAccessContext(comment *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.CommentAccessContext, error) {
	var (
		err error
		ctx *permissions.CommentAccessContext
	)

	ctx = &permissions.CommentAccessContext{
		Viewer:  viewer,
		Comment: comment,
	}

	actorIDs := []int64{comment.ParentID, comment.CreatedBy}
	relationships, err := h.Srv.SocialgraphGRPC.Browse(actorIDs, viewer.ID)
	if err != nil {
		return nil, err
	}

	ctx.ParentOwnerViewerRel = relationships[comment.ParentID]
	ctx.AuthorViewerRel = relationships[comment.CreatedBy]

	return ctx, nil
}

func (h *commentHandler) getCommentParentAccessContext(medium *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.MediumAccessContext, error) {
	var (
		err error
		ctx *permissions.MediumAccessContext
	)

	ctx = &permissions.MediumAccessContext{
		Viewer: viewer,
		Medium: medium,
	}

	ctx.OwnerViewerRel, err = h.Srv.SocialgraphGRPC.Read(ctx.Medium.GetOwnerID(), viewer.ID)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}
