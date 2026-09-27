package permissions

import (
	"anahita-libs/cmd/api/permissions"
	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"errors"
	"fmt"
)

const ErrInsufficientScope = "insufficient scope"

type LikePermissionsInterface interface {
	CanBrowse(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error)
	CanAdd(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error)
	CanDelete(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error)
}

type LikePermissions struct {
	*Env
}

func NewLikePermissions(env *Env) LikePermissionsInterface {
	return &LikePermissions{
		Env: env,
	}
}

func (p *LikePermissions) CanBrowse(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error) {
	// Step 1: Super admins can always browse
	if viewer.IsAdmin() {
		return true, nil
	}

	// Case A: The node is a comment
	if node.IsComment() {
		parent := node.GetParent()

		commentParentCtx := &permissions.MediumAccessContext{
			Viewer: viewer,
			Medium: parent,
		}

		actorIds := []int64{parent.GetOwnerID(), parent.GetAuthorID()}
		relationships, err := p.Srv.SocialgraphGRPC.Browse(actorIds, viewer.ID)
		if err != nil {
			return false, err
		}

		commentParentCtx.OwnerViewerRel = relationships[parent.GetOwnerID()]
		commentParentCtx.AuthorViewerRel = relationships[parent.GetAuthorID()]

		// Step 2A: Ensure viewer can read the parent node
		mediumPerm := permissions.NewMediumPermissions()
		ok, err := mediumPerm.CanRead(commentParentCtx)
		if err != nil || !ok {
			return false, fmt.Errorf("viewer %d cannot read parent %d", viewer.ID, parent.GetID())
		}

		return true, nil
	}

	// Case B: The node is a medium
	if node.IsMedium() {
		mediumAccessCtx := &permissions.MediumAccessContext{
			Viewer: viewer,
			Medium: node,
		}

		relationships, err := p.Srv.SocialgraphGRPC.Browse([]int64{node.GetOwnerID(), node.GetAuthorID()}, viewer.ID)
		if err != nil {
			return false, err
		}

		mediumAccessCtx.OwnerViewerRel = relationships[node.GetOwnerID()]
		mediumAccessCtx.AuthorViewerRel = relationships[node.GetAuthorID()]

		// Step 2B: Ensure viewer can read the medium node
		mediumPerm := permissions.NewMediumPermissions()
		ok, err := mediumPerm.CanRead(mediumAccessCtx)
		if err != nil || !ok {
			return false, fmt.Errorf("viewer %d cannot read medium %d", viewer.ID, node.GetID())
		}

		return true, nil
	}

	// Unsupported node type
	return false, errors.New("you are not allowed to browse this content")
}

func (p *LikePermissions) CanAdd(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error) {
	var err error

	// Before the admin short-circuit: a role never bypasses the scope
	// gate, or a token deliberately narrowed to exclude likes would
	// still be able to like as soon as its holder is an admin.
	if !viewer.HasScope(libs_constants.OAuthScopeLikesWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	if viewer.IsAdmin() {
		return true, nil
	}

	// Helper
	hasAccess := func(owner *entities.AnahitaNode, service string) bool {
		ownerVsViewerRel, err := p.Srv.SocialgraphGRPC.Read(owner.GetID(), viewer.ID)
		if err != nil {
			return false
		}

		// The owner can always like on their own profile.
		if owner.GetID() == viewer.ID {
			return true
		}

		return permissions.AddAccessAllows(owner.AddPermissionAccess(service, "like"), ownerVsViewerRel, viewer)
	}

	if node.IsComment() {
		parent := node.GetParent()
		owner := parent.GetOwner()

		// The post the comment sits under is what has to be readable — the
		// error below already says "cannot read parent". The comment itself
		// carries neither the post's owner nor its access level.
		mediumAccessCtx := &permissions.MediumAccessContext{
			Viewer: viewer,
			Medium: parent,
		}

		relationships, err := p.Srv.SocialgraphGRPC.Browse([]int64{owner.GetID(), parent.GetAuthorID()}, viewer.ID)
		if err != nil {
			return false, err
		}

		mediumAccessCtx.OwnerViewerRel = relationships[owner.GetID()]
		mediumAccessCtx.AuthorViewerRel = relationships[parent.GetAuthorID()]

		ok, err := permissions.NewMediumPermissions().CanRead(mediumAccessCtx)
		if err != nil || !ok {
			return false, fmt.Errorf("viewer %d cannot read parent %d", viewer.ID, parent.GetID())
		}

		service := parent.Service // e.g., "photo-service", "article-service"
		if hasAccess(owner, service) {
			return true, nil
		}

		return false, fmt.Errorf("viewer %d cannot like comment under parent %d", viewer.ID, parent.GetID())
	}

	if node.IsMedium() {
		owner := node.GetOwner()
		mediumAccessCtx := &permissions.MediumAccessContext{
			Viewer: viewer,
			Medium: node,
		}

		mediumAccessCtx.OwnerViewerRel, err = p.Srv.SocialgraphGRPC.Read(owner.GetID(), viewer.ID)
		if err != nil {
			return false, err
		}

		ok, err := permissions.NewMediumPermissions().CanRead(mediumAccessCtx)
		if err != nil || !ok {
			return false, fmt.Errorf("viewer %d cannot read medium %d", viewer.ID, node.GetID())
		}

		service := node.Service
		if hasAccess(owner, service) {
			return true, nil
		}

		return false, fmt.Errorf("viewer %d cannot like medium %d", viewer.ID, node.GetID())
	}

	return false, errors.New("you are not allowed to like this content")
}

func (p *LikePermissions) CanDelete(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeLikesWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	if node.IsLikedByViewer {
		return true, nil
	}

	return false, fmt.Errorf("viewer %d has not liked the node %d", viewer.ID, node.GetID())
}
