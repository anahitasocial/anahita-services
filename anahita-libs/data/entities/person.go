package entities

import (
	"anahita-libs/constants"
	"errors"
	"time"
)

type Person struct {
	AnahitaNode

	UserType      string    `json:"usertype"`
	Email         string    `json:"email"`
	Username      string    `json:"username"`
	Password      string    `json:"-"`
	LastVisitDate time.Time `json:"last_visit_date"`
	GivenName     string    `json:"given_name,omitempty"`
	FamilyName    string    `json:"family_name,omitempty"`
	Gender        string    `json:"gender,omitempty"`
}

// NewUser creates a new user
func NewPerson() *Person {
	person := &Person{}

	// Set the defaults
	person.UserType = constants.UserTypeGuest
	person.Type = constants.NodeActorPersonEntityType
	person.Service = constants.PersonService
	person.CreationTime = time.Now()
	person.UpdateTime = time.Now()
	person.LastVisitDate = time.Now()

	return person
}

// Validate validates the person
func (p *Person) Validate() error {
	if p.Username == "" {
		return errors.New("username is required")
	}

	// if p.Password == "" {
	// 	return errors.New("password is required")
	// }

	if p.Email == "" {
		return errors.New("email is required")
	}

	if p.UserType == "" {
		return errors.New("user type is required")
	}

	return nil
}

func (p *Person) IsGuest() bool {
	return p.UserType == constants.UserTypeGuest
}

func (p *Person) IsRegistered() bool {
	return p.UserType == constants.UserTypeRegistered
}

func (p *Person) IsAdmin() bool {
	return p.IsSuperAdmin() || p.UserType == constants.UserTypeAdmin
}

func (p *Person) IsSuperAdmin() bool {
	return p.UserType == constants.UserTypeSuperAdmin
}

func (p *Person) IsAdminOrSuperAdmin() bool {
	return p.IsAdmin() || p.IsSuperAdmin()
}
