package permissions

import "anahita-libs/data/entities"

type ActorNodeInterface interface {
	GetID() int64
	GetAccess() string
	HasAddPermission(service string, entity string, access string) bool
	IsPerson() bool
}

type MediumNodeInterface interface {
	GetID() int64
	GetAuthorID() int64
	GetOwnerID() int64
	GetAccess() string
	GetOwner() *entities.AnahitaNode
	GetAuthor() *entities.AnahitaNode
	GetService() string
	GetEntityName() string
}
