package services

import (
	"anahita-libs/proto/graphpb"
	"context"

	"anahita-libs/data/entities"
)

type GraphEdgeGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

// GraphEdgeGRPCInterface is the interface for graph edge service
type GraphEdgeGRPCInterface interface {
	// Browse returns a list of graph edges
	Browse(req *GraphEdgeBrowseRequest) ([]*entities.AnahitaEdge, error)

	// Read returns a single graph edge
	Read(id int64) (*entities.AnahitaEdge, error)

	// Edit edits a graph edge
	Edit(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error)

	// Add adds a new graph edge
	Add(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error)

	// Delete deletes a graph edge
	Delete(id int64) error

	// Count returns the total number of graph edges
	Count(req *GraphEdgeCountRequest) (uint64, error)

	// CheckEdgeExists checks if an edge exists between two nodes
	CheckEdgeExists(edge *entities.AnahitaEdge) (bool, error)

	// BatchRead reads multiple graph edges by their IDs
	BatchRead(ids []int64) ([]*entities.AnahitaEdge, error)

	// BatchReadByFilters reads multiple graph edges by filters
	BatchReadByFilters(req *GraphEdgeBatchReadByFiltersRequest) ([]*entities.AnahitaEdge, error)

	// BatchEdit edits multiple graph edges
	BatchEdit(edges []*entities.AnahitaEdge) ([]*entities.AnahitaEdge, error)

	// BatchAdd adds multiple graph edges
	BatchAdd(edges []*entities.AnahitaEdge) ([]*entities.AnahitaEdge, error)

	// BatchDelete deletes multiple graph edges by criteria
	BatchDelete(ids []int64) error

	// BatchDeleteByFilters deletes multiple graph edges by filters
	BatchDeleteByFilters(req *GraphEdgeBatchDeleteByFiltersRequest) error
}

// NewGraphEdgeGRPC creates a new graph edge service
func NewGraphEdgeGRPC(config GraphEdgeGRPCConfigInterface) GraphEdgeGRPCInterface {
	return &GraphEdgeGRPC{
		config: config,
	}
}

// GraphEdgeGRPC is the graph edge service
type GraphEdgeGRPC struct {
	config GraphEdgeGRPCConfigInterface
}

// GetClient returns the grpc client
// getClient returns the client, on the one connection to the service this
// process keeps (sharedConnection).
func (srv *GraphEdgeGRPC) getClient() (graphpb.EdgeServiceClient, error) {
	conn, err := sharedConnection(srv.config.GetGraphServiceAddr())
	if err != nil {
		return nil, err
	}
	return graphpb.NewEdgeServiceClient(conn), nil
}

// Browse returns a list of graph edges
func (srv *GraphEdgeGRPC) Browse(req *GraphEdgeBrowseRequest) ([]*entities.AnahitaEdge, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Browse(context.Background(), &graphpb.EdgeBrowseRequest{
		NodeAId:   req.NodeAID,
		NodeAType: req.NodeAType,
		Types:     req.Types,
		NodeBId:   req.NodeBID,
		NodeBType: req.NodeBType,
		Offset:    req.Offset,
		Limit:     req.Limit,
		OrderBy:   req.Ordering,
		OrderDir:  req.OrderingDir,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return []*entities.AnahitaEdge{}, nil
	}

	return grpcToGraphEdges(resp.GetEdges()), nil
}

// Read returns a single graph edge
func (srv *GraphEdgeGRPC) Read(id int64) (*entities.AnahitaEdge, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Read(context.Background(), &graphpb.EdgeReadRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphEdge(resp.GetEdge()), nil
}

// Edit edits a graph edge
func (srv *GraphEdgeGRPC) Edit(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.Edit(context.Background(), &graphpb.EdgeEditRequest{
		Edge: graphEdgeToGrpc(edge),
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphEdge(resp.GetEdge()), nil
}

// Add adds a new graph edge
func (srv *GraphEdgeGRPC) Add(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Add(context.Background(), &graphpb.EdgeAddRequest{
		Edge: graphEdgeToGrpc(edge),
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphEdge(resp.GetEdge()), nil
}

// Delete deletes a graph edge
func (srv *GraphEdgeGRPC) Delete(id int64) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Delete(context.Background(), &graphpb.EdgeDeleteRequest{
		Id: id,
	})
	if err != nil {
		return err
	}

	return nil
}

// Count returns the total number of graph edges
func (srv *GraphEdgeGRPC) Count(req *GraphEdgeCountRequest) (uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return 0, err
	}

	resp, err := client.Count(context.Background(), &graphpb.EdgeCountRequest{
		NodeAId:   req.NodeAID,
		NodeAType: req.NodeAType,
		Types:     req.Types,
		NodeBId:   req.NodeBID,
		NodeBType: req.NodeBType,
	})
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, nil
	}

	return resp.GetCount(), nil
}

// CheckEdgeExists checks if an edge exists between two nodes
func (srv *GraphEdgeGRPC) CheckEdgeExists(edge *entities.AnahitaEdge) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}

	resp, err := client.CheckExists(context.Background(), &graphpb.EdgeCheckExistsRequest{
		NodeAId:   edge.NodeAID,
		NodeAType: edge.NodeAType,
		Type:      edge.Type,
		NodeBId:   edge.NodeBID,
		NodeBType: edge.NodeBType,
	})
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}

	return resp.GetExists(), nil
}

