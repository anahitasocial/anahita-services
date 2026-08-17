package permissions

import (
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"errors"
	"fmt"
)

type mutualsPermissions struct {
	*Env
}

func newMutualsPermissions(env *Env) PermissionsInterface {
	return &mutualsPermissions{
		Env: env,
	}
}

func (p *mutualsPermissions) CanBrowse(actor *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
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

	return false, fmt.Errorf("%s: viewer %d, actor %d", ErrMutualsBrowseNotAllowed, viewer.ID, actor.ID)
}

func (p *mutualsPermissions) CanEdit(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	return false, errors.New("not implemented")
}

func (p *mutualsPermissions) CanAdd(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	return false, errors.New("doesn't apply to mutuals")
}

func (p *mutualsPermissions) CanDelete(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	return false, errors.New("doesn't apply to mutuals")
}
