package permissions

import (
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"errors"
	"fmt"
)

type repostPermissions struct {
	*Env
}

func NewRepostPermissions(env *Env) *repostPermissions {
	return &repostPermissions{
		Env: env,
	}
}

// CanBrowse
func (p *repostPermissions) CanBrowse(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error) {
	// Step 1: Super admins can always browse
	if viewer.IsAdmin() {
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

	return false, errors.New("you are not allowed to browse this content")
}

// CanAdd
func (p *repostPermissions) CanAdd(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error) {
	var err error

	if viewer.IsAdmin() {
		return true, nil
	}

	// Helper
	hasAccess := func(owner *entities.AnahitaNode, service string) bool {
		ownerVsViewerRel, err := p.Srv.SocialgraphGRPC.Read(owner.GetID(), viewer.ID)
		if err != nil {
			return false
		}

		switch {
		case owner.HasAddPermission(service, "repost", "admins") && ownerVsViewerRel.HasAdmin:
			return true
		case owner.HasAddPermission(service, "repost", "followers") &&
			(ownerVsViewerRel.IsLeading || ownerVsViewerRel.HasAdmin):
			return true
		case owner.HasAddPermission(service, "repost", "registered") && viewer.IsRegistered():
			return true
		default:
			return false
		}
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
		if hasAccess(owner, service) || true {
			return true, nil
		}

		return false, fmt.Errorf("viewer %d cannot repost medium %d", viewer.ID, node.GetID())
	}

	return false, errors.New("you are not allowed to repost this content")
}

// CanDelete
func (p *repostPermissions) CanDelete(viewer *entities.Viewer, repost *entities.AnahitaNode) (bool, error) {
	if viewer.IsAdmin() {
		return true, nil
	}

	if repost.Author.ID == viewer.ID {
		return true, nil
	}

	return false, errors.New("you are not allowed to delete this repost")
}

// CanAdministrate returns true if the viewer is allowed to administrate the repost
func (p *repostPermissions) CanAdministrate(viewer *entities.Viewer, entity interface{}) (bool, error) {
	return true, errors.New("you are not allowed to administrate this repost")
}