// BatchRead reads multiple graph edges by their IDs
func (srv *GraphEdgeGRPC) BatchRead(ids []int64) ([]*entities.AnahitaEdge, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.BatchRead(context.Background(), &graphpb.EdgeBatchReadRequest{
		Ids: ids,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return []*entities.AnahitaEdge{}, nil
	}

	return grpcToGraphEdges(resp.GetEdges()), nil
}

// BatchReadByFilters reads multiple graph edges by filters
func (srv *GraphEdgeGRPC) BatchReadByFilters(req *GraphEdgeBatchReadByFiltersRequest) ([]*entities.AnahitaEdge, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.BatchReadByFilters(context.Background(), &graphpb.EdgeBatchReadByFiltersRequest{
		NodeAId:   req.NodeAID,
		NodeAType: req.NodeAType,
		Types:     req.Types,
		NodeBId:   req.NodeBID,
		NodeBType: req.NodeBType,
		OrderBy:   req.Ordering,
		OrderDir:  req.OrderingDir,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return []*entities.AnahitaEdge{}, nil
	}

	return grpcToGraphEdges(resp.GetEdges()), nil
}

// BatchEdit edits multiple graph edges
func (srv *GraphEdgeGRPC) BatchEdit(edges []*entities.AnahitaEdge) ([]*entities.AnahitaEdge, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	var grpcReqs []*graphpb.Edge
	for _, r := range edges {
		grpcReqs = append(grpcReqs, graphEdgeToGrpc(r))
	}

	resp, err := client.BatchEdit(context.Background(), &graphpb.EdgeBatchEditRequest{
		Edges: grpcReqs,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return []*entities.AnahitaEdge{}, nil
	}

	return grpcToGraphEdges(resp.GetEdges()), nil
}

// BatchAdd adds multiple graph edges
func (srv *GraphEdgeGRPC) BatchAdd(edges []*entities.AnahitaEdge) ([]*entities.AnahitaEdge, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	var grpcReqs []*graphpb.Edge
	for _, r := range edges {
		grpcReqs = append(grpcReqs, graphEdgeToGrpc(r))
	}

	resp, err := client.BatchAdd(context.Background(), &graphpb.EdgeBatchAddRequest{
		Edges: grpcReqs,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return []*entities.AnahitaEdge{}, nil
	}

	return grpcToGraphEdges(resp.GetEdges()), nil
}

// BatchDelete deletes multiple graph edges by criteria
func (srv *GraphEdgeGRPC) BatchDelete(ids []int64) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.BatchDelete(context.Background(), &graphpb.EdgeBatchDeleteRequest{
		Ids: ids,
	})
	if err != nil {
		return err
	}

	return nil
}

// BatchDeleteByFilters deletes multiple graph edges by filters
func (srv *GraphEdgeGRPC) BatchDeleteByFilters(req *GraphEdgeBatchDeleteByFiltersRequest) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.BatchDeleteByFilters(context.Background(), &graphpb.EdgeBatchDeleteByFiltersRequest{
		NodeAId:   req.NodeAID,
		NodeAType: req.NodeAType,
		Types:     req.Types,
		NodeBId:   req.NodeBID,
		NodeBType: req.NodeBType,
	})
	if err != nil {
		return err
	}

	return nil
}
