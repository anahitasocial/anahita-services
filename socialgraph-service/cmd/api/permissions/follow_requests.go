package permissions

import (
	"anahita-libs/cmd/api/permissions"
	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"errors"
	"fmt"
)

type followRequestsPermissions struct {
	*Env
}

func newFollowRequestsPermissions(env *Env) PermissionsInterface {
	return &followRequestsPermissions{
		Env: env,
	}
}

func (p *followRequestsPermissions) CanBrowse(actor *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeSocialgraphRead) {
		return false, errors.New(ErrInsufficientScope)
	}

	rel, err := p.Srv.SocialgraphGRPC.Read(actor.ID, viewer.ID)
	if err != nil {
		return false, err
	}

	actorPerm := permissions.NewActorPermissions()
	ctx := &permissions.ActorAccessContext{
		Viewer:         viewer,
		Actor:          actor,
		ActorViewerRel: rel,
	}

	ok, err := actorPerm.CanRead(ctx)
	if err != nil {
		return false, err
	}

	if ok {
		ok, err = actorPerm.CanAdministrate(ctx)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, fmt.Errorf("%s: viewer %d, actor %d", ErrFollowRequestBrowseNotAllowed, viewer.ID, actor.ID)
}

func (p *followRequestsPermissions) CanEdit(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeSocialgraphWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	ctx, err := followContext(p.Env, source, target, viewer)
	if err != nil {
		return false, err
	}

	// Accepting: whoever looks after the profile that was asked.
	if permissions.CanAcceptFollowRequest(ctx) {
		return true, nil
	}

	return false, errors.New(ErrFollowRequestEditNotAllowed)
}

func (p *followRequestsPermissions) CanAdd(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeSocialgraphWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	ctx, err := followContext(p.Env, source, target, viewer)
	if err != nil {
		return false, err
	}

	// Asking is for yourself, of a profile that has said it may be asked.
	// It used to need the asker to be able to see the profile, and a
	// profile that can be seen can be followed outright: the only profiles
	// worth asking were the ones nobody was allowed to ask.
	if permissions.CanRequestToFollow(ctx) {
		return true, nil
	}

	return false, errors.New(ErrFollowRequestAddNotAllowed)
}

func (p *followRequestsPermissions) CanDelete(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeSocialgraphWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	ctx, err := followContext(p.Env, source, target, viewer)
	if err != nil {
		return false, err
	}

	// Ignoring it, as whoever looks after the profile that was asked, or
	// taking it back, as whoever asked. The two arguments were named the
	// wrong way round here, so the question was put about the asker's
	// profile and whoever had been asked could not ignore anything.
	if permissions.CanDropFollowRequest(ctx) {
		return true, nil
	}

	return false, errors.New(ErrFollowRequestDeleteNotAllowed)
}
