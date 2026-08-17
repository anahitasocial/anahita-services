package main

import (
	"anahita-libs/data"
	"anahita-libs/event"
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

	hEnv := &handlers.Env{
		Config:    config,
		Repos:     repositories.NewRepositories(conn),
		Publisher: publisher,
	}
	h := handlers.NewHandlers(hEnv)

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

	s := grpc.NewServer()

	graphpb.RegisterActorServiceServer(s, h.Actor)
	graphpb.RegisterActorAdminServiceServer(s, h.ActorAdmin)
	graphpb.RegisterFeedLeadersServiceServer(s, h.FeedLeaders)
	graphpb.RegisterFeedActorServiceServer(s, h.FeedActor)
	graphpb.RegisterEdgeServiceServer(s, h.Edge)
	graphpb.RegisterGraphServiceServer(s, h.Graph)
	graphpb.RegisterNodeServiceServer(s, h.Node)
	graphpb.RegisterLikeServiceServer(s, h.Like)
	graphpb.RegisterMediumServiceServer(s, h.Medium)
	graphpb.RegisterRepostServiceServer(s, h.Repost)
	graphpb.RegisterSearchServiceServer(s, h.Search)
	graphpb.RegisterSocialgraphServiceServer(s, h.Socialgraph)
	graphpb.RegisterSocialgraphFollowServiceServer(s, h.SocialgraphFollow)
	graphpb.RegisterSocialgraphBlockServiceServer(s, h.SocialgraphBlock)
	graphpb.RegisterSubscriptionServiceServer(s, h.Subscription)
	typespb.RegisterTypesServiceServer(s, h.Ping)

	log.Printf("gRPC server started at port %s", gRpcPort)

	if err := s.Serve(lis); err != nil {
		return err
	}

	return nil
}
