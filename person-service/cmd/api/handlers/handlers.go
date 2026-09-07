package handlers

import (
	"anahita-libs/event"
	"anahita-libs/sessions"
	"person/cmd/api/permissions"
	"person/config"
	"person/data/repositories"
	"person/data/services"
)

type Env struct {
	Config    *config.Config
	Repos     *repositories.Repositories
	Srv       *services.Services
	Perms     *permissions.Permissions
	Publisher *event.Publisher

	// Session reads the step-up marker auth-service writes at /reauth/*.
	//
	// person-service does not issue proofs and never writes here; it only asks
	// whether one is live. Deleting a profile is credential-grade and was the
	// last such action gated on nothing but a session — a stolen unlocked
	// laptop deleted a profile.
	//
	// Optional. An installation without Redis reachable loses the gate rather
	// than losing deletion, and requireStepUp says so loudly. See the note
	// there about which way that fails.
	Session sessions.KeyValueInterface
}

type Handlers struct {
	Person *personHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Person: NewPersonHandler(env),
	}
}
