package entities

import (
	"anahita-libs/constants"
	"time"
)

// Person is an actor node of the person type.
//
// It carries no fields of its own any more. Every one it used to declare —
// PersonType, Email, Username, GivenName, FamilyName, Gender, LastVisitDate —
// was a copy of a people_people column read by person-service and merged onto
// the embedded node at request time.
//
// Those copies were worse than redundant once person_type, name, alias and
// last_visit_date moved onto the node: an outer field of the same name SHADOWS
// the embedded one, so assigning the node left the outer copy empty and every
// reader of `person.PersonType` got "". The type stays as a name for "an actor
// that is a person", which is what the handlers and the response mapper talk
// about.
type Person struct {
	AnahitaNode
}

// NewPerson returns an empty person node with the type defaults set.
func NewPerson() *Person {
	person := &Person{}

	// Set the defaults
	person.PersonType = constants.PersonTypeGuest
	person.Type = constants.NodeActorPersonEntityType
	person.Service = constants.PersonService
	person.CreationTime = time.Now()
	person.UpdateTime = time.Now()

	return person
}

// Validate is gone with the fields it checked.
//
// It required a username, an email and a usertype — all of them people_people
// columns. The node's own validation lives in graph-grpc-service, next to the
// insert it guards.

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
