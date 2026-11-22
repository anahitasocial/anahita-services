package main

import (
	"anahita-libs/server"
	"anahita-libs/sessions"
	"log"

	"actor/config"
	"anahita-libs/constants"
	"anahita-libs/event"

	"actor/cmd/api/handlers"
	"actor/cmd/api/permissions"
	"actor/data/services"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	var err error

	// create config
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	// No database connection. This service reads and writes the graph over
	// gRPC and owns no tables, so it is given no database address at all.
	//
	// Worth keeping that way: a service holding a pool it never uses still
	// fails to start when the database is unreachable, and holds credentials
	// it has no use for.

	e := server.New(config.IsDev(), constants.RequestBodyMaxSizeLimit)

	// Who asks, from the token: authorize refuses a request with none,
	// setViewer lets it through.
	authorize, setViewer := server.SignIn(config)

	// The bus, connected to again whenever it closes the connection.
	publisher, err := event.NewPublisher(config)
	if err != nil {
		log.Panic(err)
	}

	// create handlers
	hEnv := &handlers.Env{
		Config:    config,
		Srv:       services.NewServices(config),
		Perms:     permissions.NewPermissions(),
		Publisher: publisher,
		Session:   stepUpStore(config),
	}
	h := handlers.NewHandlers(hEnv)

	// Start the event consumer
	go func() {
		if err = startEventListerner(h, config); err != nil {
			log.Panic(err)
		}
	}()

	// create routes
	NewRoutes(h, authorize, setViewer).Init(e)

	// Served until told to stop, then stopped gracefully.
	server.Run(e)
}

// stepUpStore connects to Valkey for the step-up marker, or returns nil.
//
// nil is a working state, not a failure: profiles and groups are served
// perfectly well without it, and refusing to start would take the whole
// service down over a guard. requireStepUp treats nil as "cannot verify" and
// refuses the guarded action, so losing Valkey costs deletions rather than
// silently unguarding them.
func stepUpStore(cfg *config.Config) sessions.KeyValueInterface {
	store, err := sessions.NewKeyValueStore(cfg)
	if err != nil {
		log.Printf("step-up: no Valkey (%v) — deleting a profile or a group will be refused "+
			"until it is reachable", err)
		return nil
	}
	return store
}
