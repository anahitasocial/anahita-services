package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/proto/graphpb"
	"context"
	"graph/requests"
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
		Nodes: services.NodesToGrpc(result),
		Total: total,
	}, nil
}

// BrowseConnectionNodes
func (h *graphHandler) BrowseConnectionNodes(ctx context.Context, req *graphpb.BrowseConnectionNodesRequest) (*graphpb.BrowseConnectionNodesResponse, error) {

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
		Nodes: services.NodesToGrpc(result),
		Total: total,
	}, nil
}

// BrowseOrderedByDegree
func (h *graphHandler) BrowseOrderedByDegree(ctx context.Context, req *graphpb.BrowseOrderedByDegreeRequest) (*graphpb.BrowseOrderedByDegreeResponse, error) {

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
		Nodes: services.NodesToGrpc(result),
		Total: total,
	}, nil
}

// ReadNeighbourhood
func (h *graphHandler) ReadNeighbourhood(ctx context.Context, req *graphpb.ReadNeighbourhoodRequest) (*graphpb.ReadNeighbourhoodResponse, error) {
	return &graphpb.ReadNeighbourhoodResponse{}, nil
}

// Traverse
func (h *graphHandler) Traverse(ctx context.Context, req *graphpb.TraverseRequest) (*graphpb.TraverseResponse, error) {
	return &graphpb.TraverseResponse{}, nil
}

// ReadShortestPath
func (h *graphHandler) ReadShortestPath(ctx context.Context, req *graphpb.ReadShortestPathRequest) (*graphpb.ReadShortestPathResponse, error) {
	return &graphpb.ReadShortestPathResponse{}, nil
}

// Trending
func (h *graphHandler) Trending(ctx context.Context, req *graphpb.TrendingRequest) (*graphpb.TrendingResponse, error) {

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType

	result, err := h.env.Repos.Graph.Trending(req, viewer)
	if err != nil {
		return &graphpb.TrendingResponse{}, err
	}
	profiles, err := h.env.Repos.Graph.TrendingProfiles(req, extractNodeIDs(result))
	if err != nil {
		return &graphpb.TrendingResponse{}, err
	}

	return &graphpb.TrendingResponse{
		Nodes:    services.NodesToGrpc(result),
		Profiles: profiles,
	}, nil
}
