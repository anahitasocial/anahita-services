package main

import (
	"anahita-libs/data"
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
	e.IPExtractor = an_mw.ClientIPExtractor()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit(constants.RequestBodyMaxSizeLimit))

	e.Validator = validators.NewRequestValidator(validator.New())

	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	if config.IsDev() {
		e.Debug = true
	}

	db, err := data.Open(config.GetDSN(), data.Pool{MaxOpen: 10, MaxIdle: 5})
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
