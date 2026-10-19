package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
)

type FeedLeadersGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

// FeedLeadersGRPCInterface is the interface for feed leaders service
type FeedLeadersGRPCInterface interface {
	// Count returns the total number of feed nodes from leaders
	Count(req *FeedLeadersCountRequest, viewer *entities.Viewer) (uint64, error)

	// Browse returns a list of feed nodes from leaders
	Browse(req *FeedLeadersBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)

	// getClient returns the grpc client
	getClient() (graphpb.FeedLeadersServiceClient, error)
}

// NewFeedLeadersGRPC creates a new feed leaders service
func NewFeedLeadersGRPC(config FeedLeadersGRPCConfigInterface) FeedLeadersGRPCInterface {
	return &FeedLeadersGRPC{
		config: config,
	}
}

// FeedLeadersGRPC is the feed leaders service
type FeedLeadersGRPC struct {
	config FeedLeadersGRPCConfigInterface
}

// getClient returns the client, on the one connection to the service this
// process keeps (sharedConnection).
func (srv *FeedLeadersGRPC) getClient() (graphpb.FeedLeadersServiceClient, error) {
	conn, err := sharedConnection(srv.config.GetGraphServiceAddr())
	if err != nil {
		return nil, err
	}
	return graphpb.NewFeedLeadersServiceClient(conn), nil
}

// Count returns the total number of feed nodes from leaders
func (srv *FeedLeadersGRPC) Count(req *FeedLeadersCountRequest, viewer *entities.Viewer) (uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return 0, err
	}

	resp, err := client.Count(context.Background(), &graphpb.FeedLeadersCountRequest{
		IncludeReposts:    req.IncludeReposts,
		IncludeLiked:      req.IncludeLiked,
		LeaderEntityTypes: req.LeaderEntityTypes,
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
	if resp == nil {
		return 0, nil
	}

	return resp.Total, nil
}

// Browse returns a list of feed nodes from leaders
func (srv *FeedLeadersGRPC) Browse(req *FeedLeadersBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Browse(context.Background(), &graphpb.FeedLeadersBrowseRequest{
		IncludeReposts:    req.IncludeReposts,
		IncludeLiked:      req.IncludeLiked,
		LeaderEntityTypes: req.LeaderEntityTypes,
		MediumEntityTypes: req.MediumEntityTypes,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
		Offset: req.Offset,
		Limit:  req.Limit,
	})
	if err != nil {
		return nil, err
	}
	if resp.Nodes == nil {
		return []*entities.AnahitaNode{}, nil
	}

	return grpcToGraphNodes(resp.Nodes), nil
}
