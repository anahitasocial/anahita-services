package main

import (
	"anahita-libs/data"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/grpctls"
	"anahita-libs/proto/graphpb"
	"anahita-libs/proto/typespb"
	"fmt"
	"graph/cmd/server/handlers"
	"graph/config"
	"graph/data/repositories"
	"log"
	"net"

	"google.golang.org/grpc"
)

const (
	gRpcPort = "50051"
)

func main() {
	// if we crash the go code, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// create config
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	// connect to the database
	conn := data.ConnectToDB(config.GetDSN())
	if conn == nil {
		log.Panic("can't connect to MySQL!")
	}
	defer conn.Close()

	// create amqp connection
	amqpConn, err := event.ConnectToAMQP(config)
	if err != nil {
		log.Panic(err)
	}

	// create event publisher
	publisher, err := event.NewEventPublisher(amqpConn)
	if err != nil {
		log.Panic(err)
	}

	repos := repositories.NewRepositories(conn)

	// The file client, for the hard delete. Optional and nil-safe, the same
	// way the sweeper treats it: a purge without it clears the database and
	// leaves the uploaded objects, which beats refusing to purge.
	var files services.FilesGRPCInterface
	if config.GetFileServiceAddr() == "" {
		log.Println("FILE_SERVICE_ADDR unset, purged uploads will not be deleted")
	} else {
		files = services.NewFilesGRPC(config)
	}

	hEnv := &handlers.Env{
		Config:    config,
		DB:        conn,
		Repos:     repos,
		Publisher: publisher,
		Files:     files,
	}
	h := handlers.NewHandlers(hEnv)

	// NO SWEEPER LOOP HERE. Purging expired accounts is a job — see cmd/sweeper
	// and the CronJob in k8s. This service runs three replicas, so a loop here
	// would put three sweeps on the same queue.
	//
	// What does run here is one pass over the purge requests, started by the
	// request that added to them, under the sweeper's own lock. See
	// actorHandler.RequestPurge.

	// start the gRPC server
	if err := startGrpcServer(h); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func startGrpcServer(h *handlers.Handlers) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", gRpcPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Mutual TLS: only callers holding a certificate from this installation's
	// authority are served. Refuses to start without certificates unless
	// GRPC_TLS says otherwise.
	transport, err := grpctls.ServerOption()
	if err != nil {
		return err
	}

	s := grpc.NewServer(transport)

	graphpb.RegisterActorServiceServer(s, h.Actor)
	graphpb.RegisterActorAdminServiceServer(s, h.ActorAdmin)
	graphpb.RegisterFeedLeadersServiceServer(s, h.FeedLeaders)
	graphpb.RegisterFeedActorServiceServer(s, h.FeedActor)
	graphpb.RegisterFeedCustomServiceServer(s, h.FeedCustom)
	graphpb.RegisterEdgeServiceServer(s, h.Edge)
	graphpb.RegisterGraphServiceServer(s, h.Graph)
	graphpb.RegisterNodeServiceServer(s, h.Node)
	graphpb.RegisterLikeServiceServer(s, h.Like)
	graphpb.RegisterMediumServiceServer(s, h.Medium)
	graphpb.RegisterRepostServiceServer(s, h.Repost)
	graphpb.RegisterSearchServiceServer(s, h.Search)
	graphpb.RegisterSocialgraphServiceServer(s, h.Socialgraph)
	graphpb.RegisterSocialgraphFollowServiceServer(s, h.SocialgraphFollow)
	graphpb.RegisterEventServiceServer(s, h.Event)
	graphpb.RegisterSocialgraphBlockServiceServer(s, h.SocialgraphBlock)
	graphpb.RegisterSubscriptionServiceServer(s, h.Subscription)
	typespb.RegisterTypesServiceServer(s, h.Ping)

	log.Printf("gRPC server started at port %s", gRpcPort)

	if err := s.Serve(lis); err != nil {
		return err
	}

	return nil
}
