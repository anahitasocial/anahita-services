package permissions

import (
	"anahita-libs/data/entities"
	"errors"
)

const (
	ErrCommentInsufficientScope = "insufficient scope"
	ErrCommentEditNotAllowed    = "you are not allowed to edit this comment"
	ErrCommentAddNotAllowed     = "you are not allowed to add a comment"
	ErrCommentDeleteNotAllowed  = "you are not allowed to delete this comment"
	ErrCommentAdminNotAllowed   = "you are not allowed to administrate this comment"
)

// CommentParentNodeInterface
type CommentParentNodeInterface interface {
	GetID() int64
	GetAuthorID() int64
	GetOwnerID() int64
	GetAccess() string
	GetOwner() *entities.AnahitaNode
	GetAuthor() *entities.AnahitaNode
	GetService() string
	GetEntityName() string
}

// CommentPermissionsInterface
type CommentPermissionsInterface interface {
	CanBrowse(ctx *MediumAccessContext) (bool, error)
	CanRead(ctx *CommentAccessContext) (bool, error)
	CanEdit(ctx *CommentAccessContext) (bool, error)
	CanAdd(ctx *CommentAccessContext) (bool, error)
	CanDelete(ctx *CommentAccessContext) (bool, error)
	CanAdministrate(ctx *CommentAccessContext) (bool, error)
}

// CommentPermissions
type CommentPermissions struct{}

// NewCommentPermissions creates a new CommentPermissions service
func NewCommentPermissions() CommentPermissionsInterface {
	return &CommentPermissions{}
}

// CanBrowse checks if the viewer can browse the comments
// Comments are visible if the parent media is readable
func (c *CommentPermissions) CanBrowse(ctx *MediumAccessContext) (bool, error) {
	mediumPerm := NewMediumPermissions()
	ok, err := mediumPerm.CanRead(ctx)
	if ok {
		return true, nil
	}

	return false, err
}

// CanRead checks if the viewer can read the comment
// Comments are readable if the parent media is readable
func (c *CommentPermissions) CanRead(ctx *CommentAccessContext) (bool, error) {
	mediumPerm := NewMediumPermissions()
	mediumPermCtx := &MediumAccessContext{
		Viewer:         ctx.Viewer,
		Medium:         ctx.Comment.GetParent(),
		OwnerViewerRel: ctx.ParentOwnerViewerRel,
	}
	ok, err := mediumPerm.CanRead(mediumPermCtx)
	if ok {
		return true, nil
	}

	return false, err
}

// CanEdit checks if the viewer can edit the comment
func (c *CommentPermissions) CanEdit(ctx *CommentAccessContext) (bool, error) {
	if !ctx.Viewer.RequireScope("write:media") {
		return false, errors.New(ErrCommentInsufficientScope)
	}

	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	if ctx.Viewer.ID == ctx.Comment.GetParent().GetOwnerID() {
		return true, nil
	}

	if ctx.ParentOwnerViewerRel.HasAdmin {
		return true, nil
	}

	mediumPerm := NewMediumPermissions()
	mediumPermCtx := &MediumAccessContext{
		Viewer:         ctx.Viewer,
		Medium:         ctx.Comment.GetParent(),
		OwnerViewerRel: ctx.ParentOwnerViewerRel,
	}
	ok, err := mediumPerm.CanRead(mediumPermCtx)
	if ok {
		if ctx.Viewer.ID == ctx.Comment.GetAuthorID() {
			return true, nil
		}
	}

	if err != nil {
		return false, err
	}

	return false, errors.New(ErrCommentEditNotAllowed)
}

// CanAdd checks if the viewer can add a comment
func (c *CommentPermissions) CanAdd(ctx *CommentAccessContext) (bool, error) {
	if !ctx.Viewer.RequireScope("write:media") {
		return false, errors.New(ErrCommentInsufficientScope)
	}

	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	parent := ctx.Comment.GetParent()
	parentOwner := parent.GetOwner()

	if ctx.Viewer.ID == parentOwner.GetID() {
		return true, nil
	}

	if ctx.ParentOwnerViewerRel.HasAdmin {
		return true, nil
	}

	service := parent.Service

	switch {
	case parentOwner.HasAddPermission(service, "comment", "admins") && ctx.ParentOwnerViewerRel.HasAdmin:
		return true, nil

	case parentOwner.HasAddPermission(service, "comment", "followers") &&
		(ctx.ParentOwnerViewerRel.IsLeading || ctx.ParentOwnerViewerRel.HasAdmin):
		return true, nil

	case parentOwner.HasAddPermission(service, "comment", "registered") && ctx.Viewer.IsRegistered():
		return true, nil
	}

	mediumPermCtx := &MediumAccessContext{
		Viewer:         ctx.Viewer,
		Medium:         ctx.Comment.GetParent(),
		OwnerViewerRel: ctx.ParentOwnerViewerRel,
	}
	if ok, _ := NewMediumPermissions().CanRead(mediumPermCtx); ok {
		return true, nil
	}

	return false, errors.New(ErrCommentAddNotAllowed)
}

// CanDelete checks if the viewer can delete the comment
func (c *CommentPermissions) CanDelete(ctx *CommentAccessContext) (bool, error) {
	if !ctx.Viewer.RequireScope("write:media") {
		return false, errors.New(ErrCommentInsufficientScope)
	}

	ok, _ := c.CanEdit(ctx)
	if ok {
		return true, nil
	}

	return false, errors.New(ErrCommentDeleteNotAllowed)
}

// CanAdministrate checks if the viewer can administrate the comment
func (c *CommentPermissions) CanAdministrate(ctx *CommentAccessContext) (bool, error) {
	if !ctx.Viewer.RequireScope("admin:media") {
		return false, errors.New(ErrCommentInsufficientScope)
	}

	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	if ctx.Viewer.ID == ctx.Comment.GetParent().GetOwnerID() {
		return true, nil
	}

	if ctx.ParentOwnerViewerRel.HasAdmin {
		return true, nil
	}

	mediumPerm := NewMediumPermissions()
	mediumPermCtx := &MediumAccessContext{
		Viewer:         ctx.Viewer,
		Medium:         ctx.Comment.GetParent(),
		OwnerViewerRel: ctx.ParentOwnerViewerRel,
	}
	ok, err := mediumPerm.CanRead(mediumPermCtx)
	if ok {
		if ctx.Viewer.ID == ctx.Comment.GetAuthorID() {
			return true, nil
		}
	}

	if err != nil {
		return false, err
	}

	return false, errors.New(ErrCommentAdminNotAllowed)
}
