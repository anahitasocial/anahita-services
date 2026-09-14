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

// ErrInviteNotPermitted is what a member gets when this installation reserves
// invitations for administrators.
var ErrInviteNotPermitted = errors.New(
	"only administrators can invite people to this site")

// NewInvitePerm creates a new InvitePerm instance
func NewInvitePerm(env *Env) *InvitePerm {
	return &InvitePerm{
		env,
	}
}

// Browse uses actors:read; add and delete use actors:write.
//
// THE ROLE CHECK IS THE ONLY GATE, not a second one. A comment here used to
// say the scope catalog gates these as AdminOnly at token-mint time and the
// role check was defence in depth. It does not: actors:write is an ordinary
// scope every registered person is granted. Removing the role check would
// have opened invitations to everybody, on the strength of a second layer
// that was never there.
//
// That is also why opening invitations to members needs no scope change —
// they already hold it.

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
//
// Who that is depends on INVITES_FROM. Under `members` any registered person
// may invite; under `administrators`, the default, only they may.
func (prm *InvitePerm) CanAdd(viewer *entities.Viewer) error {
	if !viewer.HasScope(libs_constants.OAuthScopeActorsWrite) {
		return errors.New(ErrInviteInsufficientScope)
	}

	if viewer.IsAdminOrSuperAdmin() {
		return nil
	}

	if prm.Config.MembersMayInvite() && viewer.IsRegistered() {
		return nil
	}

	return ErrInviteNotPermitted
}

// CanDeleteOwn checks whether the viewer may revoke a specific invitation.
//
// An administrator may revoke any. A member may revoke ONE THEY ISSUED, and
// only under `members` — withdrawing your own invitation is your own action,
// while revoking somebody else's is moderation.
//
// This exists because the cap has to be escapable. A member who hits it with
// three mistyped addresses is otherwise stuck for a day with no way out, while
// being told by the refusal itself that revoking would free a slot.
func (prm *InvitePerm) CanDeleteOwn(viewer *entities.Viewer, issuedBy int64) error {
	if err := prm.CanDelete(viewer); err == nil {
		return nil
	}

	if !viewer.HasScope(libs_constants.OAuthScopeActorsWrite) {
		return errors.New(ErrInviteInsufficientScope)
	}

	if prm.Config.MembersMayInvite() && viewer.IsRegistered() && viewer.ID == issuedBy {
		return nil
	}

	return errors.New(ErrInviteDelete)
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
