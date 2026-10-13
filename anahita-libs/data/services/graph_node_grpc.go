package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/grpctls"
	"anahita-libs/proto/graphpb"
	"context"
	"errors"
	"log"
	"time"

	"google.golang.org/grpc"
)

var graphNodeGRPCConnectCounts int

type GraphNodeGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

// GraphNodeGRPCInterface is the interface for graph node service
type GraphNodeGRPCInterface interface {
	// Browse returns a list of graph nodes
	Browse(req *GraphNodeBrowseRequest) ([]*entities.AnahitaNode, error)

	// Read returns a single graph node
	Read(id int64) (*entities.AnahitaNode, error)

	// ReadByAlias returns a single graph node by alias
	ReadByAlias(req *GraphNodeReadByAlias) (*entities.AnahitaNode, error)

	// ReadByUUID returns a single graph node, of any type, by its UUID.
	// Not access-checked, as Read is not.
	ReadByUUID(uuid string) (*entities.AnahitaNode, error)

	// ReadByURI returns the node kept for something on another server, by
	// what it is called there. Not access-checked.
	ReadByURI(uri string) (*entities.AnahitaNode, error)

	// UpsertRemote keeps a person or a note from another server: added if
	// its URI is new, brought up to date if it is known. It says whether it
	// was new. The graph refuses anything else: see its nodes_remote.go.
	UpsertRemote(node *entities.AnahitaNode) (*entities.AnahitaNode, bool, error)

	// DeleteRemote takes away a note that came from another server, in the
	// name of whoever it belongs to. It says whether there was one.
	DeleteRemote(uri string, ownerID int64) (bool, error)

	// DeleteRemoteActor takes away somebody from another server, with what
	// they posted and every follow that touches them.
	DeleteRemoteActor(uri string) (bool, error)

	// Edit edits a graph node
	Edit(node *entities.AnahitaNode) (*entities.AnahitaNode, error)

	// Add adds a new graph node
	Add(node *entities.AnahitaNode) (*entities.AnahitaNode, error)

	// Delete deletes a graph node
	Delete(id int64) (bool, error)

	// ClearFile takes an avatar ("avatar") or a cover ("cover") off a node.
	// Edit cannot: it leaves an image alone unless it is given a new name.
	ClearFile(id int64, kind string, modifiedBy int64) (bool, error)

	// Count returns the total number of graph nodes
	Count(req *GraphNodeBrowseRequest) (uint64, error)

	// FilterVisible returns which of the given nodes, of any kind, the
	// viewer may see. A nil viewer is the anonymous one. For a caller that
	// must not confirm a hidden node exists, a missing id means "not found".
	FilterVisible(ids []int64, viewer *entities.Viewer) ([]int64, error)

	// BatchRead reads multiple graph nodes
	BatchRead(req *GraphNodeBatchReadRequest) ([]*entities.AnahitaNode, error)

	// BatchReadByAliases reads multiple graph nodes by aliases
	BatchReadByAliases(req *GraphNodeBatchReadByAliasesRequest) ([]*entities.AnahitaNode, error)

	// BatchEdit edits multiple graph nodes
	BatchEdit(nodes []*entities.AnahitaNode) ([]*entities.AnahitaNode, error)

	// BatchAdd adds multiple graph nodes
	BatchAdd(nodes []*entities.AnahitaNode) ([]*entities.AnahitaNode, error)

	// BatchDelete deletes multiple graph nodes
	BatchDelete(ids []int64) (bool, error)
}

// NewGraphNodeGRPC creates a new graph node service
func NewGraphNodeGRPC(config GraphNodeGRPCConfigInterface) GraphNodeGRPCInterface {
	return &GraphNodeGRPC{
		config: config,
	}
}

// GraphNodeGRPC is the graph node service
type GraphNodeGRPC struct {
	config GraphNodeGRPCConfigInterface
}

// GetClient returns the grpc client
func (srv *GraphNodeGRPC) getClient() (graphpb.NodeServiceClient, error) {
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
			log.Println("GraphNode GRPC not ready...")
			graphNodeGRPCConnectCounts++
		} else {
			log.Println("Connected to GraphNode GRPC!")
			return graphpb.NewNodeServiceClient(cc), nil
		}

		if graphNodeGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// Browse returns a list of graph nodes
