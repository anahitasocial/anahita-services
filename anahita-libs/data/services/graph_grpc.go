package services

import (
	"anahita-libs/data/dtos"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var graphGRPCConnectCounts int

type GraphGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

// GraphGRPCInterface is the interface for graph service
type GraphGRPCInterface interface {
	// BrowseNeighbours fetches neighboring nodes for a given node ID.
	BrowseNeighbours(req *GrapBrowseNeighboursRequest) ([]*entities.AnahitaNode, uint64, error)

	// BrowseConnectionNodes fetches nodes connected by a specific edge type.
	BrowseConnectionNodes(req *GraphBrowseConnectionNodesRequest) ([]*entities.AnahitaNode, uint64, error)

	// BrowseOrderedByDegree fetches nodes ordered by their degree.
	BrowseOrderedByDegree(req *GraphBrowseOrderedByDegreeRequest) ([]*entities.AnahitaNode, uint64, error)

	// ReadNeighbourhood fetches the neighborhood structure for a given node ID.
	ReadNeighbourhood(req *GraphReadNeighbourhoodRequest) (*dtos.ReadNeighbourhoodResp, error)

	// Traverse performs a graph traversal based on the provided request parameters.
	Traverse(req *GraphTraverseRequest) (*dtos.TraverseResponse, error)

	// ReadShortestPath finds the shortest path between two nodes in the graph.
	ReadShortestPath(req *GraphReadShortestPathRequest) (*dtos.ReadShortestPathResponse, error)

	// Trending
	Trending(req *GraphTrendingRequest) ([]*entities.AnahitaNode, error)
}

// NewGraphGRPC creates a new graph service
func NewGraphGRPC(config GraphGRPCConfigInterface) GraphGRPCInterface {
	return &GraphGRPC{
		config: config,
	}
}

// GraphGRPC is the graph service
type GraphGRPC struct {
	config GraphGRPCConfigInterface
}

// GetClient returns the grpc client
func (srv *GraphGRPC) getClient() (graphpb.GraphServiceClient, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`))

	for {
		cc, err := grpc.NewClient(srv.config.GetGraphServiceAddr(), opts...)

		if err != nil {
			log.Println("Graph GRPC not ready...")
			graphGRPCConnectCounts++
		} else {
			log.Println("Connected to Graph GRPC!")
			return graphpb.NewGraphServiceClient(cc), nil
		}

		if graphGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// BrowseNeighbours fetches neighboring nodes for a given node ID.
func (srv *GraphGRPC) BrowseNeighbours(req *GrapBrowseNeighboursRequest) ([]*entities.AnahitaNode, uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return []*entities.AnahitaNode{}, 0, err
	}
	if client == nil {
		return []*entities.AnahitaNode{}, 0, nil
	}

	log.Println("sorting:", req.Ordering, req.OrderingDir)

	r, err := client.BrowseNeighbours(context.Background(), &graphpb.BrowseNeighboursRequest{
		NodeId:      req.NodeId,
		EdgeTypes:   req.EdgeTypes,
		Direction:   req.Direction,
		Offset:      req.Offset,
		Limit:       req.Limit,
		Ordering:    req.Ordering,
		OrderingDir: req.OrderingDir,
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
		AccessCheck: req.AccessCheck,
	})
	if err != nil {
		return []*entities.AnahitaNode{}, 0, err
	}
	if r == nil {
		return []*entities.AnahitaNode{}, 0, nil
	}

	// log.Println("received neighbours:", r.Ids, r.Total)

	return grpcToGraphNodes(r.Nodes), r.Total, nil
}

// BrowseConnectionNodes fetches nodes connected by a specific edge type.
func (srv *GraphGRPC) BrowseConnectionNodes(req *GraphBrowseConnectionNodesRequest) ([]*entities.AnahitaNode, uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return []*entities.AnahitaNode{}, 0, err
	}
	if client == nil {
		return []*entities.AnahitaNode{}, 0, nil
	}

	r, err := client.BrowseConnectionNodes(context.Background(), &graphpb.BrowseConnectionNodesRequest{
		NodeId:      req.NodeId,
		EdgeType:    req.EdgeType,
		Direction:   req.Direction,
		Offset:      req.Offset,
		Limit:       req.Limit,
		Ordering:    req.Ordering,
		OrderingDir: req.OrderingDir,
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
		AccessCheck: req.AccessCheck,
	})
	if err != nil {
		return []*entities.AnahitaNode{}, 0, err
	}
	if r == nil {
		return []*entities.AnahitaNode{}, 0, nil
	}

	return grpcToGraphNodes(r.Nodes), r.Total, nil
}

// BrowseOrderedByDegree fetches nodes ordered by their degree.
func (srv *GraphGRPC) BrowseOrderedByDegree(req *GraphBrowseOrderedByDegreeRequest) ([]*entities.AnahitaNode, uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return []*entities.AnahitaNode{}, 0, err
	}
	if client == nil {
		return []*entities.AnahitaNode{}, 0, nil
	}

	r, err := client.BrowseOrderedByDegree(context.Background(), &graphpb.BrowseOrderedByDegreeRequest{
		NodeType:    req.NodeType,
		EdgeTypes:   req.EdgeTypes,
		Direction:   req.Direction,
		Offset:      req.Offset,
		Limit:       req.Limit,
		OrderingDir: req.OrderingDir,
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
		AccessCheck: req.AccessCheck,
	})
	if err != nil {
		return []*entities.AnahitaNode{}, 0, err
	}
	if r == nil {
		return []*entities.AnahitaNode{}, 0, nil
	}

	return grpcToGraphNodes(r.Nodes), r.Total, nil
}

// ReadNeighbourhood fetches the neighborhood structure for a given node ID.
func (srv *GraphGRPC) ReadNeighbourhood(req *GraphReadNeighbourhoodRequest) (*dtos.ReadNeighbourhoodResp, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	r, err := client.ReadNeighbourhood(context.Background(), &graphpb.ReadNeighbourhoodRequest{
		NodeId:    req.NodeId,
		EdgeTypes: req.EdgeTypes,
		Depth:     req.Depth,
		Direction: req.Direction,
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
		AccessCheck: req.AccessCheck,
	})
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}

	resp := &dtos.ReadNeighbourhoodResp{
		Center:     grpcToGraphNode(r.Center),
		Neighbours: []*entities.AnahitaNode{},
		Edges:      []*entities.AnahitaEdge{},
		CreatedOn:  time.Now(),
	}
	resp.Neighbours = grpcToGraphNodes(r.Neighbours)
	resp.Edges = grpcToGraphEdges(r.Edges)

	return resp, nil
}

// Traverse performs a graph traversal based on the provided request parameters.
func (srv *GraphGRPC) Traverse(req *GraphTraverseRequest) (*dtos.TraverseResponse, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	r, err := client.Traverse(context.Background(), &graphpb.TraverseRequest{
		StartNodeId: req.StartNodeId,
		EdgeTypes:   req.EdgeTypes,
		Direction:   req.Direction,
		MaxDepth:    req.MaxDepth,
		Strategy:    req.Strategy,
		Limit:       req.Limit,
	})
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}

	resp := &dtos.TraverseResponse{
		Nodes:        []*entities.AnahitaNode{},
		Edges:        []*entities.AnahitaEdge{},
		CreatedOn:    time.Now(),
		DepthReached: r.DepthReached,
	}
	resp.Nodes = grpcToGraphNodes(r.Nodes)
	resp.Edges = grpcToGraphEdges(r.Edges)

	return resp, nil
}

// ReadShortestPath finds the shortest path between two nodes in the graph.
func (srv *GraphGRPC) ReadShortestPath(req *GraphReadShortestPathRequest) (*dtos.ReadShortestPathResponse, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	r, err := client.ReadShortestPath(context.Background(), &graphpb.ReadShortestPathRequest{
		StartNodeId: req.StartNodeId,
		EndNodeId:   req.EndNodeId,
		EdgeTypes:   req.EdgeTypes,
		Direction:   req.Direction,
		MaxDepth:    req.MaxDepth,
	})
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}

	resp := &dtos.ReadShortestPathResponse{
		Nodes:     []*entities.AnahitaNode{},
		Edges:     []*entities.AnahitaEdge{},
		CreatedOn: time.Now(),
		Hops:      r.Hops,
		Found:     r.Found,
		Strategy:  r.Strategy,
	}
	resp.Nodes = grpcToGraphNodes(r.Nodes)
	resp.Edges = grpcToGraphEdges(r.Edges)

	return resp, nil
}

// Trending
func (srv *GraphGRPC) Trending(req *GraphTrendingRequest) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return []*entities.AnahitaNode{}, err
	}
	if client == nil {
		return []*entities.AnahitaNode{}, nil
	}

	r, err := client.Trending(context.Background(), &graphpb.TrendingRequest{
		NodeType:   req.NodeType,
		EdgeTypes:  req.EdgeTypes,
		Direction:  req.Direction,
		TimeWindow: req.TimeWindow,
		Limit:      req.Limit,
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
		AccessCheck: req.AccessCheck,
	})
	if err != nil {
		return []*entities.AnahitaNode{}, err
	}
	if r == nil {
		return []*entities.AnahitaNode{}, nil
	}

	return grpcToGraphNodes(r.Nodes), nil
}
