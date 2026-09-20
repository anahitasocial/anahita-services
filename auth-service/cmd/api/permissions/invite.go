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

// ErrInviteNotPermitted is what somebody below the configured level gets.
//
// Level-neutral wording. It used to say "only administrators can invite
// people to this site", which was true when administrators were the
// only alternative to everybody; with INVITES_FROM naming a minimum
// role it would state the wrong rule on any site set to
// super-administrators, and contradict itself on one set to registered.
var ErrInviteNotPermitted = errors.New(
	"you are not permitted to invite people to this site")

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
// That is also why opening invitations to registered people needs no
// scope change — they already hold it.

// CanBrowse checks if the viewer can browse invites.
//
// ANY REGISTERED PERSON, because the list is their own. The handler
// scopes it on created_by, so browsing returns the invitations the
// viewer sent and nothing else — which makes this self-service, the
// same shape as reading your own passkeys or auth logs, rather than an
// administrative power.
//
// It was administrator-only while the list was site-wide, and had to
// be: every row carries a recipient's email address. Widening it is
// safe only BECAUSE of the scoping, and only for as long as it holds.
// If a future caller ever needs the unscoped list, this goes back to
// admin-only in the same commit.
func (prm *InvitePerm) CanBrowse(viewer *entities.Viewer) error {
	if !viewer.HasScope(libs_constants.OAuthScopeActorsRead) {
		return errors.New(ErrInviteInsufficientScope)
	}
	if !viewer.IsRegistered() && !viewer.IsAdminOrSuperAdmin() {
		return errors.New(ErrInviteBrowse)
	}
	return nil
}

// CanAdd checks if the viewer can add invites.
//
// Who that is depends on INVITES_FROM, which names a MINIMUM ROLE:
// `registered`, `administrators` (the default) or
// `super-administrators`. Each admits that role and everything above.
//
// One comparison, not a ladder. This used to open with an
// unconditional `if viewer.IsAdminOrSuperAdmin() { return nil }` above
// the config check, which meant an administrator could invite whatever
// the operator had configured — so `super-administrators` would have
// been a level that changed nothing, while appearing to tighten the
// site. Config.MayInvite ranks the viewer against the setting instead,
// and there is no path around it.
func (prm *InvitePerm) CanAdd(viewer *entities.Viewer) error {
	if !viewer.HasScope(libs_constants.OAuthScopeActorsWrite) {
		return errors.New(ErrInviteInsufficientScope)
	}

	if !prm.Config.MayInvite(viewer.PersonType) {
		return ErrInviteNotPermitted
	}

	return nil
}

// CanDeleteOwn checks whether the viewer may revoke a specific invitation.
//
// An administrator may revoke any. Anybody who may invite may revoke ONE
// THEY ISSUED — withdrawing your own invitation is your own action, while
// revoking somebody else's is moderation.
//
// This exists because the cap has to be escapable. Somebody who spends
// theirs on three mistyped addresses is otherwise stuck for a day with no
// way out, while being told by the refusal itself that revoking would free
// a slot.
func (prm *InvitePerm) CanDeleteOwn(viewer *entities.Viewer, issuedBy int64) error {
	if err := prm.CanDelete(viewer); err == nil {
		return nil
	}

	if !viewer.HasScope(libs_constants.OAuthScopeActorsWrite) {
		return errors.New(ErrInviteInsufficientScope)
	}

	if prm.Config.MayInvite(viewer.PersonType) && viewer.ID == issuedBy {
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
