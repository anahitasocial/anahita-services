package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"atproto/bsky"
	"atproto/cmd/api/handlers"
	"atproto/config"
	"atproto/data/repositories"

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

	// Where this installation is. Reading Bluesky asks nothing of it but
	// a way out: its name is used only to say who is asking, and to know
	// a handle of its own when one is typed.
	addresses := site.FromEnv()

	db, err := connectToDB(config.GetDSN())
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	jwks := an_mw.NewJWKSClient(config.GetOAuthJWKSURL())
	authorize := an_mw.Authorize(config, jwks)

	userAgent := "Anahita/1.0"
	if addresses.WebURL != "" {
		userAgent += " (+" + addresses.WebURL + ")"
	}
	client := bsky.NewClient(config.AppViewURL, userAgent)
	blocks := federation.NewBlocks(db)
	nodes := services.NewGraphNodeGRPC(config)

	// Sent on when who is followed on Bluesky may have changed. One
	// waiting is enough: what is asked then is who is followed now.
	changed := make(chan struct{}, 1)

	env := &handlers.Env{
		Site:          addresses,
		Nodes:         nodes,
		Actors:        repositories.NewActors(db),
		Bluesky:       client,
		Blocks:        blocks,
		Reposts:       services.NewRepostGRPC(config),
		RepostRecords: repositories.NewReposts(db),
		Keeper: &federation.Keeper{
			Nodes:      nodes,
			Files:      services.NewFilesGRPC(config),
			PhotoFiles: services.NewMediaGRPC(config),
			Pictures:   client,
			Refused:    blocks.IsBlocked,
		},
		Cards:    federation.NewCards(config.LinkPreviewURL),
		Announce: (&announcer{config: config}).Publish,
		Limiter:  sessions.NewSession(config),
		CDN:      config.CDNURL,
		Settle:   3 * time.Second,
		Changed: func() {
			select {
			case changed <- struct{}{}:
			default:
			}
		},
	}
	// Writing to Bluesky, as accounts people here connected. It needs a
	// key to keep their tokens under; without one the service reads
	// Bluesky and writes nothing.
	var queue *federation.Queue
	if config.CanKeepSecrets() {
		secrets, err := crypt.NewCrypt(config)
		if err != nil {
			log.Fatalf("the key that tokens are kept under: %v", err)
		}
		if queue, err = federation.NewQueue(db, "atproto_writes"); err != nil {
			log.Fatalf("the queue of what is to be written: %v", err)
		}
		env.Accounts = repositories.NewAccounts(db)
		env.Sent = repositories.NewSent(db)
		env.Writer = client
		env.Crypt = secrets
		env.Queue = queue
		env.Directory = config.DirectoryURL
	} else {
		log.Println("CRYPT_LOCAL_KEY is not set: Bluesky is read, and nothing is written to it")
	}

	h := handlers.NewHandlers(env)

	// `app-binary lookup alice.bsky.social`: look somebody up, print who
	// was found, and stop. See docs/federation.md.
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

	// From here on this is the service, answering requests: what need not
	// hold up an answer is done beside it.
	env.Later = func(work func()) { go work() }

	ctx := context.Background()

	// What is posted on Bluesky by whoever is followed from here.
	go listenToBluesky(ctx, db, &bsky.Stream{
		URLs:      config.JetstreamURLs,
		UserAgent: userAgent,
		Wanted:    h.Wanted,
		Changed:   changed,
		Cursor: func() int64 {
			at, err := repositories.NewCursor(db).Read()
			if err != nil {
				log.Println("where the listener was could not be read:", err)
			}
			return at
		},
		Save: func(at int64) {
			if err := repositories.NewCursor(db).Save(at); err != nil {
				log.Println("where the listener is could not be kept:", err)
			}
		},
		Handle: h.Heard,
	})

	// What waits to be written to Bluesky. Any number of copies may run:
	// a row is handed to one of them at a time.
	if queue != nil {
		go queue.Run(ctx, 2*time.Second, 10, h.Write)
	}

	// What the bus lost is looked for now and then.
	go h.RunCatchUp(ctx, time.Minute)

	// That somebody here followed somebody on Bluesky. Without the bus it
	// is found a minute later.
	go listenToBus(h, config)

	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})

	NewRoutes(h, authorize).Init(e)

	if err := e.Start(fmt.Sprintf(":%v", webPort)); err != nil {
		e.Logger.Fatal(err)
	}
}

// listenToBluesky keeps one listener going for as long as the service
// runs, in whichever copy of it has the turn.
//
// Only one copy listens: two would keep every post twice over, and each
// would move the other's place in what has been said. The rest wait, and
// one of them takes over when the one listening stops.
func listenToBluesky(ctx context.Context, db *sql.DB, stream *bsky.Stream) {
	const (
		// How long is waited before asking for the turn again, and before
		// listening again after a connection is lost.
		wait = 30 * time.Second
		// How often the turn is checked to be still held.
		check = 20 * time.Second
	)

	for ctx.Err() == nil {
		turn, err := repositories.TakeTurn(ctx, db)
		if err != nil || turn == nil {
			if err != nil {
				log.Println("the listener's turn could not be asked for:", err)
			}
			sleep(ctx, wait)
			continue
		}
		log.Println("this copy listens to Bluesky")

		for ctx.Err() == nil && turn.Held(ctx) {
			// Listening stops with the turn: a copy that has lost it and
			// does not know would listen beside whoever took it.
			listening, stop := context.WithCancel(ctx)
			go func() {
				ticker := time.NewTicker(check)
				defer ticker.Stop()
				for {
					select {
					case <-listening.Done():
						return
					case <-ticker.C:
						if !turn.Held(listening) {
							stop()
							return
						}
					}
				}
			}()

			err := stream.Run(listening)
			stop()

			switch {
			case errors.Is(err, bsky.ErrTurnOver):
				// Nothing wrong: on to the next Jetstream, at once.
			case errors.Is(err, bsky.ErrNobodyWanted):
				// Nobody to listen for: wait to be told that there is.
				select {
				case <-ctx.Done():
				case <-stream.Changed:
				}
			case ctx.Err() == nil:
				log.Printf("listening to Bluesky stopped (%v): beginning again in %s", err, wait)
				sleep(ctx, wait)
			}
		}

		turn.Give()
		log.Println("this copy no longer listens to Bluesky")
	}
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// announcer says things on the bus, connecting when it first has something
// to say and again after the bus has been away.
type announcer struct {
	config *config.Config

	mu        sync.Mutex
	conn      io.Closer
	publisher *event.Publisher
}

func (a *announcer) Publish(payload *event.EventPayload, name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.publisher == nil {
		conn, err := event.ConnectToAMQP(a.config)
		if err != nil {
			return err
		}
		publisher, err := event.NewEventPublisher(conn)
		if err != nil {
			_ = conn.Close()
			return err
		}
		a.conn, a.publisher = conn, publisher
	}

	err := a.publisher.Publish(payload, name)
	if err != nil {
		// Whatever went wrong, begin again next time.
		_ = a.conn.Close()
		a.conn, a.publisher = nil, nil
	}
	return err
}

// listenToBus hears what happens here, for as long as the bus is there,
// and comes back to it when it is not.
func listenToBus(h *handlers.Handlers, config *config.Config) {
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
