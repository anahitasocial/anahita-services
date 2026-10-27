package permissions

import (
	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/permissions"
	"errors"
	"fmt"
)

type blocksPermissions struct {
	*Env
}

func newBlocksPermissions(env *Env) PermissionsInterface {
	return &blocksPermissions{
		Env: env,
	}
}

func (p *blocksPermissions) CanBrowse(actor *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
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
		if viewer.IsAdmin() || viewer.ID == actor.ID || ctx.ActorViewerRel.HasAdmin {
			return true, nil
		}

		ok, err := actorPerm.CanAdministrate(ctx)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, fmt.Errorf("%s: viewer %d, actor %d", ErrBlocksBrowseNotAllowed, viewer.ID, actor.GetID())
}

func (p *blocksPermissions) CanEdit(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	return false, errors.New("not implemented")
}

func (p *blocksPermissions) CanAdd(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeSocialgraphWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	if source.ID == target.GetID() {
		return false, fmt.Errorf("source %d cannot block itself", source.ID)
	}

	if target.GetID() == viewer.ID {
		return false, fmt.Errorf("viewer %d cannot block itself", viewer.ID)
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
	sourceVsViewerRel, err := p.Srv.SocialgraphGRPC.Read(source.ID, viewer.ID)
	if err != nil {
		return false, err
	}

	if ok && target.ID != source.ID && !targetVsSourceRel.IsBlocking && !sourceVsTargetRel.IsBlocking {
		if viewer.IsAdmin() || sourceVsViewerRel.HasAdmin || viewer.ID == source.ID {
			return true, nil
		}
	}

	return false, fmt.Errorf("%s: source %d, target %d", ErrBlocksAddNotAllowed, source.ID, target.ID)
}

func (p *blocksPermissions) CanDelete(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeSocialgraphWrite) {
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

	sourceVsTargetRel, err := p.Srv.SocialgraphGRPC.Read(source.ID, target.ID)
	if err != nil {
		return false, err
	}
	sourceVsViewerRel, err := p.Srv.SocialgraphGRPC.Read(source.ID, viewer.ID)
	if err != nil {
		return false, err
	}

	if ok && target.ID != source.ID && sourceVsTargetRel.IsBlocking {
		if viewer.IsAdmin() || sourceVsViewerRel.HasAdmin || viewer.ID == source.ID {
			return true, nil
		}
	}

	return false, fmt.Errorf("%s: source %d, target %d", ErrBlocksDeleteNotAllowed, source.ID, target.ID)
}
