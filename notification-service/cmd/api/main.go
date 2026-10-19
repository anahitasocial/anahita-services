package main

import (
	"fmt"
	"log"
	"net/http"

	"anahita-libs/constants"
	"anahita-libs/data"
	an_mw "anahita-libs/middleware"
	"anahita-libs/validators"
	"notification/cmd/api/handlers"
	"notification/cmd/api/permissions"
	"notification/config"
	"notification/data/repositories"
	"notification/data/services"

	"github.com/go-playground/validator/v10"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	webPort  = "80"
	gRpcPort = "50051"
)

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

	// connect to the database
	conn := data.ConnectToDB(config.GetDSN())
	if conn == nil {
		log.Panic("can't connect to MySQL!")
	}
	defer conn.Close()

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

	// create handlers
	hEnv := &handlers.Env{
		Config: config,
		Repos:  repositories.NewRepositories(conn),
		Srv:    services.NewServices(config),
		Perms:  permissions.NewPermissions(),
	}
	h := handlers.NewHandlers(hEnv)

	// Start the event consumer
	go func() {
		if err = startEventListerner(h, config); err != nil {
			log.Panic(err)
		}
	}()

	// Read notifications past keeping go (NOTIFICATIONS_KEEP_DAYS).
	go keepNotifications(hEnv.Repos.Notifications, config.KeepDays)

	// heartbeat
	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	// create routes
	NewRoutes(h, authorize).Init(e)

	// start server
	err = e.Start(fmt.Sprintf(":%v", webPort))
	if err != nil {
		e.Logger.Fatal(err)
	}
}
