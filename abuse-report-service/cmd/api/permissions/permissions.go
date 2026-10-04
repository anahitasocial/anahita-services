// Package permissions says who may do what with abuse reports.
//
// Two groups of people. Anybody signed in may report something they can see
// and read the list of reasons. Only administrators and super administrators
// may read what has been reported, see who reported it, and decide a case.
//
// No scope of its own. Filing a report is something a person does on their
// own account's behalf, so it needs account:write. The administrators' side
// uses actors:read and actors:write, as the signup queue does: deciding a
// case is deciding what happens to somebody's profile or posts.
package permissions

import (
	"errors"

	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
)

var (
	ErrInsufficientScope = errors.New("insufficient scope")
	ErrNotSignedIn       = errors.New("sign in to report something")
	ErrAdminsOnly        = errors.New("only administrators can see or decide reports")
)

type Permissions struct{}

func NewPermissions() *Permissions {
	return &Permissions{}
}

// CanReport: anybody signed in, on an enabled account.
func (p *Permissions) CanReport(viewer *entities.Viewer) error {
	if viewer == nil || viewer.ID == 0 {
		return ErrNotSignedIn
	}
	if !viewer.HasScope(libs_constants.OAuthScopeAccountWrite) {
		return ErrInsufficientScope
	}
	return nil
}

// CanBrowseCases: administrators, reading.
func (p *Permissions) CanBrowseCases(viewer *entities.Viewer) error {
	if viewer == nil || !viewer.IsAdminOrSuperAdmin() {
		return ErrAdminsOnly
	}
	if !viewer.HasScope(libs_constants.OAuthScopeActorsRead) {
		return ErrInsufficientScope
	}
	return nil
}

// CanDecideCases: administrators, writing.
func (p *Permissions) CanDecideCases(viewer *entities.Viewer) error {
	if viewer == nil || !viewer.IsAdminOrSuperAdmin() {
		return ErrAdminsOnly
	}
	if !viewer.HasScope(libs_constants.OAuthScopeActorsWrite) {
		return ErrInsufficientScope
	}
	return nil
}
