package permissions

import (
	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"errors"
	"fmt"
)

// Mutes are a person's own (roadmap row 133): only they mute, unmute and see
// whom they muted. Not a group's, not an administrator's on their behalf.
type mutesPermissions struct {
	*Env
}

func newMutesPermissions(env *Env) PermissionsInterface {
	return &mutesPermissions{Env: env}
}

func (p *mutesPermissions) CanBrowse(actor *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeSocialgraphRead) {
		return false, errors.New(ErrInsufficientScope)
	}
	if viewer.ID == 0 || actor.ID != viewer.ID {
		return false, fmt.Errorf("%s: viewer %d, actor %d", ErrMutesBrowseNotAllowed, viewer.ID, actor.ID)
	}
	return true, nil
}

func (p *mutesPermissions) CanEdit(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	return false, errors.New("not implemented")
}

// CanAdd: the viewer mutes a person, here or elsewhere, who is not them and
// is not blocked either way; a block hides more than a mute already.
func (p *mutesPermissions) CanAdd(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeSocialgraphWrite) {
		return false, errors.New(ErrInsufficientScope)
	}
	if viewer.ID == 0 || source.ID != viewer.ID || target.ID == source.ID || !target.IsPerson() {
		return false, fmt.Errorf("%s: source %d, target %d", ErrMutesAddNotAllowed, source.ID, target.ID)
	}

	mine, err := p.Srv.SocialgraphGRPC.Read(source.ID, target.ID)
	if err != nil {
		return false, err
	}
	theirs, err := p.Srv.SocialgraphGRPC.Read(target.ID, source.ID)
	if err != nil {
		return false, err
	}
	if mine.IsBlocking || theirs.IsBlocking {
		return false, fmt.Errorf("%s: blocked, source %d, target %d", ErrMutesAddNotAllowed, source.ID, target.ID)
	}

	return true, nil
}

// CanDelete: the viewer unmutes; unmuting somebody not muted does nothing.
func (p *mutesPermissions) CanDelete(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeSocialgraphWrite) {
		return false, errors.New(ErrInsufficientScope)
	}
	if viewer.ID == 0 || source.ID != viewer.ID {
		return false, fmt.Errorf("%s: source %d, target %d", ErrMutesDeleteNotAllowed, source.ID, target.ID)
	}
	return true, nil
}
