package permissions

import (
	"errors"

	"auth/data/entities"

	libs_constants "anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"
)

const (
	ErrAgreementInsufficientScope = "insufficient scope"
	ErrAgreementReadNotAllowed    = "you are not allowed to read this person's agreements"
)

type AgreementPerm struct {
	*Env
}

func NewAgreementPerm(env *Env) *AgreementPerm {
	return &AgreementPerm{
		env,
	}
}

// CanRead gates reading what somebody has accepted.
//
// Your own, always. A consent record is a record of what YOU agreed to,
// and the one person who should never have to ask an administrator for
// it is its subject — which is the whole argument for this being a read
// anybody can make about themselves rather than an administrative one.
//
// Somebody else's needs administrator or super administrator, the same
// shape as AuthLogPerm.CanBrowse. Both answer "what does this account's
// history look like", and they should not disagree about who may ask.
//
// Note what this does NOT gate: writing. There is no permission for
// accepting on another person's behalf because there is no endpoint —
// the PATCH routes take no person id at all, which makes it
// unexpressible rather than merely forbidden.
func (prm *AgreementPerm) CanRead(viewer *libs_entities.Viewer, user *entities.User) error {
	if !viewer.HasScope(libs_constants.OAuthScopeAccountRead) {
		return errors.New(ErrAgreementInsufficientScope)
	}

	if viewer.ID == user.ID {
		return nil
	}

	if !viewer.IsAdminOrSuperAdmin() {
		return errors.New(ErrAgreementReadNotAllowed)
	}

	return nil
}
