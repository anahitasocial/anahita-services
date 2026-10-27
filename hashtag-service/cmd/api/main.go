package main

import (
	"fmt"
	"log"
	"net/http"

	"anahita-libs/constants"
	an_mw "anahita-libs/middleware"
	"anahita-libs/validators"
	"hashtag/cmd/api/handlers"
	"hashtag/cmd/api/permissions"
	"hashtag/config"
	"hashtag/data/services"

	"github.com/go-playground/validator/v10"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const webPort = "80"

func main() {
	var err error

	e := echo.New()
	e.IPExtractor = an_mw.ClientIPExtractor()
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
	hEnv := &handlers.Env{
		Config: config,
		Perms:  permissions.NewPermissions(),
		Srv:    services.NewServices(config),
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
	if err = e.Start(fmt.Sprintf(":%v", webPort)); err != nil {
		e.Logger.Fatal(err)
	}
}
