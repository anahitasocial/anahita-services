// The export worker: makes the archives people have asked for. It answers
// no requests. See package worker.
package main

import (
	"anahita-libs/data"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"anahita-libs/data/services"
	xconfig "export/config"
	"export/data/repositories"
	"export/worker"

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
		},
		Files:     &worker.StoredFiles{Transfers: transfers},
		Transfers: transfers,
		Remover:   services.NewFilesGRPC(config),
		WebURL:    config.PublicWebURL,
		SiteName:  config.SiteName,
		Dir:       os.Getenv("EXPORT_WORK_DIR"),
		KeepFor:   time.Duration(config.KeepDays) * 24 * time.Hour,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("export worker: archives are kept %d days", config.KeepDays)
	w.Run(ctx)
}