func (srv *GraphNodeGRPC) Browse(req *GraphNodeBrowseRequest) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.Browse(context.Background(), &graphpb.NodeBrowseRequest{
		Types:           req.Types,
		OwnerIds:        req.OwnerIDs,
		ParentIds:       req.ParentIDs,
		NearbyLatitude:  req.NearbyLatitude,
		NearbyLongitude: req.NearbyLongitude,
		NearbyRadius:    req.NearbyRadius,
		Search:          req.Search,
		Ordering:        req.Ordering,
		OrderingDir:     req.OrderingDir,
		Limit:           req.Limit,
		Offset:          req.Offset,
		AccessCheck:     req.AccessCheck,
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNodes(resp.Nodes), nil
}

// Read returns a single graph node
func (srv *GraphNodeGRPC) Read(id int64) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.Read(context.Background(), &graphpb.NodeReadRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Node), nil
}

// ReadByAlias returns a single graph node by alias
func (srv *GraphNodeGRPC) ReadByAlias(req *GraphNodeReadByAlias) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.ReadByAlias(context.Background(), &graphpb.NodeReadByAliasRequest{
		Alias:    req.Alias,
		NodeType: req.NodeType,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Node), nil
}

// ReadByUUID returns a single graph node, of any type, by its UUID
func (srv *GraphNodeGRPC) ReadByUUID(uuid string) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.ReadByUUID(context.Background(), &graphpb.NodeReadByUUIDRequest{Uuid: uuid})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Node), nil
}

// ReadByURI returns the node kept for something on another server
func (srv *GraphNodeGRPC) ReadByURI(uri string) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.ReadByURI(context.Background(), &graphpb.NodeReadByURIRequest{Uri: uri})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Node), nil
}

// UpsertRemote keeps a person or a note from another server
func (srv *GraphNodeGRPC) UpsertRemote(node *entities.AnahitaNode) (*entities.AnahitaNode, bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, false, err
	}
	if client == nil {
		return nil, false, nil
	}

	resp, err := client.UpsertRemote(context.Background(), &graphpb.NodeUpsertRemoteRequest{Node: graphNodeToGrpc(node)})
	if err != nil {
		return nil, false, err
	}
	if resp == nil {
		return nil, false, nil
	}

	return grpcToGraphNode(resp.Node), resp.Created, nil
}

// DeleteRemote takes away a note that came from another server
func (srv *GraphNodeGRPC) DeleteRemote(uri string, ownerID int64) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}
	if client == nil {
		return false, nil
	}

	resp, err := client.DeleteRemote(context.Background(), &graphpb.NodeDeleteRemoteRequest{Uri: uri, OwnerId: ownerID})
	if err != nil || resp == nil {
		return false, err
	}
	return resp.Deleted, nil
}

// DeleteRemoteActor takes away somebody from another server
func (srv *GraphNodeGRPC) DeleteRemoteActor(uri string) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}
	if client == nil {
		return false, nil
	}

	resp, err := client.DeleteRemoteActor(context.Background(), &graphpb.NodeDeleteRemoteActorRequest{Uri: uri})
	if err != nil || resp == nil {
		return false, err
	}
	return resp.Deleted, nil
}

// Edit edits a graph node
func (srv *GraphNodeGRPC) Edit(node *entities.AnahitaNode) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	req := &graphpb.NodeEditRequest{
		Node: graphNodeToGrpc(node),
	}

	resp, err := client.Edit(context.Background(), req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Node), nil
}

// Add adds a new graph node
func (srv *GraphNodeGRPC) Add(node *entities.AnahitaNode) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	req := &graphpb.NodeAddRequest{
		Node: graphNodeToGrpc(node),
	}

	resp, err := client.Add(context.Background(), req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Node), nil
}

// Delete deletes a graph node
func (srv *GraphNodeGRPC) Delete(id int64) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}
	if client == nil {
		return false, nil
	}

	resp, err := client.Delete(context.Background(), &graphpb.NodeDeleteRequest{
		Id: id,
	})
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}

	return resp.Success, nil
}

