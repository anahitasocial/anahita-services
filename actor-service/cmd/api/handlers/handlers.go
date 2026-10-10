package handlers

import (
	"actor/cmd/api/permissions"
	"actor/config"
	"actor/data/services"
	"anahita-libs/event"
	"anahita-libs/sessions"
)

// Env is what every handler in this service works with: the same for people
// and for groups, which is the point of their being one service.
type Env struct {
	Config    *config.Config
	Srv       *services.Services
	Perms     *permissions.Permissions
	Publisher *event.Publisher

	// Session reads the step-up marker auth-service writes at /reauth/*.
	//
	// This service issues no proofs and never writes here; it only asks
	// whether one is live. Deleting a profile is credential-grade, and
	// deleting a group destroys a shared space that other people's posts live
	// in, so both ask for one.
	//
	// Optional: nil means the guard cannot verify, and requireStepUp refuses
	// rather than waving through. See the note there about which way that
	// fails.
	Session sessions.KeyValueInterface
}

// Handlers are the kinds of actor this service serves, and the administrators
// of the ones that have them.
//
// A new kind of actor is a handler here and its routes in routes.go. What
// they share (signing in the viewer, step-up, presigning images, the answers
// for a refused change) is already in this package.
type Handlers struct {
	Person *personHandler
	Group  *groupHandler
	Admin  *adminHandler
	// Events, and the administrators of events.
	Event      *eventHandler
	EventAdmin *adminHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Person:     NewPersonHandler(env),
		Group:      NewGroupHandler(env),
		Admin:      NewAdminHandler(env),
		Event:      NewEventHandler(env),
		EventAdmin: NewEventAdminHandler(env),
	}
}
