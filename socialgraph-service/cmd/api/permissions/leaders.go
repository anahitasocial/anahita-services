package permissions

import (
	"anahita-libs/data/entities"
	"anahita-libs/permissions"
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

	ctx := &permissions.ActorAccessContext{
		Viewer:         viewer,
		Actor:          actor,
		ActorViewerRel: rel,
	}

	// Whoever may see the profile, narrowed by who it lets see its
	// followers and whom it follows (roadmap row 151).
	if permissions.CanSeeFollows(ctx) {
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
