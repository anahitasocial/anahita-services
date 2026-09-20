package permissions

import (
	"errors"

	libs_constants "anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"
	"auth/data/entities"
)

// Permission error strings. Declared as strings rather than error
// values to match the existing OAuth signing-key permissions, which
// permForbidden switches on by comparing err.Error().
const (
	ErrWebAuthnInsufficientScope = "insufficient scope for passkey management"
	ErrWebAuthnNotOwner          = "passkey belongs to another user"
	ErrWebAuthnNoViewer          = "no authenticated viewer"
	ErrWebAuthnNotAdmin          = "revoking another person's passkeys requires an administrator"
	ErrWebAuthnNoTarget          = "target user not found"
	ErrWebAuthnNoActor           = "viewer not hydrated"
)

type WebAuthnPerm struct{}

func NewWebAuthnPerm(env *Env) *WebAuthnPerm {
	return &WebAuthnPerm{}
}

// CanBrowse gates listing a person's own passkeys.
//
// Own only — there is deliberately no admin variant of this. See
// CanDeleteAll for why revoking is allowed to admins while listing is
// not.
func (p *WebAuthnPerm) CanBrowse(viewer *libs_entities.Viewer) (bool, error) {
	if viewer == nil || !viewer.Authenticated {
		return false, errors.New(ErrWebAuthnNoViewer)
	}
	if !viewer.HasScope(libs_constants.OAuthScopeAccountRead) {
		return false, errors.New(ErrWebAuthnInsufficientScope)
	}
	return true, nil
}

// CanAdd gates enrolling a new passkey.
//
// Deliberately not gated on TOTP. Registration already happens inside
// an authenticated session that passed full 2FA at login, and
// requiring a step-up here would mean a person has to reach for their
// phone to set up the thing whose entire purpose is to stop them
// reaching for their phone. The security event worth notifying on is
// the enrolment itself, which the notifier covers.
func (p *WebAuthnPerm) CanAdd(viewer *libs_entities.Viewer) (bool, error) {
	if viewer == nil || !viewer.Authenticated {
		return false, errors.New(ErrWebAuthnNoViewer)
	}
	if !viewer.HasScope(libs_constants.OAuthScopeAccountWrite) {
		return false, errors.New(ErrWebAuthnInsufficientScope)
	}
	return true, nil
}

// CanEdit gates renaming. Ownership is checked against the row, not
// inferred from the request — the UUID in the path is
// person-supplied.
func (p *WebAuthnPerm) CanEdit(
	viewer *libs_entities.Viewer,
	cred *entities.WebAuthnCredential,
) (bool, error) {
	return p.canMutate(viewer, cred)
}

// CanDelete gates revocation.
//
// Revoking your last passkey is allowed. It cannot lock anyone out:
// email + password + TOTP remains the permanent baseline, so the
// passkey is always an alternative and never the only door.
func (p *WebAuthnPerm) CanDelete(
	viewer *libs_entities.Viewer,
	cred *entities.WebAuthnCredential,
) (bool, error) {
	return p.canMutate(viewer, cred)
}

func (p *WebAuthnPerm) canMutate(
	viewer *libs_entities.Viewer,
	cred *entities.WebAuthnCredential,
) (bool, error) {
	if viewer == nil || !viewer.Authenticated {
		return false, errors.New(ErrWebAuthnNoViewer)
	}
	if !viewer.HasScope(libs_constants.OAuthScopeAccountWrite) {
		return false, errors.New(ErrWebAuthnInsufficientScope)
	}
	if cred == nil || cred.UserID != viewer.ID {
		// Same message and status for "not yours" and "doesn't
		// exist". Distinguishing them would turn this endpoint into
		// an oracle for whether a given credential UUID is real.
		return false, errors.New(ErrWebAuthnNotOwner)
	}
	return true, nil
}

