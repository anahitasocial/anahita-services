// The video worker: makes posts of the videos people have uploaded. It
// answers no requests. See package worker.
package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"anahita-libs/data/services"
	vconfig "video/config"
	"video/data/repositories"
	"video/worker"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	config := vconfig.NewConfig()
	if err := config.ValidateWorker(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	// Found now, and not when the first video is waiting.
	for _, program := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(program); err != nil {
			log.Fatalf("%s is not installed: %v", program, err)
		}
	}

	db, err := connectToDB(config.GetDSN())
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	uploads := repositories.NewUploads(db)
	// A video is another worker's to take once this one must have died:
	// after longer than the work on one may last.
	heights := vconfig.Heights()
	uploads.ClaimFor = worker.TimeoutFor(config.VideoMaxSeconds, heights) + 15*time.Minute

	w := &worker.Worker{
		Uploads:    uploads,
		Transfers:  services.NewFileTransfersGRPC(config),
		Files:      services.NewFilesGRPC(config),
		Posts:      services.NewMediaGRPC(config),
		Owners:     services.NewActorsGRPC(config),
		Encoder:    worker.FFmpeg{},
		Dir:        os.Getenv("VIDEO_WORK_DIR"),
		Heights:    heights,
		MaxBytes:   config.VideoMaxBytes,
		MaxSeconds: config.VideoMaxSeconds,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("video worker: sizes %v, up to %d seconds and %d bytes", w.Heights, w.MaxSeconds, w.MaxBytes)
	w.Run(ctx)
}

// connectToDB waits for MySQL, which may still be starting when this does.
func connectToDB(dsn string) (*sql.DB, error) {
	var lastErr error

	for attempt := 1; attempt <= 15; attempt++ {
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			if err = db.Ping(); err == nil {
				db.SetMaxOpenConns(4)
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
