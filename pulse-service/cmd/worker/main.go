// pulse-worker reads the feeds of Pulses and posts what is new in them as
// their articles, and checks every day that their domains are still proved.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os/signal"
	"syscall"
	"time"

	"pulse/config"
	"pulse/data/repositories"
	"pulse/feeds"
	"pulse/websub"
	"pulse/worker"

	an_const "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/domains"
	"anahita-libs/federation"
	"anahita-libs/safehttp"
	"anahita-libs/site"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	config := config.NewConfig()
	if err := config.ValidateWorker(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	db, err := connectToDB(config.GetDSN())
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	w := &worker.Worker{
		Pulses:  repositories.NewPulses(db),
		Items:   repositories.NewItems(db),
		Domains: repositories.NewDomains(db),
		Feeds:   feeds.NewReader(),
		Checker: domains.NewChecker(),
		Graph: &graph{
			actors: services.NewActorsGRPC(config),
			admins: services.NewActorAdminsGRPC(config),
			media:  services.NewMediaGRPC(config),
		},
		ReadEvery: time.Duration(config.ReadEvery) * time.Minute,
		HourlyCap: config.HourlyCap,
	}
	// Left nil when there is no address, so that no card is asked for.
	if cards := federation.NewCards(config.LinkPreviewURL); cards != nil {
		w.Cards = cards
	}

	// Hubs are asked to call back only where one could: an installation
	// reachable from the internet, over https, as for federation.
	addresses := site.FromEnv()
	if key := config.GetSecureCookieKey(); key != "" && addresses.CanFederate() == nil {
		client := safehttp.NewClient(safehttp.Options{Timeout: 10 * time.Second})
		w.Subscribe = func(ctx context.Context, hub, topic, callback string) error {
			return websub.Subscribe(ctx, client, hub, topic, callback)
		}
		w.Callback = func(nodeID int64) string {
			return websub.Callback(addresses.API, nodeID, key)
		}
	} else {
		log.Println("pulse-worker: not reachable from the internet; feeds are read on the clock, without WebSub")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("pulse-worker: reading feeds")
	w.Run(ctx)
	log.Println("pulse-worker: stopped")
}

// graph is the worker's way to the graph, through graph-grpc-service.
type graph struct {
	actors services.ActorsGRPCInterface
	admins services.ActorAdminsGRPCInterface
	media  services.MediaGRPCInterface
}

// The Pulse is read as nobody in particular: it is public.
func (g *graph) Pulse(id int64) (*entities.AnahitaNode, error) {
	return g.actors.ReadOfType(id, an_const.NodeActorPulseEntityType, entities.NewViewer())
}

// FirstAdmin is the administrator its articles are made as: the one who
// has been one longest.
func (g *graph) FirstAdmin(pulseID int64) (int64, error) {
	admins, err := g.admins.Browse(pulseID)
	if err != nil {
		return 0, err
	}
	var first int64
	for _, admin := range admins {
		if admin != nil && admin.ID > 0 && (first == 0 || admin.ID < first) {
			first = admin.ID
		}
	}
	if first == 0 {
		return 0, errors.New("the Pulse has no administrator")
	}
	return first, nil
}

func (g *graph) Post(article *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	return g.media.Add(article, viewer)
}

func (g *graph) Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	return g.media.Read(id, viewer)
}

func (g *graph) Edit(article *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	return g.media.Edit(article, viewer)
}

// connectToDB waits for MySQL, which may still be starting when this does.
func connectToDB(dsn string) (*sql.DB, error) {
	var lastErr error
	for attempt := 1; attempt <= 15; attempt++ {
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			if err = db.Ping(); err == nil {
				db.SetMaxOpenConns(4)
				db.SetMaxIdleConns(2)
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
