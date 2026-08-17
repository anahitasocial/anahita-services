package permissions

import (
	"anahita-libs/data/entities"
	"errors"
)

type SignupPermissions struct{}

func NewSignupPermissions(env *Env) *SignupPermissions {
	return &SignupPermissions{}
}

// CanAdd returns true if the viewer is allowed to add themselves (sign up)
func (p *SignupPermissions) CanAdd(viewer *entities.Viewer, entity interface{}) (bool, error) {
	if viewer == nil || viewer.IsGuest() {
		return true, nil
	}

	return false, errors.New("you can't sign up while logged in")
}
