package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var feedActorGRPCConnectCounts int

type FeedActorGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

// FeedActorGRPCInterface is the interface for feed actor service
type FeedActorGRPCInterface interface {
	// Count returns the total number of feed nodes from actor
	Count(req *FeedActorCountRequest, viewer *entities.Viewer) (uint64, error)

	// Browse returns a list of feed nodes from actor
	Browse(req *FeedActorBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)

	// getClient returns the grpc client
	getClient() (graphpb.FeedActorServiceClient, error)
}

// NewFeedActorGRPC creates a new feed actor service
func NewFeedActorGRPC(config FeedActorGRPCConfigInterface) FeedActorGRPCInterface {
	return &FeedActorGRPC{
		config: config,
	}
}

// FeedActorGRPC is the feed actor service
type FeedActorGRPC struct {
	config FeedActorGRPCConfigInterface
}

// getClient returns the grpc client
func (srv *FeedActorGRPC) getClient() (graphpb.FeedActorServiceClient, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`))

	for {
		cc, err := grpc.NewClient(srv.config.GetGraphServiceAddr(), opts...)

		if err != nil {
			log.Println("Feed Actor GRPC not ready...")
			feedActorGRPCConnectCounts++
		} else {
			log.Println("Connected to FeedActor GRPC!")
			return graphpb.NewFeedActorServiceClient(cc), nil
			// return srv.client, nil
		}

		if actorsGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// Count returns the total number of feed nodes from actor
func (srv *FeedActorGRPC) Count(req *FeedActorCountRequest, viewer *entities.Viewer) (uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return 0, err
	}

	resp, err := client.Count(context.Background(), &graphpb.FeedActorCountRequest{
		ActorId:           req.ActorID,
		IncludeComments:   req.IncludeComments,
		IncludeReposts:    req.IncludeReposts,
		IncludeLiked:      req.IncludeLiked,
		MediumEntityTypes: req.MediumEntityTypes,
		Viewer: &graphpb.Viewer{
			Id:       viewer.ID,
			UserType: viewer.UserType,
			Email:    viewer.Email,
			Username: viewer.Username,
		},
	})

	if err != nil {
		return 0, err
	}

	return resp.Total, nil
}

// Browse returns a list of feed nodes from actor
func (srv *FeedActorGRPC) Browse(req *FeedActorBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Browse(context.Background(), &graphpb.FeedActorBrowseRequest{
		ActorId:           req.ActorID,
		IncludeComments:   req.IncludeComments,
		IncludeReposts:    req.IncludeReposts,
		IncludeLiked:      req.IncludeLiked,
		MediumEntityTypes: req.MediumEntityTypes,
		Limit:             req.Limit,
		Offset:            req.Offset,
		Viewer: &graphpb.Viewer{
			Id:       viewer.ID,
			UserType: viewer.UserType,
			Email:    viewer.Email,
			Username: viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return []*entities.AnahitaNode{}, nil
	}

	return grpcToGraphNodes(resp.Nodes), nil
}
