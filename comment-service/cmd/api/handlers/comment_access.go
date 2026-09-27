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

	// The viewer's relationship to the post's OWNER. It used to be keyed on
	// comment.ParentID — the post itself, which nobody follows or
	// administers — so it was always empty. Every handler now loads the
	// parent first (readParentWithOwner); without it the id is 0 and the
	// relationship is the empty one the permissions already treat as none.
	var ownerID int64
	if parent := comment.GetParent(); parent != nil {
		ownerID = parent.GetOwnerID()
	}

	actorIDs := []int64{ownerID, comment.CreatedBy}
	relationships, err := h.Srv.SocialgraphGRPC.Browse(actorIDs, viewer.ID)
	if err != nil {
		return nil, err
	}

	ctx.ParentOwnerViewerRel = relationships[ownerID]
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

// readParentWithOwner reads the post a comment sits under, with its owner read
// as an actor.
//
// Every comment permission is about the post and the profile it is on: who
// can see it, whose profile it is, who administers that profile, what the
// profile allows. A medium read carries only the owner's id, and only the
// actor read returns the owner's access and add_permissions with the
// actor_features defaults merged in.
func (h *commentHandler) readParentWithOwner(parentID int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	parent, err := h.Srv.MediaGRPC.Read(parentID, viewer)
	if err != nil {
		return nil, err
	}

	parent.Owner, err = h.Srv.ActorsGRPC.Read(parent.OwnerID, viewer)
	if err != nil {
		return nil, err
	}

	return parent, nil
}
