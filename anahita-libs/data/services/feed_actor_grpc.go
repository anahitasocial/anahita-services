package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
)

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

// getClient returns the client, on the one connection to the service this
// process keeps (sharedConnection).
func (srv *FeedActorGRPC) getClient() (graphpb.FeedActorServiceClient, error) {
	conn, err := sharedConnection(srv.config.GetGraphServiceAddr())
	if err != nil {
		return nil, err
	}
	return graphpb.NewFeedActorServiceClient(conn), nil
}

// Count returns the total number of feed nodes from actor
func (srv *FeedActorGRPC) Count(req *FeedActorCountRequest, viewer *entities.Viewer) (uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return 0, err
	}

	resp, err := client.Count(context.Background(), &graphpb.FeedActorCountRequest{
		ActorId:           req.ActorID,
		Filter:            req.Filter,
		IncludeReposts:    req.IncludeReposts,
		IncludeLiked:      req.IncludeLiked,
		MediumEntityTypes: req.MediumEntityTypes,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
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
		Filter:            req.Filter,
		IncludeReposts:    req.IncludeReposts,
		IncludeLiked:      req.IncludeLiked,
		MediumEntityTypes: req.MediumEntityTypes,
		Limit:             req.Limit,
		Offset:            req.Offset,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
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
