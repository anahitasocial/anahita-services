// Command sweeper purges accounts whose deletion grace period has expired.
//
// Runs once and exits, so it can be scheduled as a Kubernetes CronJob, run from
// docker compose, or invoked by hand when somebody wants to know whether it
// works. The exit code is the answer: zero if the sweep completed, non-zero if
// it could not run.
//
// It is deliberately NOT a goroutine inside graph-grpc-service. That service
// runs three replicas in Kubernetes, so an in-process loop would put three
// concurrent sweeps on the same queue.
package main

import (
	"flag"
	"log"

	"anahita-libs/data"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"graph/config"
	"graph/data/repositories"
	"graph/sweeper"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// -tags-only runs the tag sweep alone, by hand, after a purge.
	tagsOnly := flag.Bool("tags-only", false, "remove unused hashtags and locations, and nothing else")
	flag.Parse()

	cfg := config.NewConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	conn := data.ConnectToDB(cfg.GetDSN())
	if conn == nil {
		log.Fatal("can't connect to MySQL!")
	}
	defer conn.Close()

	if *tagsOnly {
		swept, err := sweeper.RunTags(conn, repositories.NewRepositories(conn))
		if err != nil {
			log.Fatalf("sweeper: %v", err)
		}
		log.Printf("sweeper: finished, %d hashtag(s) and %d location(s) removed", swept.Hashtags, swept.Locations)
		return
	}

	// The publisher is optional. Without AMQP the sweep still purges content
	// and writes the tombstone; auth-service simply does not hear about it, and
	// its credential rows are cleaned up on the next purge event or by hand.
	// Losing the whole run because the broker is down would be the worse trade
	// — the accounts are past their deadline either way.
	var publisher *event.Publisher

	amqpConn, err := event.ConnectToAMQP(cfg)
	if err != nil {
		log.Println("sweeper: no AMQP connection, purging without publishing:", err)
	} else {
		defer amqpConn.Close()

		publisher, err = event.NewEventPublisher(amqpConn)
		if err != nil {
			log.Println("sweeper: no publisher, purging without publishing:", err)
			publisher = nil
		}
	}

	// The file client deletes uploaded avatars, covers and photos as accounts
	// are purged. Optional: without FILE_SERVICE_ADDR the sweep still clears the
	// database and leaves the objects, which is what happened before this
	// existed — better than refusing to purge accounts that are past their
	// deadline.
	var files services.FilesGRPCInterface
	if cfg.GetFileServiceAddr() == "" {
		log.Println("sweeper: FILE_SERVICE_ADDR unset, uploaded files will not be deleted")
	} else {
		files = services.NewFilesGRPC(cfg)
	}

	purged, err := sweeper.Run(conn, repositories.NewRepositories(conn), publisher, files)
	if err != nil {
		log.Fatalf("sweeper: %v", err)
	}

	log.Printf("sweeper: finished, %d account(s) purged", purged)
}
