package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"graph/requests"
	"log"
)

type nodeHandler struct {
	graphpb.NodeServiceServer
	env *Env
}

func NewNodeHandler(env *Env) *nodeHandler {
	return &nodeHandler{
		env: env,
	}
}

// Browse
func (h *nodeHandler) Browse(ctx context.Context, req *graphpb.NodeBrowseRequest) (*graphpb.NodeBrowseResponse, error) {
	log.Printf("Received Browse request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	gReq := requests.NewNodeRequest().Browse
	gReq.Viewer = viewer
	gReq.Types = FilterValidStrings(req.Types)
	gReq.OwnerIDs = FilterValidIDs(req.OwnerIds)
	gReq.ParentIDs = FilterValidIDs(req.ParentIds)
	gReq.Search = req.Search
	gReq.NearbyLatitude = req.NearbyLatitude
	gReq.NearbyLongitude = req.NearbyLongitude
	gReq.NearbyRadius = req.NearbyRadius
	gReq.Offset = req.Offset
	gReq.Limit = req.Limit
	gReq.Ordering = req.Ordering
	gReq.OrderingDir = req.OrderingDir
	gReq.AccessCheck = req.AccessCheck

	nodes, err := h.env.Repos.Nodes.Browse(gReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.NodeBrowseResponse{
		Nodes: nodesToGrpc(nodes),
	}, nil
}

// Read
func (h *nodeHandler) Read(ctx context.Context, req *graphpb.NodeReadRequest) (*graphpb.NodeReadResponse, error) {
	log.Printf("Received Read request: %v", req)

	node, err := h.env.Repos.Nodes.Read(req.Id)
	if err != nil {
		return nil, err
	}

	return &graphpb.NodeReadResponse{
		Node: nodeToGrpc(node),
	}, nil
}

// ReadByAlias
func (h *nodeHandler) ReadByAlias(ctx context.Context, req *graphpb.NodeReadByAliasRequest) (*graphpb.NodeReadByAliasResponse, error) {
	log.Printf("Received ReadByAlias request: %v", req)

	gReq := requests.NewNodeRequest().ReadByAlias
	gReq.Alias = req.Alias
	gReq.NodeType = req.NodeType
	node, err := h.env.Repos.Nodes.ReadByAlias(gReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.NodeReadByAliasResponse{
		Node: nodeToGrpc(node),
	}, nil
}

// Edit
func (h *nodeHandler) Edit(ctx context.Context, req *graphpb.NodeEditRequest) (*graphpb.NodeEditResponse, error) {
	log.Printf("Received Edit request: %v", req)

	node := ProtoNodeToAnahitaNode(req.Node)
	node, err := h.env.Repos.Nodes.Edit(node)
	if err != nil {
		return nil, err
	}

	return &graphpb.NodeEditResponse{
		Node: nodeToGrpc(node),
	}, nil
}

// Add
func (h *nodeHandler) Add(ctx context.Context, req *graphpb.NodeAddRequest) (*graphpb.NodeAddResponse, error) {
	log.Printf("Received Add request: %v", req)

	node := ProtoNodeToAnahitaNode(req.Node)
	node, err := h.env.Repos.Nodes.Add(node)
	if err != nil {
		return nil, err
	}

	return &graphpb.NodeAddResponse{
		Node: nodeToGrpc(node),
	}, nil
}

// Delete
func (h *nodeHandler) Delete(ctx context.Context, req *graphpb.NodeDeleteRequest) (*graphpb.NodeDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req)
	var err error

	success, err := h.env.Repos.Nodes.Delete(req.Id)
	if err != nil {
		return nil, err
	}

	return &graphpb.NodeDeleteResponse{
		Success: success,
	}, nil
}

// Count
func (h *nodeHandler) Count(ctx context.Context, req *graphpb.NodeCountRequest) (*graphpb.NodeCountResponse, error) {
	log.Printf("Received Count request: %v", req)
	// var err error

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	gReq := requests.NewNodeRequest().Count
	gReq.Viewer = viewer
	gReq.Types = FilterValidStrings(req.Types)
	gReq.OwnerIDs = FilterValidIDs(req.OwnerIds)
	gReq.ParentIDs = FilterValidIDs(req.ParentIds)
	gReq.Search = req.Search
	gReq.NearbyLatitude = req.NearbyLatitude
	gReq.NearbyLongitude = req.NearbyLongitude
	gReq.NearbyRadius = req.NearbyRadius
	gReq.AccessCheck = req.AccessCheck

	total, err := h.env.Repos.Nodes.Count(gReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.NodeCountResponse{
		Total: total,
	}, nil
}

// BatchRead
func (h *nodeHandler) BatchRead(ctx context.Context, req *graphpb.NodeBatchReadRequest) (*graphpb.NodeBatchReadResponse, error) {
	log.Printf("Received Read request: %v", req)

	gReq := requests.NewNodeRequest().BatchRead
	gReq.IDs = req.Ids
	gReq.Ordering = req.Ordering
	gReq.OrderingDir = req.OrderingDir

	nodes, err := h.env.Repos.Nodes.BatchRead(gReq)
	if err != nil {
		return nil, err
	}
	return &graphpb.NodeBatchReadResponse{
		Nodes: nodesToGrpc(nodes),
	}, nil
}

// BatchReadByAliases
func (h *nodeHandler) BatchReadByAliases(ctx context.Context, req *graphpb.NodeBatchReadByAliasesRequest) (*graphpb.NodeBatchReadByAliasesResponse, error) {
	log.Printf("Received BatchReadByAliases request: %v", req)

	gReq := requests.NewNodeRequest().BatchReadByAliases
	gReq.Aliases = req.Aliases
	gReq.NodeType = req.NodeType
	gReq.Ordering = req.Ordering
	gReq.OrderingDir = req.OrderingDir

	nodes, err := h.env.Repos.Nodes.BatchReadByAliases(gReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.NodeBatchReadByAliasesResponse{
		Nodes: nodesToGrpc(nodes),
	}, nil
}

// BatchEdit
func (h *nodeHandler) BatchEdit(ctx context.Context, req *graphpb.NodeBatchEditRequest) (*graphpb.NodeBatchEditResponse, error) {
	log.Printf("Received BatchEdit request: %v", req)
	var err error

	nodes := make([]*entities.AnahitaNode, len(req.Nodes))
	for i, n := range req.Nodes {
		nodes[i] = ProtoNodeToAnahitaNode(n)
	}

	nodes, err = h.env.Repos.Nodes.BatchEdit(nodes)
	if err != nil {
		return nil, err
	}

	return &graphpb.NodeBatchEditResponse{
		Nodes: nodesToGrpc(nodes),
	}, nil
}

// BatchAdd
func (h *nodeHandler) BatchAdd(ctx context.Context, req *graphpb.NodeBatchAddRequest) (*graphpb.NodeBatchAddResponse, error) {
	log.Printf("Received BatchAdd request: %v", req)
	var err error

	nodes := make([]*entities.AnahitaNode, len(req.Nodes))
	for i, n := range req.Nodes {
		nodes[i] = ProtoNodeToAnahitaNode(n)
	}

	nodes, err = h.env.Repos.Nodes.BatchAdd(nodes)
	if err != nil {
		return nil, err
	}

	return &graphpb.NodeBatchAddResponse{
		Nodes: nodesToGrpc(nodes),
	}, err
}

// BatchDelete
func (h *nodeHandler) BatchDelete(ctx context.Context, req *graphpb.NodeBatchDeleteRequest) (*graphpb.NodeBatchDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req)
	var err error

	success, err := h.env.Repos.Nodes.BatchDelete(req.Ids)
	if err != nil {
		return nil, err
	}

	return &graphpb.NodeBatchDeleteResponse{
		Success: success,
	}, nil
}

// BrowseMutualConnections
func (h *nodeHandler) BrowseMutualConnections(ctx context.Context, req *graphpb.BrowseMutualConnectionsRequest) (*graphpb.BrowseMutualConnectionsResponse, error) {
	log.Printf("Received BrowseMutualConnections request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().BrowseMutualConnections
	rReq.Viewer = viewer
	rReq.AccessCheck = req.AccessCheck

	nodes, err := h.env.Repos.Nodes.BrowseMutualConnections(rReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.BrowseMutualConnectionsResponse{
		Nodes: nodesToGrpc(nodes),
	}, nil
}

// CountConnections
func (h *nodeHandler) CountConnections(ctx context.Context, req *graphpb.CountConnectionsRequest) (*graphpb.CountConnectionsResponse, error) {
	log.Printf("Received CountConnections request: %v", req)
	var err error

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().CountConnections

	total, err := h.env.Repos.Nodes.CountConnections(rReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.CountConnectionsResponse{
		Total: total,
	}, nil
}

// ReadConnectionStats
func (h *nodeHandler) ReadConnectionStats(ctx context.Context, req *graphpb.ReadConnectionStatsRequest) (*graphpb.ReadConnectionStatsResponse, error) {
	log.Printf("Received ReadConnectionStats request: %v", req)
	var err error

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	gReq := requests.NewNodeRequest().ReadConnectionStats

	resp, err := h.env.Repos.Nodes.ReadConnectionStats(gReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.ReadConnectionStatsResponse{
		FollowerCount:   resp.FollowerCount,
		LeaderCount:     resp.LeaderCount,
		MutualCount:     resp.MutualCount,
		SubscriberCount: resp.SubscriberCount,
		CommentCount:    resp.CommentCount,
		VoteUpCount:     resp.VoteUpCount,
		VoteDownCount:   resp.VoteDownCount,
		RepostCount:     resp.RepostCount,
		QuoteCount:      resp.QuoteCount,
	}, nil
}
