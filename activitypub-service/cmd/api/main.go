package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"activitypub/cmd/api/handlers"
	"activitypub/config"
	"activitypub/data/repositories"
	"activitypub/fetch"

	"anahita-libs/constants"
	"anahita-libs/crypt"
	"anahita-libs/data/services"
	an_mw "anahita-libs/middleware"
	"anahita-libs/sessions"
	"anahita-libs/site"
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
	e.IPExtractor = echo.ExtractIPFromXFFHeader()
	e.Validator = validators.NewRequestValidator(validator.New())

	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}
	if config.IsDev() {
		e.Debug = true
	}

	// Where this installation is, as other servers will be told. Said once
	// at the start: an address that changed between two requests would make
	// one person two.
	addresses := site.FromEnv()
	if err := addresses.CanFederate(); err != nil {
		log.Printf("this installation is not on the fediverse: %v", err)
	} else {
		log.Printf("on the fediverse as %s, reached at %s", addresses.Domain, addresses.APIURL)
	}

	// Fails here, at the start, on a key that is wrong: not in the middle
	// of somebody turning federation on.
	secrets, err := crypt.NewCrypt(config)
	if err != nil {
		log.Fatalf("the key that signing keys are kept under: %v", err)
	}

	db, err := connectToDB(config.GetDSN())
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	jwks := an_mw.NewJWKSClient(config.GetOAuthJWKSURL())
	authorize := an_mw.Authorize(config, jwks)
	setViewer := an_mw.SetViewer(config, jwks)

	env := &handlers.Env{
		Site:     addresses,
		Nodes:    services.NewGraphNodeGRPC(config),
		Accounts: repositories.NewAccounts(db),
		Keys:     repositories.NewKeys(db),
		Actors:   repositories.NewActors(db),
		Crypt:    secrets,
		Limiter:  sessions.NewSession(config),
	}
	h := handlers.NewHandlers(env)

	// What reads from other servers, signed as the installation. It goes
	// only to public addresses, except in the lab, where every server is
	// at a private one.
	userAgent := "Anahita/1.0 (+" + addresses.WebURL + ")"
	if config.Lab {
		log.Println("ACTIVITYPUB_LAB is on: private addresses are fetched from. For the federation lab only.")
		env.Fetcher = fetch.NewWith(userAgent, h.SignAsInstance, fetch.LabAddresses)
	} else {
		env.Fetcher = fetch.New(userAgent, h.SignAsInstance)
	}

	// `app-binary lookup @alice@example.net`: look somebody up as the
	// installation, print who was found, and stop. See docs/federation.md.
	if len(os.Args) == 3 && os.Args[1] == "lookup" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		found, err := h.LookupFromTheCommandLine(ctx, os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "not found:", err)
			os.Exit(1)
		}
		out, _ := json.MarshalIndent(found, "", "  ")
		fmt.Println(string(out))
		return
	}

	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	NewRoutes(h, authorize, setViewer).Init(e)

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
