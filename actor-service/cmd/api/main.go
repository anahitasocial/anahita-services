package main

import (
	"anahita-libs/sessions"
	"fmt"
	"log"
	"net/http"

	"actor/config"
	"anahita-libs/constants"
	"anahita-libs/event"
	an_mw "anahita-libs/middleware"
	"anahita-libs/validators"

	"actor/cmd/api/handlers"
	"actor/cmd/api/permissions"
	"actor/data/services"

	"github.com/go-playground/validator/v10"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const webPort = "80"

func main() {
	var err error

	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit(constants.RequestBodyMaxSizeLimit))

	// set custom validator
	e.Validator = validators.NewRequestValidator(validator.New())

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

	if config.IsDev() {
		e.Debug = true
	}

	// JWKS client. One per process. Public keys are fetched from
	// auth-service's JWKS endpoint and cached, so access tokens are
	// verified locally on every request without a roundtrip.
	jwks := an_mw.NewJWKSClient(config.GetOAuthJWKSURL())

	// Pre-built middleware for use in routes. Authorize rejects
	// unauthenticated requests; SetViewer identifies the caller when a
	// token is present but lets anonymous requests through. Handlers
	// read the resulting viewer via an_mw.GetViewer.
	authorize := an_mw.Authorize(config, jwks)
	setViewer := an_mw.SetViewer(config, jwks)

	// create amqp connection
	amqpConn, err := event.ConnectToAMQP(config)
	if err != nil {
		log.Panic(err)
	}

	// create event publisher
	publisher, err := event.NewEventPublisher(amqpConn)
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

	// heartbeat
	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	// create routes
	NewRoutes(h, authorize, setViewer).Init(e)

	// start server
	err = e.Start(fmt.Sprintf(":%v", webPort))
	if err != nil {
		e.Logger.Fatal(err)
	}
}

// stepUpStore connects to Redis for the step-up marker, or returns nil.
//
// nil is a working state, not a failure: profiles and groups are served
// perfectly well without it, and refusing to start would take the whole
// service down over a guard. requireStepUp treats nil as "cannot verify" and
// refuses the guarded action, so losing Redis costs deletions rather than
// silently unguarding them.
func stepUpStore(cfg *config.Config) sessions.KeyValueInterface {
	store, err := sessions.NewKeyValueStore(cfg)
	if err != nil {
		log.Printf("step-up: no Redis (%v) — deleting a profile or a group will be refused "+
			"until it is reachable", err)
		return nil
	}
	return store
}
