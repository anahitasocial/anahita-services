package permissions

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"errors"
	"fmt"
)

const (
	ErrMediumInsufficientScope = "insufficient scope"
)

type MediumPermissionsInterface interface {
	CanBrowse(viewer *entities.Viewer) (bool, error)
	CanRead(ctx *MediumAccessContext) (bool, error)
	CanEdit(ctx *MediumAccessContext) (bool, error)
	CanAdd(ctx *MediumAccessContext) (bool, error)
	CanDelete(ctx *MediumAccessContext) (bool, error)
	CanAdministrate(ctx *MediumAccessContext) (bool, error)
}

type MediumPermissions struct{}

func NewMediumPermissions() MediumPermissionsInterface {
	return &MediumPermissions{}
}

func (m *MediumPermissions) CanBrowse(viewer *entities.Viewer) (bool, error) {
	return true, nil
}

// CanRead returns true if the viewer is allowed to read the medium node
//
// TRUE CONDITIONS:
// 1. If the viewer is an admin or super admin
// 2. If the viewer is the same as the medium owner
// 3. If the viewer is the same as the medium author
// 4. If the medium access is public
// 5. If the medium access is registered and the viewer is logged in
// 6. If the medium access is followers and the viewer is following the owner, leading the owner, or is mutual with the owner, or admin of the owner
// 7. If the medium access is leaders and the viewer is leading the owner, is mutual with the owner, or admin of the owner
// 8. If the medium access is mutuals and the viewer is mutual with the owner, or admin of the owner
// 9. If the medium access is admins and the viewer is admin of the owner
// 10. If the medium access is myself and the viewer is the same as the medium owner
func (m *MediumPermissions) CanRead(ctx *MediumAccessContext) (bool, error) {
	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	if ctx.Viewer.ID == ctx.Medium.GetOwnerID() {
		return true, nil
	}

	actorPerm := NewActorPermissions()
	actorPermCtx := &ActorAccessContext{
		Viewer:         ctx.Viewer,
		Actor:          ctx.Medium.GetOwner(),
		ActorViewerRel: ctx.OwnerViewerRel,
	}
	ok, err := actorPerm.CanRead(actorPermCtx)
	if err != nil {
		return false, err
	}
	if ok {
		if ctx.Viewer.ID == ctx.Medium.GetAuthorID() {
			return true, nil
		}

		if ctx.Medium.GetAccess() == constants.AnahitaAccessPublic {
			return true, nil
		}

		if ctx.Medium.GetAccess() == constants.AnahitaAccessRegistered && ctx.Viewer.ID > 0 {
			return true, nil
		}

		if ctx.Medium.GetAccess() == constants.AnahitaAccessFollowers {
			if ctx.OwnerViewerRel.IsLeading || ctx.OwnerViewerRel.IsFollowing || ctx.OwnerViewerRel.IsMutual || ctx.OwnerViewerRel.HasAdmin {
				return true, nil
			}
		}

		if ctx.Medium.GetAccess() == constants.AnahitaAccessLeaders {
			if ctx.OwnerViewerRel.IsFollowing || ctx.OwnerViewerRel.IsMutual || ctx.OwnerViewerRel.HasAdmin {
				return true, nil
			}
		}

		if ctx.Medium.GetAccess() == constants.AnahitaAccessMutuals {
			if ctx.OwnerViewerRel.IsMutual || ctx.OwnerViewerRel.HasAdmin {
				return true, nil
			}
		}

		if ctx.Medium.GetAccess() == constants.AnahitaAccessAdmins {
			if ctx.OwnerViewerRel.HasAdmin {
				return true, nil
			}
		}

		if ctx.Medium.GetAccess() == constants.AnahitaAccessMyself {
			if ctx.Viewer.ID == ctx.Medium.GetOwnerID() {
				return true, nil
			}
		}
	}

	return false, errors.New("you are not allowed to read this medium node")
}

