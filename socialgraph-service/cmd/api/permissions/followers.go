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

	ctx, err := followContext(p.Env, source, target, viewer)
	if err != nil {
		return false, err
	}

	// The rule is in anahita-libs: following is done for yourself, to
	// something you can see, and the one case of adding somebody else is a
	// group's "follower" permission.
	if permissions.CanFollow(ctx) {
		return true, nil
	}

	return false, fmt.Errorf("%s: viewer %d, source %d, target %d", ErrFollowersAddNotAllowed, viewer.ID, source.ID, target.ID)
}

func (p *followersPermissions) CanDelete(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeSocialgraphWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	ctx, err := followContext(p.Env, source, target, viewer)
	if err != nil {
		return false, err
	}

	// Unfollowing, or whoever looks after the target removing a follower.
	// It was anybody who could see the target.
	if permissions.CanRemoveFollower(ctx) {
		return true, nil
	}

	return false, fmt.Errorf("%s: viewer %d, source %d, target %d", ErrFollowersDeleteNotAllowed, viewer.ID, source.ID, target.ID)
}

// followContext reads what the follow rules are asked over: the target's
// relation to the viewer, and to the source when that is somebody else.
func followContext(env *Env, source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.FollowContext, error) {
	targetVsViewer, err := env.Srv.SocialgraphGRPC.Read(target.ID, viewer.ID)
	if err != nil {
		return nil, err
	}

	targetVsSource := targetVsViewer
	if source.ID != viewer.ID {
		targetVsSource, err = env.Srv.SocialgraphGRPC.Read(target.ID, source.ID)
		if err != nil {
			return nil, err
		}
	}

	return &permissions.FollowContext{
		Viewer:          viewer,
		Source:          source,
		Target:          target,
		TargetViewerRel: targetVsViewer,
		TargetSourceRel: targetVsSource,
	}, nil
}