// ClearFile takes an avatar or a cover off a node.
func (srv *GraphNodeGRPC) ClearFile(id int64, kind string, modifiedBy int64) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}
	if client == nil {
		return false, nil
	}

	resp, err := client.ClearFile(context.Background(), &graphpb.NodeClearFileRequest{
		Id:         id,
		Kind:       kind,
		ModifiedBy: modifiedBy,
	})
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}

	return resp.Success, nil
}

// Count returns the total number of graph nodes
func (srv *GraphNodeGRPC) Count(req *GraphNodeBrowseRequest) (uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return 0, err
	}
	if client == nil {
		return 0, nil
	}

	r, err := client.Count(context.Background(), &graphpb.NodeCountRequest{
		Types:           req.Types,
		OwnerIds:        req.OwnerIDs,
		ParentIds:       req.ParentIDs,
		Search:          req.Search,
		NearbyLatitude:  req.NearbyLatitude,
		NearbyLongitude: req.NearbyLongitude,
		NearbyRadius:    req.NearbyRadius,
		AccessCheck:     req.AccessCheck,
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
	})
	if err != nil {
		return 0, err
	}
	if r == nil {
		return 0, nil
	}

	return r.Total, nil
}

// FilterVisible returns which of the given nodes the viewer may see.
func (srv *GraphNodeGRPC) FilterVisible(ids []int64, viewer *entities.Viewer) ([]int64, error) {
	if len(ids) == 0 {
		return []int64{}, nil
	}

	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, errors.New("graph node client unavailable")
	}

	req := &graphpb.NodeFilterVisibleRequest{Ids: ids}
	if viewer != nil {
		req.Viewer = &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Username:   viewer.Username,
			Email:      viewer.Email,
		}
	}

	resp, err := client.FilterVisible(context.Background(), req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return []int64{}, nil
	}

	return resp.Ids, nil
}

// BatchRead reads multiple graph nodes
func (srv *GraphNodeGRPC) BatchRead(req *GraphNodeBatchReadRequest) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.BatchRead(context.Background(), &graphpb.NodeBatchReadRequest{
		Ids:         req.IDs,
		Ordering:    req.Ordering,
		OrderingDir: req.OrderingDir,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNodes(resp.Nodes), nil
}

// BatchReadByAliases reads multiple graph nodes by aliases
func (srv *GraphNodeGRPC) BatchReadByAliases(req *GraphNodeBatchReadByAliasesRequest) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.BatchReadByAliases(context.Background(), &graphpb.NodeBatchReadByAliasesRequest{
		Aliases:     req.Aliases,
		NodeType:    req.NodeType,
		Ordering:    req.Ordering,
		OrderingDir: req.OrderingDir,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNodes(resp.Nodes), nil
}

// BatchEdit edits multiple graph nodes
func (srv *GraphNodeGRPC) BatchEdit(nodes []*entities.AnahitaNode) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	var grpcNodes []*graphpb.Node
	for _, n := range nodes {
		grpcNodes = append(grpcNodes, graphNodeToGrpc(n))
	}

	resp, err := client.BatchEdit(context.Background(), &graphpb.NodeBatchEditRequest{
		Nodes: grpcNodes,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNodes(resp.Nodes), nil
}

// BatchAdd adds multiple graph nodes
func (srv *GraphNodeGRPC) BatchAdd(nodes []*entities.AnahitaNode) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	var grpcNodes []*graphpb.Node
	for _, n := range nodes {
		grpcNodes = append(grpcNodes, graphNodeToGrpc(n))
	}

	resp, err := client.BatchAdd(context.Background(), &graphpb.NodeBatchAddRequest{
		Nodes: grpcNodes,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNodes(resp.Nodes), nil
}

// BatchDelete deletes multiple graph nodes
func (srv *GraphNodeGRPC) BatchDelete(ids []int64) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}
	if client == nil {
		return false, nil
	}

	resp, err := client.BatchDelete(context.Background(), &graphpb.NodeBatchDeleteRequest{
		Ids: ids,
	})
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}

	return resp.Success, nil
}
