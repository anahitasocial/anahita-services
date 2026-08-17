package permissions

import (
	"errors"

	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
)

const (
	ErrInviteInsufficientScope = "insufficient scope"
	ErrInviteBrowse            = "only admins or super admins can browse invites"
	ErrInviteAdd               = "only admins or super admins can add invites"
	ErrInviteDelete            = "only admins or super admins can delete invites"
)

type InvitePerm struct {
	*Env
}

// NewInvitePerm creates a new InvitePerm instance
func NewInvitePerm(env *Env) *InvitePerm {
	return &InvitePerm{
		env,
	}
}

// Invites are inherently a "managing people" operation. Browse uses
// actors:read; add/delete use actors:write. The scope catalog already
// gates these as AdminOnly at token-mint time, so a non-admin will
// never have these scopes — the redundant role check below is defense
// in depth.

// CanBrowse checks if the viewer can browse invites.
func (prm *InvitePerm) CanBrowse(viewer *entities.Viewer) error {
	if !viewer.HasScope(libs_constants.OAuthScopeActorsRead) {
		return errors.New(ErrInviteInsufficientScope)
	}
	if !viewer.IsAdminOrSuperAdmin() {
		return errors.New(ErrInviteBrowse)
	}
	return nil
}

// CanAdd checks if the viewer can add invites.
func (prm *InvitePerm) CanAdd(viewer *entities.Viewer) error {
	if !viewer.HasScope(libs_constants.OAuthScopeActorsWrite) {
		return errors.New(ErrInviteInsufficientScope)
	}
	if !viewer.IsAdminOrSuperAdmin() {
		return errors.New(ErrInviteAdd)
	}
	return nil
}

// CanDelete checks if the viewer can delete invites.
func (prm *InvitePerm) CanDelete(viewer *entities.Viewer) error {
	if !viewer.HasScope(libs_constants.OAuthScopeActorsWrite) {
		return errors.New(ErrInviteInsufficientScope)
	}
	if !viewer.IsAdminOrSuperAdmin() {
		return errors.New(ErrInviteDelete)
	}
	return nil
}
