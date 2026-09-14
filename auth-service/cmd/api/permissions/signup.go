package permissions

import (
	"errors"

	"anahita-libs/data/entities"
)

// ErrRegistrationClosed is what a stranger gets when this installation does
// not take self-serve registrations.
var ErrRegistrationClosed = errors.New("this site is not open for registration")

type SignupPermissions struct {
	*Env
}

func NewSignupPermissions(env *Env) *SignupPermissions {
	return &SignupPermissions{env}
}

// CanAdd reports whether the viewer may create an account for themselves.
//
// Two questions, and both have to be asked here rather than in the page that
// renders the form. The client's opinion about whether signup is available was
// the whole problem with REACT_APP_SIGNUP_CLOSED: it hid a tab while
// POST /signups stayed open to anybody who noticed.
func (p *SignupPermissions) CanAdd(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if viewer != nil && !viewer.IsGuest() {
		return false, errors.New("you can't sign up while logged in")
	}

	// `approval` counts as allowed here. An applicant is starting an
	// application, not an account, and refusing them at this gate would make
	// the approval queue unreachable.
	if !p.Config.AllowsSelfServeRegistration() {
		return false, ErrRegistrationClosed
	}

	return true, nil
}
