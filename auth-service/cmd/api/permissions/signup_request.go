package permissions

import (
	"errors"

	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
)

const (
	ErrSignupRequestInsufficientScope = "insufficient scope"

	ErrSignupRequestBrowse  = "only admins or super admins can browse signup requests"
	ErrSignupRequestApprove = "only admins or super admins can approve a signup request"
	ErrSignupRequestReject  = "only admins or super admins can reject a signup request"
)

type SignupRequestPerm struct {
	*Env
}

func NewSignupRequestPerm(env *Env) *SignupRequestPerm {
	return &SignupRequestPerm{
		env,
	}
}

// The approval queue's own gates.
//
// These used to be Perms.Invite's, and the three methods did not agree
// about who may do what. Browse borrowed CanBrowse and Reject borrowed
// CanDelete, both of which are IsAdminOrSuperAdmin — but Approve
// borrowed CanAdd, which carries a branch the other two do not:
//
//	if prm.Config.MembersMayInvite() && viewer.IsRegistered() {
//	    return nil
//	}
//
// That branch is the right answer to "may this person send an
// invitation". It is the wrong answer to "may this person create an
// account", which is what approving a signup request does. Wherever
// the invite level admitted registered people, it admitted every one
// of them here too — and actors:write, the scope checked first, is
// held by every registered person — so the account-creating half of
// the queue was gated more loosely than the rejecting half. Deciding
// who joins does not move with a setting about invitations.
//
// Kept separate from InvitePerm rather than fixed in place so the two
// questions cannot be conflated again: a future change to who may
// invite has nothing here to reach.
//
// Scopes are unchanged from what the borrowed predicates asked for:
// actors:read to browse, actors:write to decide. Only the role branch
// is different. Tightening the browse scope as well would be a second
// change riding along with a security fix, and it could refuse an
// administrator holding a read-only token who can read the queue today.

func (prm *SignupRequestPerm) CanBrowse(viewer *entities.Viewer) error {
	if !viewer.HasScope(libs_constants.OAuthScopeActorsRead) {
		return errors.New(ErrSignupRequestInsufficientScope)
	}
	if !viewer.IsAdminOrSuperAdmin() {
		return errors.New(ErrSignupRequestBrowse)
	}
	return nil
}

func (prm *SignupRequestPerm) CanApprove(viewer *entities.Viewer) error {
	if !viewer.HasScope(libs_constants.OAuthScopeActorsWrite) {
		return errors.New(ErrSignupRequestInsufficientScope)
	}
	if !viewer.IsAdminOrSuperAdmin() {
		return errors.New(ErrSignupRequestApprove)
	}
	return nil
}

func (prm *SignupRequestPerm) CanReject(viewer *entities.Viewer) error {
	if !viewer.HasScope(libs_constants.OAuthScopeActorsWrite) {
		return errors.New(ErrSignupRequestInsufficientScope)
	}
	if !viewer.IsAdminOrSuperAdmin() {
		return errors.New(ErrSignupRequestReject)
	}
	return nil
}
