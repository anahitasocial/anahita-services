// The video worker: makes posts of the videos people have uploaded. It
// answers no requests. See package worker.
package main

import (
	"anahita-libs/data"
	"context"
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

	"anahita-libs/data/entities"
	"anahita-libs/handlers"

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

	db, err := data.Open(config.GetDSN(), data.Pool{MaxOpen: 4})
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	uploads := repositories.NewUploads(db)
	// A video is another worker's to take once this one must have died:
	// after longer than the work on one may last.
	heights := vconfig.Heights()
	uploads.ClaimFor = worker.TimeoutFor(config.VideoMaxSeconds, heights) + 15*time.Minute

	w := &worker.Worker{
		Uploads:   uploads,
		Transfers: services.NewFileTransfersGRPC(config),
		Files:     services.NewFilesGRPC(config),
		Posts:     services.NewMediaGRPC(config),
		Owners:    services.NewActorsGRPC(config),
		Replies: replies{handlers.ThreadReaders{
			Media:       services.NewMediaGRPC(config),
			Actors:      services.NewActorsGRPC(config),
			Socialgraph: services.NewSocialgraphGRPC(config),
		}},
		Encoder:             worker.FFmpeg{},
		Dir:                 os.Getenv("VIDEO_WORK_DIR"),
		Heights:             heights,
		MaxBytes:            config.VideoMaxBytes,
		MaxSeconds:          config.VideoMaxSeconds,
		AnimationMaxBytes:   config.AnimationMaxBytes,
		AnimationMaxSeconds: config.AnimationMaxSeconds,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("video worker: sizes %v, up to %d seconds and %d bytes", w.Heights, w.MaxSeconds, w.MaxBytes)
	w.Run(ctx)
}

// replies asks the graph whether somebody may answer a post now.
type replies struct {
	readers handlers.ThreadReaders
}

func (r replies) MayReplyTo(parentID int64, viewer *entities.Viewer) (bool, error) {
	return handlers.MayReplyTo(r.readers, parentID, viewer)
}
