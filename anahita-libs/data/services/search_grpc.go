package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/grpctls"
	"anahita-libs/proto/graphpb"
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
)

var searchGRPCConnectCounts int

type SearchGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

// SearchGRPCInterface is the interface for search service
type SearchGRPCInterface interface {
	Count(req *SearchBrowseRequest, viewer *entities.Viewer) (uint64, error)
	Browse(req *SearchBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)
}

// NewSearchGRPC creates a new search service
func NewSearchGRPC(config SearchGRPCConfigInterface) SearchGRPCInterface {
	return &SearchGRPC{
		config: config,
	}
}

type SearchGRPC struct {
	config SearchGRPCConfigInterface
}

// GetClient returns the grpc client
func (srv *SearchGRPC) getClient() (graphpb.SearchServiceClient, error) {
	var opts []grpc.DialOption
	transport, err := grpctls.DialOption()
	if err != nil {
		return nil, err
	}
	opts = append(opts, transport)
	opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`))

	for {
		cc, err := grpc.NewClient(srv.config.GetGraphServiceAddr(), opts...)

		if err != nil {
			log.Println("Search GRPC not ready...")
			searchGRPCConnectCounts++
		} else {
			log.Println("Connected to Search GRPC!")
			return graphpb.NewSearchServiceClient(cc), nil
		}

		if searchGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// Count implements SearchGRPCInterface
func (srv *SearchGRPC) Count(req *SearchBrowseRequest, viewer *entities.Viewer) (uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return 0, err
	}
	if client == nil {
		return 0, nil
	}

	resp, err := client.Count(context.Background(), &graphpb.SearchCountRequest{
		Keywords:        req.Keywords,
		Scope:           req.Scope,
		IncludeComments: req.IncludeComments,
		CoordRadius:     req.CoordRadius,
		CoordLng:        req.CoordLng,
		CoordLat:        req.CoordLat,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
		AccessCheck: req.AccessCheck,
	})
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, nil
	}

	return resp.Total, nil
}

// Browse implements SearchGRPCInterface
func (srv *SearchGRPC) Browse(req *SearchBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.Browse(context.Background(), &graphpb.SearchBrowseRequest{
		Offset:          req.Offset,
		Limit:           req.Limit,
		Ordering:        req.Ordering,
		OrderingDir:     req.OrderingDir,
		Keywords:        req.Keywords,
		Scope:           req.Scope,
		IncludeComments: req.IncludeComments,
		CoordRadius:     req.CoordRadius,
		CoordLng:        req.CoordLng,
		CoordLat:        req.CoordLat,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
		AccessCheck: req.AccessCheck,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNodes(resp.Nodes), nil
}
