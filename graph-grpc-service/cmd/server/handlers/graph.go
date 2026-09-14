package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"graph/requests"
	"log"
)

type graphHandler struct {
	graphpb.GraphServiceServer
	env *Env
}

func NewGraphHandler(env *Env) *graphHandler {
	return &graphHandler{env: env}
}

// BrowseNeighbours
func (h *graphHandler) BrowseNeighbours(ctx context.Context, req *graphpb.BrowseNeighboursRequest) (*graphpb.BrowseNeighboursResponse, error) {
	log.Printf("Received BrowseNeighbours request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType

	total, err := h.env.Repos.Graph.CountNeighbours(req, viewer)
	if err != nil {
		return &graphpb.BrowseNeighboursResponse{}, err
	}

	result, err := h.env.Repos.Graph.BrowseNeighbours(req, viewer)
	if err != nil {
		return &graphpb.BrowseNeighboursResponse{}, err
	}

	// log.Printf("BrowseNeighbours response IDs: %v, Total: %d", ids, total)

	return &graphpb.BrowseNeighboursResponse{
		Nodes: nodesToGrpc(result),
		Total: total,
	}, nil
}

// BrowseConnectionNodes
func (h *graphHandler) BrowseConnectionNodes(ctx context.Context, req *graphpb.BrowseConnectionNodesRequest) (*graphpb.BrowseConnectionNodesResponse, error) {
	log.Printf("Received BrowseConnectionNodes request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType

	total, err := h.env.Repos.Graph.CountConnectionNodes(req, viewer)
	if err != nil {
		return &graphpb.BrowseConnectionNodesResponse{}, err
	}

	result, err := h.env.Repos.Graph.BrowseConnectionNodes(req, viewer)
	if err != nil {
		return &graphpb.BrowseConnectionNodesResponse{}, err
	}

	return &graphpb.BrowseConnectionNodesResponse{
		Nodes: nodesToGrpc(result),
		Total: total,
	}, nil
}

// BrowseOrderedByDegree
func (h *graphHandler) BrowseOrderedByDegree(ctx context.Context, req *graphpb.BrowseOrderedByDegreeRequest) (*graphpb.BrowseOrderedByDegreeResponse, error) {
	log.Printf("Received BrowseOrderedByDegree request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType

	result, err := h.env.Repos.Graph.BrowseOrderedByDegree(req, viewer)
	if err != nil {
		return &graphpb.BrowseOrderedByDegreeResponse{}, err
	}

	cReq := requests.NewNodeRequest().Count
	cReq.Types = []string{req.NodeType}
	cReq.AccessCheck = req.AccessCheck
	total, err := h.env.Repos.Nodes.Count(cReq)
	if err != nil {
		return &graphpb.BrowseOrderedByDegreeResponse{}, err
	}

	return &graphpb.BrowseOrderedByDegreeResponse{
		Nodes: nodesToGrpc(result),
		Total: total,
	}, nil
}

// ReadNeighbourhood
func (h *graphHandler) ReadNeighbourhood(ctx context.Context, req *graphpb.ReadNeighbourhoodRequest) (*graphpb.ReadNeighbourhoodResponse, error) {
	log.Printf("Received ReadNeighbourhood request: %v", req)
	return &graphpb.ReadNeighbourhoodResponse{}, nil
}

// Traverse
func (h *graphHandler) Traverse(ctx context.Context, req *graphpb.TraverseRequest) (*graphpb.TraverseResponse, error) {
	log.Printf("Received Traverse request: %v", req)
	return &graphpb.TraverseResponse{}, nil
}

// ReadShortestPath
func (h *graphHandler) ReadShortestPath(ctx context.Context, req *graphpb.ReadShortestPathRequest) (*graphpb.ReadShortestPathResponse, error) {
	log.Printf("Received ReadShortestPath request: %v", req)
	return &graphpb.ReadShortestPathResponse{}, nil
}

// Trending
func (h *graphHandler) Trending(ctx context.Context, req *graphpb.TrendingRequest) (*graphpb.TrendingResponse, error) {
	log.Printf("Received Trending request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType

	result, err := h.env.Repos.Graph.Trending(req, viewer)
	if err != nil {
		return &graphpb.TrendingResponse{}, err
	}

	return &graphpb.TrendingResponse{
		Nodes: nodesToGrpc(result),
	}, nil
}
