package main

import (
	"anahita-libs/data"
	"anahita-libs/sessions"

	"fmt"
	"log"
	"net/http"
	"video/data/services"

	_ "github.com/go-sql-driver/mysql"

	"anahita-libs/constants"
	an_mw "anahita-libs/middleware"
	"anahita-libs/validators"
	"video/cmd/api/handlers"
	"video/cmd/api/permissions"
	"video/config"

	"github.com/go-playground/validator/v10"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const webPort = "80"

func main() {
	e := echo.New()
	e.IPExtractor = an_mw.ClientIPExtractor()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	// A video is never sent here: only what is said about it.
	e.Use(middleware.BodyLimit(constants.RequestBodyMaxSizeLimit))

	// set custom validator
	e.Validator = validators.NewRequestValidator(validator.New())

	// create config
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

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

	// create handlers
	// The uploads are kept in the database. Without it nothing can be
	// begun or finished: the service does not start.
	db, err := data.Open(config.GetDSN(), data.Pool{MaxOpen: 10, MaxIdle: 5})
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	hEnv := &handlers.Env{
		Config:  config,
		Perms:   permissions.NewPermissions(),
		Limiter: sessions.NewSession(config),
		Srv:     services.NewServices(config, db),
	}
	h := handlers.NewHandlers(hEnv)

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
