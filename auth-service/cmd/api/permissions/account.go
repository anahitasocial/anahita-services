package permissions

import (
	"errors"

	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
)

const (
	ErrAccountInsufficientScope = "insufficient scope"
	ErrAccountBrowse            = "only super admins can browse the accounts list"
	ErrAccountMakeRegistered    = "only super admins can make everything public registered-only"
)

type AccountPerm struct {
	*Env
}

func NewAccountPerm(env *Env) *AccountPerm {
	return &AccountPerm{
		env,
	}
}

// CanBrowse gates the administration list of accounts.
//
// Super administrators only, where the approval queue next to it admits any
// administrator. The list says who has verified an address, who has not been
// seen for years and how much each has written, and its purpose is choosing
// accounts to remove for good, which only a super administrator may do.
func (prm *AccountPerm) CanBrowse(viewer *entities.Viewer) error {
	if !viewer.HasScope(libs_constants.OAuthScopeActorsRead) {
		return errors.New(ErrAccountInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return errors.New(ErrAccountBrowse)
	}
	return nil
}

// CanMakeRegistered gates the one-step change of everything public to
// registered-only. Super administrators only: it rewrites who can see
// thousands of other people's posts and profiles, and cannot be undone.
func (prm *AccountPerm) CanMakeRegistered(viewer *entities.Viewer) error {
	if !viewer.HasScope(libs_constants.OAuthScopeActorsWrite) {
		return errors.New(ErrAccountInsufficientScope)
	}
	if !viewer.IsSuperAdmin() {
		return errors.New(ErrAccountMakeRegistered)
	}
	return nil
}
