package permissions

import (
	"anahita-libs/cmd/api/permissions"
	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"errors"
	"fmt"
)

type followersPermissions struct {
	*Env
}

func newFollowersPermissions(env *Env) PermissionsInterface {
	return &followersPermissions{
		Env: env,
	}
}

func (p *followersPermissions) CanBrowse(actor *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
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

	return false, fmt.Errorf("%s: viewer %d, actor %d", ErrFollowersBrowseNotAllowed, viewer.ID, actor.ID)
}

func (p *followersPermissions) CanEdit(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	return false, errors.New("not implemented")
}

func (p *followersPermissions) CanAdd(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeSocialgraphWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	if source.ID == target.ID {
		return false, fmt.Errorf("source %d cannot follow itself", source.ID)
	}

	if viewer.ID == target.ID {
		return false, fmt.Errorf("viewer %d cannot follow itself", viewer.ID)
	}

	targetVsViewer, err := p.Srv.SocialgraphGRPC.Read(target.ID, viewer.ID)
	if err != nil {
		return false, err
	}

	actorPerm := permissions.NewActorPermissions()
	ctx := &permissions.ActorAccessContext{
		Viewer:         viewer,
		Actor:          target,
		ActorViewerRel: targetVsViewer,
	}

	ok, err := actorPerm.CanRead(ctx)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("%s: source %d, target %d", ErrFollowersAddNotAllowed, source.ID, target.ID)
	}

	targetVsSourceRel, err := p.Srv.SocialgraphGRPC.Read(target.ID, source.ID)
	if err != nil {
		return false, err
	}
	if targetVsSourceRel.IsBlocking {
		return false, fmt.Errorf("target %d is blocking source %d", target.ID, source.ID)
	}

	if target.IsPerson() {
		return true, nil
	}

	// Joining a group yourself: seeing it is enough, as for a person. The
	// read check above has already asked that.
	if source.ID == viewer.ID {
		return true, nil
	}

	// Adding somebody else: the group's "follower" permission, measured
	// against the VIEWER doing the adding.
	//
	// It used to test targetVsSourceRel.HasAdmin — whether the person being
	// ADDED administers the group — and every join goes through here with
	// the joiner as the source. So with the default, admins, nobody but the
	// group's own administrators could follow a public group, and those
	// administrators could not add anybody else.
	if permissions.CanAddOthersAsFollowers(viewer, target, targetVsViewer) {
		return true, nil
	}

	return false, fmt.Errorf("%s: viewer %d, source %d, target %d", ErrFollowersAddNotAllowed, viewer.ID, source.ID, target.ID)
}

func (p *followersPermissions) CanDelete(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeSocialgraphWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	targetVsViewer, err := p.Srv.SocialgraphGRPC.Read(target.ID, viewer.ID)
	if err != nil {
		return false, err
	}

	actorPerm := permissions.NewActorPermissions()
	ctx := &permissions.ActorAccessContext{
		Viewer:         viewer,
		Actor:          target,
		ActorViewerRel: targetVsViewer,
	}

	ok, err := actorPerm.CanRead(ctx)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}

	return false, fmt.Errorf("%s: viewer %d, source %d, target %d", ErrFollowersDeleteNotAllowed, viewer.ID, source.ID, target.ID)
}
