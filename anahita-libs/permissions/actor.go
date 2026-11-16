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

	if MeetsAccessLevel(ctx.Actor.GetAccess(), ctx) {
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

// MeetsAccessLevel says whether the viewer is admitted by an access level of
// this actor's: anybody for public, anybody signed in for registered, and
// for the rest how the viewer stands with it. A profile's administrators
// meet every level. The viewer's being the actor, an administrator of the
// site, or blocked, is for whoever asks to settle first.
func MeetsAccessLevel(level string, ctx *ActorAccessContext) bool {
	rel := ctx.ActorViewerRel
	if rel == nil {
		rel = &entities.ActorRelation{}
	}
	switch level {
	case constants.AnahitaAccessPublic:
		return true
	case constants.AnahitaAccessRegistered:
		return ctx.Viewer.ID > 0
	case constants.AnahitaAccessFollowers:
		return rel.IsLeading || rel.IsFollowing || rel.IsMutual || rel.HasAdmin
	case constants.AnahitaAccessLeaders:
		return rel.IsFollowing || rel.IsMutual || rel.HasAdmin
	case constants.AnahitaAccessMutuals:
		return rel.IsMutual || rel.HasAdmin
	case constants.AnahitaAccessAdmins:
		return rel.HasAdmin
	}
	return false
}

// CanSeeFollows says whether the viewer may see an actor's followers and
// whom it follows (roadmap row 151): whoever may see the actor, narrowed by
// its graph access when it has one. Never wider than the actor itself. Its
// own administrators, itself and the site's administrators always may.
func CanSeeFollows(ctx *ActorAccessContext) bool {
	if ok, _ := NewActorPermissions().CanRead(ctx); !ok {
		return false
	}
	level := ctx.Actor.GetGraphAccess()
	if level == "" || ctx.Viewer.IsAdmin() || ctx.Viewer.ID == ctx.Actor.GetID() {
		return true
	}
	if ctx.ActorViewerRel != nil && ctx.ActorViewerRel.HasAdmin {
		return true
	}
	return MeetsAccessLevel(level, ctx)
}
