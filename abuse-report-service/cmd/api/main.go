package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"abusereport/cmd/api/handlers"
	"abusereport/cmd/api/permissions"
	"abusereport/config"
	an_abuse "abusereport/constants"
	"abusereport/data/reasons"
	"abusereport/data/repositories"
	"abusereport/data/services"

	"anahita-libs/constants"
	an_mw "anahita-libs/middleware"
	"anahita-libs/sessions"
	"anahita-libs/validators"

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
	e.Use(middleware.BodyLimit(constants.RequestBodyMaxSizeLimit))

	// The address a report came from is recorded with it. Taken from
	// X-Forwarded-For, read from the right and skipping the proxies, so an
	// entry a client typed is never used.
	e.IPExtractor = echo.ExtractIPFromXFFHeader()

	e.Validator = validators.NewRequestValidator(validator.New())

	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	if config.IsDev() {
		e.Debug = true
	}

	db, err := connectToDB(config.GetDSN())
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	jwks := an_mw.NewJWKSClient(config.GetOAuthJWKSURL())
	authorize := an_mw.Authorize(config, jwks)

	hEnv := &handlers.Env{
		Config:  config,
		Perms:   permissions.NewPermissions(),
		Repos:   repositories.NewRepositories(db),
		Srv:     services.NewServices(config),
		Reasons: reasons.NewSource(config.GetReasonsPath()),
		Limiter: sessions.NewSession(config),
	}
	h := handlers.NewHandlers(hEnv)

	// Fail at start-up, not at the first report, if the installation's own
	// reasons file is unusable. The service still runs, on the built-in
	// list, and the log has already said why.
	log.Printf("reasons: %d in force", len(hEnv.Reasons.Catalogue().Reasons))

	go func() {
		if err := startEventListener(h, config); err != nil {
			log.Panic(err)
		}
	}()

	go forgetOldAddresses(hEnv.Repos)

	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	NewRoutes(h, authorize).Init(e)

	if err := e.Start(fmt.Sprintf(":%v", webPort)); err != nil {
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

// forgetOldAddresses blanks the network address on reports older than
// AddressRetentionDays, at start-up and then hourly.
//
// It runs in every replica. They all issue the same statement, which changes
// nothing the second time, so there is nothing to coordinate.
func forgetOldAddresses(repos *repositories.Repositories) {
	for {
		cutoff := time.Now().AddDate(0, 0, -an_abuse.AddressRetentionDays)

		blanked, err := repos.Cases.ForgetAddresses(cutoff)
		if err != nil {
			log.Println("abuse report: could not blank old network addresses:", err)
		} else if blanked > 0 {
			log.Printf("abuse report: blanked the network address on %d report(s) older than %d days",
				blanked, an_abuse.AddressRetentionDays)
		}

		time.Sleep(time.Hour)
	}
}
