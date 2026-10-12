package main

import (
	"fmt"
	"log"
	"net/http"

	"anahita-libs/constants"
	an_mw "anahita-libs/middleware"
	"anahita-libs/validators"
	"identityimage/cmd/api/handlers"
	"identityimage/cmd/api/permissions"
	"identityimage/config"
	"identityimage/data/services"

	"github.com/go-playground/validator/v10"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	webPort = "80"
)

func main() {
	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit(constants.ImageRequestBodyMaxSizeLimit))

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
	// unauthenticated requests; the viewer it builds from the token
	// claims is read by handlers via an_mw.GetViewer.
	authorize := an_mw.Authorize(config, jwks)
	// For the two addresses anybody may ask: it is what refuses them on a
	// site for members only.
	setViewer := an_mw.SetViewer(config, jwks)

	// create handlers
	hEnv := &handlers.Env{
		Config: config,
		Srv:    services.NewServices(config),
		Perms:  permissions.NewPermissions(),
	}
	h := handlers.NewHandlers(hEnv)

	// heartbeat
	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	// create routes
	NewRoutes(h, authorize, setViewer).Init(e)

	// start server
	err := e.Start(fmt.Sprintf(":%v", webPort))
	if err != nil {
		e.Logger.Fatal(err)
	}
}