// CanEdit returns true if the viewer is allowed to edit the medium node
func (m *MediumPermissions) CanEdit(ctx *MediumAccessContext) (bool, error) {
	if !ctx.Viewer.HasScope(constants.OAuthScopeMediaWrite) {
		return false, errors.New(ErrMediumInsufficientScope)
	}

	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	if ctx.Viewer.ID > 0 && ctx.Viewer.ID == ctx.Medium.GetOwnerID() {
		return true, nil
	}

	actorPerm := NewActorPermissions()
	actorPermCtx := &ActorAccessContext{
		Viewer:         ctx.Viewer,
		Actor:          ctx.Medium.GetOwner(),
		ActorViewerRel: ctx.OwnerViewerRel,
	}
	ok, err := actorPerm.CanRead(actorPermCtx)
	if ok {
		if ctx.OwnerViewerRel.HasAdmin {
			return true, nil
		}

		if ctx.Viewer.ID == ctx.Medium.GetAuthorID() {
			return true, nil
		}
	}

	if err != nil {
		return false, err
	}

	return false, errors.New("you are not allowed to edit this medium node")
}

// CanAdd returns true if the viewer is allowed to add a medium node
func (m *MediumPermissions) CanAdd(ctx *MediumAccessContext) (bool, error) {
	if !ctx.Viewer.HasScope(constants.OAuthScopeMediaWrite) {
		return false, errors.New(ErrMediumInsufficientScope)
	}

	// Step 1: Global admin override
	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	owner := ctx.Medium.GetOwner()
	entityName := ctx.Medium.GetEntityName()
	// Step 2: Owner can always add to self
	if ctx.Viewer.ID > 0 && ctx.Viewer.ID == owner.GetID() {
		return true, nil
	}

	// Step 3: Can viewer read this actor (privacy check)
	actorPerm := NewActorPermissions()
	actorPermCtx := &ActorAccessContext{
		Viewer:         ctx.Viewer,
		Actor:          ctx.Medium.GetOwner(),
		ActorViewerRel: ctx.OwnerViewerRel,
	}
	ok, err := actorPerm.CanRead(actorPermCtx)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("viewer %d is not allowed to read actor %d", ctx.Viewer.ID, ctx.Medium.GetOwnerID())
	}

	// Step 4: Check add_permissions policies
	// Access can be either "followers" or "admins"
	if ctx.Medium.GetOwner().HasAddPermission(ctx.Medium.GetService(), ctx.Medium.GetEntityName(), "admins") {
		if ctx.OwnerViewerRel.HasAdmin {
			return true, nil
		}
	} else if ctx.Medium.GetOwner().HasAddPermission(ctx.Medium.GetService(), ctx.Medium.GetEntityName(), "followers") {
		if ctx.OwnerViewerRel.IsLeading || ctx.OwnerViewerRel.HasAdmin {
			return true, nil
		}
	}

	// Step 5: Denied
	return false, fmt.Errorf("viewer %d does not have permission to add %s to actor %d", ctx.Viewer.ID, entityName, ctx.Medium.GetOwnerID())
}

// CanDelete returns true if the viewer is allowed to delete the medium node
func (m *MediumPermissions) CanDelete(ctx *MediumAccessContext) (bool, error) {
	if !ctx.Viewer.HasScope(constants.OAuthScopeMediaWrite) {
		return false, errors.New(ErrMediumInsufficientScope)
	}

	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	if ctx.Viewer.ID > 0 && ctx.Viewer.ID == ctx.Medium.GetOwnerID() {
		return true, nil
	}

	actorPerm := NewActorPermissions()
	actorPermCtx := &ActorAccessContext{
		Viewer:         ctx.Viewer,
		Actor:          ctx.Medium.GetOwner(),
		ActorViewerRel: ctx.OwnerViewerRel,
	}
	ok, err := actorPerm.CanRead(actorPermCtx)
	if ok {
		if ctx.Viewer.ID == ctx.Medium.GetAuthorID() {
			return true, nil
		}

		if ctx.OwnerViewerRel.HasAdmin {
			return true, nil
		}
	}

	if err != nil {
		return false, err
	}

	return false, errors.New("you are not allowed to delete this medium node")
}

// CanAdministrate returns true if the viewer is allowed to administrate the medium node
func (m *MediumPermissions) CanAdministrate(ctx *MediumAccessContext) (bool, error) {
	if !ctx.Viewer.HasScope(constants.OAuthScopeMediaWrite) {
		return false, errors.New(ErrMediumInsufficientScope)
	}

	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	if ctx.Viewer.ID > 0 && ctx.Viewer.ID == ctx.Medium.GetOwnerID() {
		return true, nil
	}

	if ctx.OwnerViewerRel.HasAdmin {
		return true, nil
	}

	return false, errors.New("you are not allowed to administrate this medium node")
}
