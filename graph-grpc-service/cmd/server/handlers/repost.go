package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"graph/requests"
	"log"
)

type repostHandler struct {
	graphpb.RepostServiceServer
	env *Env
}

func NewRepostHandler(env *Env) *repostHandler {
	return &repostHandler{env: env}
}

// Browse
func (h *repostHandler) Browse(ctx context.Context, req *graphpb.RepostBrowseRequest) (*graphpb.RepostBrowseResponse, error) {
	log.Printf("Received Browse request: %v", req)

	ids, err := h.env.Repos.Reposts.Browse(req)
	if err != nil {
		return nil, err
	}

	gReq := requests.NewNodeRequest().BatchRead
	gReq.IDs = ids
	gReq.Ordering = req.Ordering
	gReq.OrderingDir = req.Direction
	results, err := h.env.Repos.Nodes.BatchRead(gReq)
	if err != nil {
		return nil, err
	}

	nodes := make([]*graphpb.Node, len(results))
	for i, result := range results {
		nodes[i] = nodeToGrpc(result)
	}

	return &graphpb.RepostBrowseResponse{
		Reposts: nodes,
	}, nil
}

// BrowseAuthors
func (h *repostHandler) BrowseAuthors(ctx context.Context, req *graphpb.RepostBrowseAuthorsRequest) (*graphpb.RepostBrowseAuthorsResponse, error) {
	log.Printf("Received BrowseAuthors request: %v", req)

	ids, err := h.env.Repos.Reposts.BrowseAuthors(req)
	if err != nil {
		return nil, err
	}

	gReq := requests.NewNodeRequest().BatchRead
	gReq.IDs = ids
	gReq.Ordering = req.Ordering
	gReq.OrderingDir = req.Direction
	nodes, err := h.env.Repos.Nodes.BatchRead(gReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.RepostBrowseAuthorsResponse{
		Authors: nodesToGrpc(nodes),
	}, nil
}

// Read
func (h *repostHandler) Read(ctx context.Context, req *graphpb.RepostReadRequest) (*graphpb.RepostReadResponse, error) {
	log.Printf("Received Read request: %v", req)

	id, err := h.env.Repos.Reposts.ReadIdByAuthorAndParent(req.ParentId, req.AuthorId)
	if err != nil {
		return nil, err
	}

	node, err := h.env.Repos.Nodes.Read(id)
	if err != nil {
		return nil, err
	}

	return &graphpb.RepostReadResponse{
		Repost: nodeToGrpc(node),
	}, nil
}

// Add
func (h *repostHandler) Add(ctx context.Context, req *graphpb.RepostAddRequest) (*graphpb.RepostAddResponse, error) {
	log.Printf("Received Add request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id

	owner, err := h.env.Repos.Nodes.Read(req.OwnerId)
	if err != nil {
		return nil, err
	}

	parent, err := h.env.Repos.Nodes.Read(req.ParentId)
	if err != nil {
		return nil, err
	}

	node := entities.NewAnahitaNode()
	node.Type = constants.NodeBaseRepostType
	node.Alias = parent.Alias
	node.OwnerID = owner.ID
	node.OwnerType = owner.Type
	node.ParentID = parent.ID
	node.ParentType = parent.Type
	node.Service = req.Service
	node.Access = req.Access
	// A repost is live from the moment it is made. Left unset it was stored
	// switched off, which the feeds do not look at and other listings do.
	node.Enabled = true
	node.CreatedBy = viewer.ID
	node.ModifiedBy = viewer.ID
	node, err = h.env.Repos.Nodes.Add(node)
	if err != nil {
		return nil, err
	}

	if err := h.env.Repos.Reposts.UpdateRepostCount(parent.ID); err != nil {
		return nil, err
	}

	// Implement the logic to add a repost
	return &graphpb.RepostAddResponse{
		Repost: nodeToGrpc(node),
	}, nil
}

// Delete
func (h *repostHandler) Delete(ctx context.Context, req *graphpb.RepostDeleteRequest) (*graphpb.RepostDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req)

	id, err := h.env.Repos.Reposts.ReadIdByAuthorAndParent(req.ParentId, req.AuthorId)
	if err != nil {
		return nil, err
	}

	success, err := h.env.Repos.Nodes.Delete(id)
	if err != nil {
		return nil, err
	}

	if err := h.env.Repos.Reposts.UpdateRepostCount(req.ParentId); err != nil {
		return nil, err
	}

	return &graphpb.RepostDeleteResponse{
		Success: success,
	}, nil
}

// Count
func (h *repostHandler) Count(ctx context.Context, req *graphpb.RepostCountRequest) (*graphpb.RepostCountResponse, error) {
	log.Printf("Received Count request: %v", req)

	total, err := h.env.Repos.Reposts.Count(req)
	if err != nil {
		return nil, err
	}

	return &graphpb.RepostCountResponse{
		Total: total,
	}, nil
}

// IsReposted
func (h *repostHandler) IsReposted(ctx context.Context, req *graphpb.IsRepostedRequest) (*graphpb.IsRepostedResponse, error) {
	statuses, err := h.env.Repos.Reposts.IsRepostedMany(req.AuthorId, []int64{req.NodeId})
	if err != nil {
		return nil, err
	}

	_, ok := statuses[req.NodeId]

	return &graphpb.IsRepostedResponse{
		IsReposted: ok,
	}, nil
}

// IsRepostedMany
func (h *repostHandler) IsRepostedMany(ctx context.Context, req *graphpb.IsRepostedManyRequest) (*graphpb.IsRepostedManyResponse, error) {
	statuses, err := h.env.Repos.Reposts.IsRepostedMany(req.AuthorId, req.NodeIds)
	if err != nil {
		return nil, err
	}

	results := make(map[int64]bool, len(req.NodeIds))
	for _, id := range req.NodeIds {
		_, ok := statuses[id]
		results[id] = ok
	}

	return &graphpb.IsRepostedManyResponse{
		Results: results,
	}, nil
}
