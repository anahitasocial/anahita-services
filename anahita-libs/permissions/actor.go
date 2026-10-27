package permissions

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"errors"
)

const (
	ErrActorInsufficientScope      = "insufficient scope"
	ErrActorBlockedByActor         = "you are blocked by this actor"
	ErrActorReadNotAllowed         = "you are not allowed to read this actor"
	ErrActorEditNotAllowed         = "you are not allowed to edit this actor"
	ErrActorAddNotAllowed          = "you are not allowed to add this actor"
	ErrActorDeleteNotAllowed       = "you are not allowed to delete this actor"
	ErrActorAdministrateNotAllowed = "you are not allowed to administrate this actor"
	ErrActorAddAdminNotAllowed     = "you are not allowed to add an admin to this actor"
	ErrActorDeleteAdminNotAllowed  = "you are not allowed to delete an admin from this actor"
)

type ActorPermissionsInterface interface {
	CanBrowse(viewer *entities.Viewer) (bool, error)
	CanRead(ctx *ActorAccessContext) (bool, error)
	CanEdit(ctx *ActorAccessContext) (bool, error)
	CanAdd(ctx *ActorAccessContext) (bool, error)
	CanDelete(ctx *ActorAccessContext) (bool, error)
	CanAdministrate(ctx *ActorAccessContext) (bool, error)
	CanAddAdmin(ctx *ActorAccessContext, admin *entities.AnahitaNode) (bool, error)
	CanDeleteAdmin(ctx *ActorAccessContext, admin *entities.AnahitaNode) (bool, error)
}

type ActorsPermissions struct{}

func NewActorPermissions() ActorPermissionsInterface {
	return &ActorsPermissions{}
}

func (p *ActorsPermissions) CanBrowse(viewer *entities.Viewer) (bool, error) {
	return true, nil
}

func (p *ActorsPermissions) CanRead(ctx *ActorAccessContext) (bool, error) {
	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	if ctx.Viewer.ID == ctx.Actor.GetID() {
		return true, nil
	}

	if ctx.Viewer.ID > 0 && ctx.ActorViewerRel.IsBlocking {
		return false, errors.New(ErrActorBlockedByActor)
	}

	if ctx.Actor.GetAccess() == constants.AnahitaAccessPublic {
		return true, nil
	}

	if ctx.Actor.GetAccess() == constants.AnahitaAccessRegistered && ctx.Viewer.ID > 0 {
		return true, nil
	}

	if ctx.Actor.GetAccess() == constants.AnahitaAccessFollowers {
		if ctx.ActorViewerRel.IsLeading || ctx.ActorViewerRel.IsFollowing || ctx.ActorViewerRel.IsMutual || ctx.ActorViewerRel.HasAdmin {
			return true, nil
		}
	}

	if ctx.Actor.GetAccess() == constants.AnahitaAccessLeaders {
		if ctx.ActorViewerRel.IsFollowing || ctx.ActorViewerRel.IsMutual || ctx.ActorViewerRel.HasAdmin {
			return true, nil
		}
	}

	if ctx.Actor.GetAccess() == constants.AnahitaAccessMutuals {
		if ctx.ActorViewerRel.IsMutual || ctx.ActorViewerRel.HasAdmin {
			return true, nil
		}
	}

	if ctx.Actor.GetAccess() == constants.AnahitaAccessAdmins && ctx.ActorViewerRel.HasAdmin {
		return true, nil
	}

	return false, errors.New(ErrActorReadNotAllowed)
}

func (p *ActorsPermissions) CanEdit(ctx *ActorAccessContext) (bool, error) {
	if !ctx.Viewer.HasScope(constants.OAuthScopeActorsWrite) {
		return false, errors.New(ErrActorInsufficientScope)
	}

	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	if ctx.Viewer.ID == ctx.Actor.GetID() {
		return true, nil
	}

	if ctx.ActorViewerRel.HasAdmin {
		return true, nil
	}

	return false, errors.New(ErrActorEditNotAllowed)
}

// CanAdd reports whether the viewer may create an actor.
//
// By RANK against the configured level, with NO administrator shortcut above
// it. This used to open with an unconditional `if ctx.Viewer.IsAdmin()`, which
// was correct while administrators were the only answer and becomes a bypass
// the moment the answer is configurable: `super-administrators` would be a
// level that did nothing, and `registered` would still refuse the registered.
func (p *ActorsPermissions) CanAdd(ctx *ActorAccessContext) (bool, error) {
	if !ctx.Viewer.HasScope(constants.OAuthScopeActorsWrite) {
		return false, errors.New(ErrActorInsufficientScope)
	}

	if constants.MeetsRoleLevel(ctx.Viewer.PersonType, ctx.CreateLevel) {
		return true, nil
	}

	return false, errors.New(ErrActorAddNotAllowed)
}

func (p *ActorsPermissions) CanDelete(ctx *ActorAccessContext) (bool, error) {
	if !ctx.Viewer.HasScope(constants.OAuthScopeActorsWrite) {
		return false, errors.New(ErrActorInsufficientScope)
	}

	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	if ctx.Viewer.ID > 0 && ctx.Viewer.ID == ctx.Actor.GetID() {
		return true, nil
	}

	if ctx.ActorViewerRel.HasAdmin {
		return true, nil
	}

	return false, errors.New(ErrActorDeleteNotAllowed)
}

func (p *ActorsPermissions) CanAdministrate(ctx *ActorAccessContext) (bool, error) {
	if !ctx.Viewer.HasScope(constants.OAuthScopeActorsWrite) {
		return false, errors.New(ErrActorInsufficientScope)
	}

	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	if ctx.Viewer.ID == ctx.Actor.GetID() {
		return true, nil
	}

	if ctx.Viewer.ID > 0 && ctx.ActorViewerRel.HasAdmin {
		return true, nil
	}

	return false, errors.New(ErrActorAdministrateNotAllowed)
}

func (p *ActorsPermissions) CanAddAdmin(ctx *ActorAccessContext, admin *entities.AnahitaNode) (bool, error) {
	if !ctx.Viewer.HasScope(constants.OAuthScopeActorsWrite) {
		return false, errors.New(ErrActorInsufficientScope)
	}

	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	if ctx.Viewer.ID > 0 && ctx.ActorViewerRel.HasAdmin && ctx.AdminViewerRel.HasAdmin {
		return true, nil
	}

	return false, errors.New(ErrActorAddAdminNotAllowed)
}

func (p *ActorsPermissions) CanDeleteAdmin(ctx *ActorAccessContext, admin *entities.AnahitaNode) (bool, error) {
	if !ctx.Viewer.HasScope(constants.OAuthScopeActorsWrite) {
		return false, errors.New(ErrActorInsufficientScope)
	}

	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	if ctx.Viewer.ID > 0 && ctx.ActorViewerRel.HasAdmin {
		return true, nil
	}

	return false, errors.New(ErrActorDeleteAdminNotAllowed)
}
