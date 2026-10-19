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
	"strconv"
	"strings"
	"sync"
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
		Objects:     repositories.NewObjects(db),
		Blocks:      repositories.NewBlocks(db),
		Missed:      repositories.NewMissed(db),
		FollowEdges: services.NewSocialgraphFollowGRPC(config),
		Likes:       services.NewLikesGRPC(config),
		Reposts:     services.NewRepostGRPC(config),
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
	env.Lab = config.Lab
	env.Pictures = client
	env.Files = services.NewFilesGRPC(config)
	env.Cards = handlers.NewCards(config.LinkPreviewURL)
	env.PhotoFiles = services.NewMediaGRPC(config)
	// The locations a post was tagged with, which go out with it.
	env.Locations = services.NewGraphGRPC(config)
	env.Announce = (&announcer{config: config}).Publish

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

	// `app-binary reread https://mastodon.example/users/alice/statuses/1`:
	// read a post that is kept here again from its own server, and keep
	// what it says now. For a post that was kept before something about
	// keeping posts was put right. See docs/federation.md.
	if len(os.Args) == 3 && os.Args[1] == "reread" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := h.RereadFromTheCommandLine(ctx, os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "not read again:", err)
			os.Exit(1)
		}
		fmt.Println("read again and kept")
		return
	}

	// For the lab only: `app-binary lab-follow|lab-unfollow|lab-post|
	// lab-unpost …` do what somebody here would do by pressing a button. See
	// handlers/lab.go.
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "lab-") {
		if !config.Lab {
			log.Fatal(os.Args[1], " is for the federation lab: ACTIVITYPUB_LAB is not on")
		}
		if err := lab(h, config, os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "not done:", err)
			os.Exit(1)
		}
		return
	}

	// From here on this is the service, answering requests: what need not
	// hold up an answer is done beside it. The commands above are done by
	// then, having waited for all of their own work.
	env.Later = func(work func()) { go work() }

	// The pictures of posts from other servers that have been left alone
	// for a while are removed from storage, to be copied again when asked
	// for. Any copy may do it: what one has removed the next does not find.
	go handlers.RunPictureSweep(context.Background(), time.Hour, func() int {
		return h.PictureKeeper().RemoveOldPictures(db, constants.NodeActorActivityPubPersonEntityType, time.Now(), 200)
	})

	// And what nobody here holds to goes altogether: the posts of
	// somebody nobody follows any more, and then whoever nothing is left
	// of. See anahita-libs/federation/retention.go for the rules.
	go handlers.RunTidy(context.Background(), time.Hour, func() (int, int) {
		rules, now := federation.RetentionFromEnv(), time.Now()
		posts, err := federation.OldPosts(db, constants.NodeActorActivityPubPersonEntityType, rules, now, 200)
		if err != nil {
			log.Println("looking for posts that are due to go:", err)
		}
		removed, _ := h.Tidy(posts, nil)
		// After the posts, so that whoever has just been left with
		// none is found in the same round.
		people, err := federation.StalePeople(db, constants.NodeActorActivityPubPersonEntityType, rules, now, 200)
		if err != nil {
			log.Println("looking for people of whom nothing is left:", err)
		}
		_, forgotten := h.Tidy(nil, people)
		return removed, forgotten
	})

	// What the bus lost is looked for now and then. See handlers/sweep.go.
	go h.RunSweep(context.Background(), 5*time.Minute)

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

// lab runs one of the lab's commands.
func lab(h *handlers.Handlers, config *config.Config, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	switch {
	case args[0] == "lab-follow" && len(args) == 3:
		return h.LabFollow(ctx, args[1], args[2], true)
	case args[0] == "lab-unfollow" && len(args) == 3:
		return h.LabFollow(ctx, args[1], args[2], false)
	case args[0] == "lab-join" && len(args) == 2:
		return h.LabJoin(args[1])
	case args[0] == "lab-leave" && len(args) == 2:
		return h.LabLeave(ctx, args[1])
	case args[0] == "lab-post" && len(args) == 3:
		id, err := h.LabPost(services.NewMediaGRPC(config), args[1], args[2])
		if err == nil {
			fmt.Println("posted as note", id)
		}
		return err
	case args[0] == "lab-unpost" && len(args) == 2:
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return err
		}
		return h.LabUnpost(services.NewMediaGRPC(config), id)
	}
	return errors.New("lab-join <name here>, lab-leave <name here>, lab-follow <name here> <handle>, lab-unfollow <name here> <handle>, lab-post <name here> <text>, lab-unpost <note id>")
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
