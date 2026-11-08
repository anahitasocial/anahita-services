// The export worker: makes the archives people have asked for. It answers
// no requests. See package worker.
package main

import (
	"anahita-libs/data"
	"anahita-libs/servicecalls"
	"anahita-libs/site"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"anahita-libs/data/services"
	xconfig "importexport/config"
	"importexport/data/repositories"
	"importexport/worker"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	config := xconfig.NewConfig()
	if err := config.ValidateWorker(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	db, err := data.Open(config.GetDSN(), data.Pool{MaxOpen: 4})
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	transfers := services.NewFileTransfersGRPC(config)
	w := &worker.Worker{
		Exports: repositories.NewExports(db),
		Source: &worker.GraphSource{
			Actors:  services.NewActorsGRPC(config),
			Written: services.NewAuthoredGRPC(config),
			Media:   services.NewMediaGRPC(config),
			Graph:   services.NewGraphGRPC(config),
			Edges:   services.NewGraphEdgeGRPC(config),
		},
		Files:     &worker.StoredFiles{Transfers: transfers},
		Transfers: transfers,
		Remover:   services.NewFilesGRPC(config),
		WebURL:    config.PublicWebURL,
		SiteName:  config.SiteName,
		Site:      site.FromEnv(),
		Dir:       os.Getenv("EXPORT_WORK_DIR"),
		KeepFor:   time.Duration(config.KeepDays) * 24 * time.Hour,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Imports, beside the exports (roadmap row 124): a line a second.
	calls, err := servicecalls.NewClient(time.Minute)
	if err != nil {
		log.Fatalf("the certificates for calling the other services: %v", err)
	}
	importer := &worker.Importer{
		Imports: repositories.NewImports(db),
		Acts: &worker.GraphActs{
			Site:        site.FromEnv(),
			Calls:       calls,
			Nodes:       services.NewGraphNodeGRPC(config),
			Actors:      services.NewActorsGRPC(config),
			Relations:   services.NewSocialgraphGRPC(config),
			Follows:     services.NewSocialgraphFollowGRPC(config),
			Blocks:      services.NewSocialgraphBlockGRPC(config),
			Mutes:       services.NewSocialgraphMuteGRPC(config),
			Media:       services.NewMediaGRPC(config),
			ActivityPub: hostOr("ACTIVITYPUB_INTERNAL_HOST", "activitypub-service"),
			ATProto:     hostOr("ATPROTO_INTERNAL_HOST", "atproto-service"),
		},
		Pace: time.Second,
		Idle: 5 * time.Second,
	}
	go importer.Run(ctx)

	log.Printf("export worker: archives are kept %d days", config.KeepDays)
	w.Run(ctx)
}

// hostOr is a service's host from the environment, or its name in the
// cluster and in docker-compose.
func hostOr(name, otherwise string) string {
	if host := os.Getenv(name); host != "" {
		return host
	}
	return otherwise
}
