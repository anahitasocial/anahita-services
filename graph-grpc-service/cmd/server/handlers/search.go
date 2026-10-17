package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"log"
)

type searchHandler struct {
	graphpb.SearchServiceServer
	env *Env
}

func NewSearchHandler(env *Env) *searchHandler {
	return &searchHandler{env: env}
}

// Count return the total number of search results
func (h *searchHandler) Count(ctx context.Context, req *graphpb.SearchCountRequest) (*graphpb.SearchCountResponse, error) {
	log.Printf("Received Search Count request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	total, err := h.env.Repos.Search.Count(req, viewer)
	if err != nil {
		return nil, err
	}

	return &graphpb.SearchCountResponse{
		Total: total,
	}, nil
}

// Browse return a list of search results
func (h *searchHandler) Browse(ctx context.Context, req *graphpb.SearchBrowseRequest) (*graphpb.SearchBrowseResponse, error) {
	log.Printf("Received Search Browse request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	nodes, err := h.env.Repos.Search.Browse(req, viewer)
	if err != nil {
		return nil, err
	}

	return &graphpb.SearchBrowseResponse{
		Nodes: nodesToGrpc(nodes),
	}, nil
}

// ReadPreferences is the viewer's own steps for Most relevant.
func (h *searchHandler) ReadPreferences(ctx context.Context, req *graphpb.SearchPreferencesReadRequest) (*graphpb.SearchPreferences, error) {
	if req.Viewer == nil || req.Viewer.Id <= 0 {
		return &graphpb.SearchPreferences{Steps: map[string]float64{}}, nil
	}
	steps, err := h.env.Repos.Search.ReadPreferences(req.Viewer.Id)
	if err != nil {
		return nil, err
	}
	return &graphpb.SearchPreferences{Steps: steps}, nil
}

// SavePreferences keeps the viewer's own steps, and answers with what was
// kept: a name that may not be set, or a value that is not a step, is not.
func (h *searchHandler) SavePreferences(ctx context.Context, req *graphpb.SearchPreferencesSaveRequest) (*graphpb.SearchPreferences, error) {
	if req.Viewer == nil || req.Viewer.Id <= 0 {
		return &graphpb.SearchPreferences{Steps: map[string]float64{}}, nil
	}
	steps, err := h.env.Repos.Search.SavePreferences(req.Viewer.Id, req.Steps)
	if err != nil {
		return nil, err
	}
	return &graphpb.SearchPreferences{Steps: steps}, nil
}
