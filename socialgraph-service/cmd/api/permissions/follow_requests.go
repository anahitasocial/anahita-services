package permissions

import (
	"anahita-libs/cmd/api/permissions"
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
	if !viewer.HasScope("read:socialgraph") {
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
	if !viewer.HasScope("write:socialgraph") {
		return false, errors.New(ErrInsufficientScope)
	}

	rel, err := p.Srv.SocialgraphGRPC.Read(target.ID, viewer.ID)
	if err != nil {
		return false, err
	}

	actorPerm := permissions.NewActorPermissions()
	ctx := &permissions.ActorAccessContext{
		Viewer:         viewer,
		Actor:          target,
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

	return false, errors.New(ErrFollowRequestEditNotAllowed)
}

func (p *followRequestsPermissions) CanAdd(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope("write:socialgraph") {
		return false, errors.New(ErrInsufficientScope)
	}

	rel, err := p.Srv.SocialgraphGRPC.Read(target.ID, viewer.ID)
	if err != nil {
		return false, err
	}

	actorPerm := permissions.NewActorPermissions()
	ctx := &permissions.ActorAccessContext{
		Viewer:         viewer,
		Actor:          target,
		ActorViewerRel: rel,
	}

	ok, err := actorPerm.CanRead(ctx)
	if err != nil {
		return false, err
	}

	targetVsSourceRel, err := p.Srv.SocialgraphGRPC.Read(target.ID, source.ID)
	if err != nil {
		return false, err
	}
	sourceVsTargetRel, err := p.Srv.SocialgraphGRPC.Read(source.ID, target.ID)
	if err != nil {
		return false, err
	}

	if ok && !targetVsSourceRel.IsBlocking && !sourceVsTargetRel.IsFollowing {
		return true, nil
	}

	return false, errors.New(ErrFollowRequestAddNotAllowed)
}

func (p *followRequestsPermissions) CanDelete(target *entities.AnahitaNode, source *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope("write:socialgraph") {
		return false, errors.New(ErrInsufficientScope)
	}

	rel, err := p.Srv.SocialgraphGRPC.Read(target.ID, viewer.ID)
	if err != nil {
		return false, err
	}

	actorPerm := permissions.NewActorPermissions()
	ctx := &permissions.ActorAccessContext{
		Viewer:         viewer,
		Actor:          target,
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

	return false, errors.New(ErrFollowRequestDeleteNotAllowed)
}
