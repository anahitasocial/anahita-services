package main

import (
	"database/sql"
	"time"

	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"net/http"
	"video/data/services"

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
	db, err := connectToDB(config.GetDSN())
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	hEnv := &handlers.Env{
		Config: config,
		Perms:  permissions.NewPermissions(),
		Srv:    services.NewServices(config, db),
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

// connectToDB waits for MySQL, which may still be starting when this does.
func connectToDB(dsn string) (*sql.DB, error) {
	var lastErr error

	for attempt := 1; attempt <= 15; attempt++ {
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			if err = db.Ping(); err == nil {
				db.SetMaxOpenConns(10)
				db.SetMaxIdleConns(5)
				db.SetConnMaxLifetime(5 * time.Minute)
				return db, nil
			}
			_ = db.Close()
		}

		lastErr = err
		log.Printf("database not ready (attempt %d): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}

	return nil, lastErr
}
