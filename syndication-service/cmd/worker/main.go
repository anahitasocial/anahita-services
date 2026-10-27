// syndication-worker reads the feeds of websites and posts what is new in them as
// their notes, and checks every day that their domains are still proved.
package main

import (
	"anahita-libs/data"
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"
	"time"

	"syndication/config"
	"syndication/data/repositories"
	"syndication/feeds"
	"syndication/websub"
	"syndication/worker"

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

	db, err := data.Open(config.GetDSN(), data.Pool{MaxOpen: 4, MaxIdle: 2})
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	w := &worker.Worker{
		Websites: repositories.NewWebsites(db),
		Items:    repositories.NewItems(db),
		Domains:  repositories.NewDomains(db),
		Feeds:    feeds.NewReader(),
		Checker:  domains.NewChecker(),
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
		log.Println("syndication-worker: not reachable from the internet; feeds are read on the clock, without WebSub")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("syndication-worker: reading feeds")
	w.Run(ctx)
	log.Println("syndication-worker: stopped")
}

// graph is the worker's way to the graph, through graph-grpc-service.
type graph struct {
	actors services.ActorsGRPCInterface
	admins services.ActorAdminsGRPCInterface
	media  services.MediaGRPCInterface
}

// The website is read as nobody in particular: it is public.
func (g *graph) Website(id int64) (*entities.AnahitaNode, error) {
	return g.actors.ReadOfType(id, an_const.NodeActorWebsiteEntityType, entities.NewViewer())
}

// FirstAdmin is the administrator its notes are made as: the one who
// has been one longest.
func (g *graph) FirstAdmin(websiteID int64) (int64, error) {
	admins, err := g.admins.Browse(websiteID)
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
		return 0, errors.New("the Website has no administrator")
	}
	return first, nil
}

func (g *graph) Post(note *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	return g.media.Add(note, viewer)
}

func (g *graph) Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	return g.media.Read(id, viewer)
}

func (g *graph) Edit(note *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	return g.media.Edit(note, viewer)
}