// CanDeleteAll gates revoking every passkey belonging to someone else.
//
// ADMINISTRATOR ONLY, and deliberately asymmetric with CanBrowse:
// an admin may revoke another person's passkeys but may not list them.
//
// The asymmetry is the point. Listing reveals which devices a person
// carries — a MacBook, an iPhone, a YubiKey, when each was last used.
// That is device profiling of a person, and it buys nothing
// operationally, because the only admin action worth having needs no
// listing at all. Revoking is a single blunt operation on an account:
// no enumeration, no per-device visibility, nothing learned about the
// person beyond a count.
//
// Org admins are excluded. The scenarios that justify this — an
// incapacitated person, an account actively being taken over — are
// support escalations, not routine org administration, and an org
// admin who can strip a colleague's sign-in methods is a lateral
// attack path inside the tenant.
//
// This does not lock anyone out. Email, password, and TOTP remain the
// permanent baseline, so revoking passkeys removes a shortcut rather
// than access.
// actor is the viewer's hydrated Person. It is a separate parameter
// rather than being read off viewer.Person because that makes the
// requirement impossible to skip: a caller who has not hydrated has
// nothing to pass, instead of passing a zero-valued struct that
// silently fails every check below. See hydrateViewer in
// handlers/webauthn_helper.go for why that trap exists.
func (p *WebAuthnPerm) CanDeleteAll(
	viewer *libs_entities.Viewer,
	actor *entities.User,
	target *entities.User,
) (bool, error) {
	if viewer == nil || !viewer.Authenticated {
		return false, errors.New(ErrWebAuthnNoViewer)
	}
	if !viewer.HasScope(libs_constants.OAuthScopeAccountWrite) {
		return false, errors.New(ErrWebAuthnInsufficientScope)
	}
	if actor == nil {
		return false, errors.New(ErrWebAuthnNoActor)
	}
	if target == nil {
		return false, errors.New(ErrWebAuthnNoTarget)
	}

	// Acting on your own account needs no elevation — that is the
	// ordinary self-service path, just in bulk.
	if target.ID == actor.ID {
		return true, nil
	}

	// ADMINISTRATOR, not super administrator.
	//
	// Authority here divides by domain rather than by seniority: super
	// administrators own system configuration — OAuth clients, signing
	// keys — and site administrators own people and actors in full.
	// Revoking somebody's passkeys is squarely the second. It used to
	// require a super admin, which put an account-recovery action
	// behind the one role that should not be a bottleneck for it, and
	// left the neighbouring TOTP revoke — the same act on the other
	// factor — admitting administrators.
	if !actor.IsAdminOrSuperAdmin() {
		return false, errors.New(ErrWebAuthnNotAdmin)
	}

	// Presence is asked for separately, by the handler, through
	// requireStepUp. Authority and presence are different questions:
	// this function answers whether the right person is asking, and a
	// step-up answers whether they are at the keyboard.
	//
	// It used to be answered here, as `!actor.TotpEnabled ||
	// actor.IsMfaExpired()`. That was freshness, correctly — a global
	// admin session sitting open is not by itself authority to strip
	// someone's credentials — but freshness of TOTP SPECIFICALLY, so
	// an administrator carrying a passkey and no authenticator could
	// not do it at all. The same reasoning that moved the OAuth
	// endpoints onto step-up applies with more force here, because the
	// people most likely to hold only a passkey are the ones who
	// enrolled recently.
	return true, nil
}

// CanReadCount gates reading how many passkeys someone else holds.
//
// Administrator, same as CanDeleteAll — but deliberately without the
// step-up, because this is a read that populates a card rather than an
// action that destroys anything. Gating it the
// same way would mean the person page could not render for an admin
// with a stale session, which is a lot of friction to protect a
// single integer.
//
// A COUNT IS NOT A LIST, and the distinction is the whole reason this
// endpoint exists while a browse endpoint does not. Listing returns
// device models, transports, and last-used times — enough to profile
// which devices a person carries. A count says "this person uses
// passkeys, and how many", which reveals almost nothing about them
// and is the minimum an admin needs to know whether there is anything
// to revoke. Building the revoke UI without it meant offering an
// action that might do nothing, with no way to tell in advance.
func (p *WebAuthnPerm) CanReadCount(
	viewer *libs_entities.Viewer,
	actor *entities.User,
	target *entities.User,
) (bool, error) {
	if viewer == nil || !viewer.Authenticated {
		return false, errors.New(ErrWebAuthnNoViewer)
	}
	if !viewer.HasScope(libs_constants.OAuthScopeAccountWrite) {
		return false, errors.New(ErrWebAuthnInsufficientScope)
	}
	if actor == nil {
		return false, errors.New(ErrWebAuthnNoActor)
	}
	if target == nil {
		return false, errors.New(ErrWebAuthnNoTarget)
	}

	if target.ID == actor.ID {
		return true, nil
	}

	// Administrator, for the same reason as CanDeleteAll: how many
	// passkeys somebody holds is people administration, and a card that
	// an administrator can see but not act on would be worse than
	// either alone.
	if !actor.IsAdminOrSuperAdmin() {
		return false, errors.New(ErrWebAuthnNotAdmin)
	}

	return true, nil
}
