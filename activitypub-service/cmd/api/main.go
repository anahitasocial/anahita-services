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
	"anahita-libs/event"
	"anahita-libs/federation"
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
		Site:        addresses,
		Nodes:       services.NewGraphNodeGRPC(config),
		Accounts:    repositories.NewAccounts(db),
		Keys:        repositories.NewKeys(db),
		Actors:      repositories.NewActors(db),
		Follows:     repositories.NewFollows(db),
		FollowEdges: services.NewSocialgraphFollowGRPC(config),
		Relations:   services.NewSocialgraphGRPC(config),
		Crypt:       secrets,
		Limiter:     sessions.NewSession(config),
	}
	h := handlers.NewHandlers(env)

	// What reads from other servers, signed as the installation. It goes
	// only to public addresses, except in the lab, where every server is
	// at a private one.
	userAgent := "Anahita/1.0 (+" + addresses.WebURL + ")"
	client := fetch.New(userAgent, h.SignAsInstance)
	if config.Lab {
		log.Println("ACTIVITYPUB_LAB is on: private addresses are fetched from. For the federation lab only.")
		client = fetch.NewWith(userAgent, h.SignAsInstance, fetch.LabAddresses)
	}
	env.Fetcher = client
	env.Poster = client

	// What is to be sent to other servers waits in a table, and is sent
	// from there until it arrives or two days have passed.
	queue, err := federation.NewQueue(db, "activitypub_deliveries")
	if err != nil {
		log.Fatalf("the queue of what is to be sent: %v", err)
	}
	env.Queue = queue

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

	// For the lab only: `app-binary lab-follow <username here> <handle
	// elsewhere>` makes somebody here follow somebody on another server, as
	// pressing Follow would. It is how following is tried before the web
	// app can show somebody from another server.
	if len(os.Args) == 4 && os.Args[1] == "lab-follow" {
		if !config.Lab {
			log.Fatal("lab-follow is for the federation lab: ACTIVITYPUB_LAB is not on")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := h.LabFollow(ctx, os.Args[2], os.Args[3], true); err != nil {
			fmt.Fprintln(os.Stderr, "not followed:", err)
			os.Exit(1)
		}
		fmt.Println("followed here; the other server is asked by the running service")
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "lab-unfollow" {
		if !config.Lab {
			log.Fatal("lab-unfollow is for the federation lab: ACTIVITYPUB_LAB is not on")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := h.LabFollow(ctx, os.Args[2], os.Args[3], false); err != nil {
			fmt.Fprintln(os.Stderr, "not unfollowed:", err)
			os.Exit(1)
		}
		fmt.Println("unfollowed here; the other server is told by the running service")
		return
	}

	// Any number of copies of this service may run: a row of the queue is
	// handed to one of them at a time.
	go queue.Run(context.Background(), 5*time.Second, 10, h.Deliver)

	// What happens here that another server has to be told. Without the
	// bus the service still answers and is still followed; it is following
	// from here that waits.
	go listen(h, config)

	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	NewRoutes(h, authorize, setViewer).Init(e)

	if err := e.Start(fmt.Sprintf(":%v", webPort)); err != nil {
		e.Logger.Fatal(err)
	}
}

// listen hears what happens here, for as long as the bus is there, and
// comes back to it when it is not.
func listen(h *handlers.Handlers, config *config.Config) {
	for {
		conn, err := event.ConnectToAMQP(config)
		if err == nil {
			var consumer event.Consumer
			if consumer, err = event.NewConsumer(conn); err == nil {
				log.Println("listening for what happens here")
				err = consumer.Listen(handlers.EventTopics, h.HandleEvent)
			}
			_ = conn.Close()
		}
		log.Printf("the bus is not there (%v): trying again in ten seconds", err)
		time.Sleep(10 * time.Second)
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
