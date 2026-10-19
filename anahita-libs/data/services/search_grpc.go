package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
)

type SearchGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

// SearchGRPCInterface is the interface for search service
type SearchGRPCInterface interface {
	Count(req *SearchBrowseRequest, viewer *entities.Viewer) (uint64, error)
	Browse(req *SearchBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)
	// The viewer's own steps for Most relevant, and keeping them.
	ReadPreferences(viewer *entities.Viewer) (map[string]float64, error)
	SavePreferences(viewer *entities.Viewer, steps map[string]float64) (map[string]float64, error)
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
// getClient returns the client, on the one connection to the service this
// process keeps (sharedConnection).
func (srv *SearchGRPC) getClient() (graphpb.SearchServiceClient, error) {
	conn, err := sharedConnection(srv.config.GetGraphServiceAddr())
	if err != nil {
		return nil, err
	}
	return graphpb.NewSearchServiceClient(conn), nil
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
		Keywords:    req.Keywords,
		Scope:       req.Scope,
		CoordRadius: req.CoordRadius,
		CoordLng:    req.CoordLng,
		CoordLat:    req.CoordLat,
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
		FollowedFirst: req.FollowedFirst,
		RecentFirst:   req.RecentFirst,
		Offset:        req.Offset,
		Limit:         req.Limit,
		Ordering:      req.Ordering,
		OrderingDir:   req.OrderingDir,
		Keywords:      req.Keywords,
		Scope:         req.Scope,
		CoordRadius:   req.CoordRadius,
		CoordLng:      req.CoordLng,
		CoordLat:      req.CoordLat,
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

// ReadPreferences implements SearchGRPCInterface.
func (srv *SearchGRPC) ReadPreferences(viewer *entities.Viewer) (map[string]float64, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return map[string]float64{}, nil
	}
	resp, err := client.ReadPreferences(context.Background(), &graphpb.SearchPreferencesReadRequest{
		Viewer: &graphpb.Viewer{Id: viewer.ID, PersonType: viewer.PersonType},
	})
	if err != nil {
		return nil, err
	}
	return resp.GetSteps(), nil
}

// SavePreferences implements SearchGRPCInterface.
func (srv *SearchGRPC) SavePreferences(viewer *entities.Viewer, steps map[string]float64) (map[string]float64, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return map[string]float64{}, nil
	}
	resp, err := client.SavePreferences(context.Background(), &graphpb.SearchPreferencesSaveRequest{
		Viewer: &graphpb.Viewer{Id: viewer.ID, PersonType: viewer.PersonType},
		Steps:  steps,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetSteps(), nil
}
