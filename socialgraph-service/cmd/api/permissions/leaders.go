package permissions

import (
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"errors"
	"fmt"
)

type leadersPermissions struct {
	*Env
}

func newLeadersPermissions(env *Env) PermissionsInterface {
	return &leadersPermissions{
		Env: env,
	}
}

func (p *leadersPermissions) CanBrowse(actor *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
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
		return true, nil
	}

	return false, fmt.Errorf("%s: viewer %d, actor %d", ErrLeadersBrowseNotAllowed, viewer.ID, actor.ID)
}

func (p *leadersPermissions) CanEdit(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	return false, errors.New("not implemented")
}

func (p *leadersPermissions) CanAdd(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	return false, errors.New("not implemented")
}

func (p *leadersPermissions) CanDelete(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	return false, errors.New("not implemented")
}
