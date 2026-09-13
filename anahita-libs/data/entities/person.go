package entities

import (
	"anahita-libs/constants"
	"errors"
	"time"
)

type Person struct {
	AnahitaNode

	PersonType    string    `json:"usertype"`
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
	person.PersonType = constants.PersonTypeGuest
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

	if p.PersonType == "" {
		return errors.New("user type is required")
	}

	return nil
}

func (p *Person) IsGuest() bool {
	return p.PersonType == constants.PersonTypeGuest
}

func (p *Person) IsRegistered() bool {
	return p.PersonType == constants.PersonTypeRegistered
}

func (p *Person) IsAdmin() bool {
	return p.IsSuperAdmin() || p.PersonType == constants.PersonTypeAdmin
}

func (p *Person) IsSuperAdmin() bool {
	return p.PersonType == constants.PersonTypeSuperAdmin
}

func (p *Person) IsAdminOrSuperAdmin() bool {
	return p.IsAdmin() || p.IsSuperAdmin()
}
