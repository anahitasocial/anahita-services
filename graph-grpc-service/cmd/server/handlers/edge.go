package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"graph/requests"
	"log"
)

type edgeHandler struct {
	graphpb.EdgeServiceServer
	env *Env
}

func NewEdgeHandler(env *Env) *edgeHandler {
	return &edgeHandler{env: env}
}

// EdgeBrowse
func (h *edgeHandler) Browse(ctx context.Context, req *graphpb.EdgeBrowseRequest) (*graphpb.EdgeBrowseResponse, error) {
	log.Printf("Received Browse request: %v", req)

	edgeReq := requests.NewEdgeRequest().Browse
	edgeReq.NodeAID = req.NodeAId
	edgeReq.NodeAType = req.NodeAType
	edgeReq.Types = req.Types
	edgeReq.NodeBID = req.NodeBId
	edgeReq.NodeBType = req.NodeBType
	edgeReq.Limit = req.Limit
	edgeReq.Offset = req.Offset
	edgeReq.Ordering = req.OrderBy
	edgeReq.OrderingDir = req.OrderDir
	edges, err := h.env.Repos.Edges.Browse(edgeReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeBrowseResponse{
		Edges: edgesToGrpc(edges),
	}, nil
}

// EdgeRead
func (h *edgeHandler) Read(ctx context.Context, req *graphpb.EdgeReadRequest) (*graphpb.EdgeReadResponse, error) {
	log.Printf("Received Read request: %v", req)

	edge, err := h.env.Repos.Edges.Read(req.Id)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeReadResponse{
		Edge: edgeToGrpc(edge),
	}, nil
}

// EdgeEdit
func (h *edgeHandler) Edit(ctx context.Context, req *graphpb.EdgeEditRequest) (*graphpb.EdgeEditResponse, error) {
	log.Printf("Received Edit request: %v", req)

	edge := entities.NewAnahitaEdge()
	edge.ID = req.Edge.Id
	edge.Type = req.Edge.Type
	edge.NodeAID = req.Edge.NodeAId
	edge.NodeAType = req.Edge.NodeAType
	edge.NodeBID = req.Edge.NodeBId
	edge.NodeBType = req.Edge.NodeBType
	edge.Meta = req.Edge.Meta
	edge.ModifiedBy = req.Edge.ModifiedBy
	edge.Ordering = req.Edge.Ordering
	if req.Edge.StartsAt != nil {
		edge.StartsAt = req.Edge.StartsAt.AsTime()
	}
	if req.Edge.EndsAt != nil {
		edge.EndsAt = req.Edge.EndsAt.AsTime()
	}
	result, err := h.env.Repos.Edges.Edit(edge)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeEditResponse{
		Edge: edgeToGrpc(result),
	}, nil
}

// EdgeAdd
func (h *edgeHandler) Add(ctx context.Context, req *graphpb.EdgeAddRequest) (*graphpb.EdgeAddResponse, error) {
	log.Printf("Received Add request: %v", req)

	edge := entities.NewAnahitaEdge()
	edge.Type = req.Edge.Type
	edge.NodeAID = req.Edge.NodeAId
	edge.NodeAType = req.Edge.NodeAType
	edge.NodeBID = req.Edge.NodeBId
	edge.NodeBType = req.Edge.NodeBType
	edge.Meta = req.Edge.Meta
	edge.CreatedBy = req.Edge.CreatedBy
	// Was missing, so a single Add recorded who made the edge and never who
	// last touched it. See BatchAdd for the mirror of this.
	edge.ModifiedBy = req.Edge.ModifiedBy
	edge.Ordering = req.Edge.Ordering
	if req.Edge.StartsAt != nil {
		edge.StartsAt = req.Edge.StartsAt.AsTime()
	}
	if req.Edge.EndsAt != nil {
		edge.EndsAt = req.Edge.EndsAt.AsTime()
	}
	result, err := h.env.Repos.Edges.Add(edge)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeAddResponse{
		Edge: edgeToGrpc(result),
	}, nil
}

// EdgeDelete
func (h *edgeHandler) Delete(ctx context.Context, req *graphpb.EdgeDeleteRequest) (*graphpb.EdgeDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req)

	err := h.env.Repos.Edges.Delete(req.Id)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeDeleteResponse{
		Success: true,
	}, nil
}

// EdgeCount
func (h *edgeHandler) Count(ctx context.Context, req *graphpb.EdgeCountRequest) (*graphpb.EdgeCountResponse, error) {
	log.Printf("Received Count request: %v", req)

	edgeReq := requests.NewEdgeRequest().Count
	edgeReq.NodeAID = req.NodeAId
	edgeReq.NodeAType = req.NodeAType
	edgeReq.Types = req.Types
	edgeReq.NodeBID = req.NodeBId
	edgeReq.NodeBType = req.NodeBType
	count, err := h.env.Repos.Edges.Count(edgeReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeCountResponse{
		Count: count,
	}, nil
}

// EdgeCheckEdgeExists
func (h *edgeHandler) CheckEdgeExists(ctx context.Context, req *graphpb.EdgeCheckExistsRequest) (*graphpb.EdgeCheckExistsResponse, error) {
	log.Printf("Received CheckEdgeExists request: %v", req)

	edge := entities.NewAnahitaEdge()
	edge.Type = req.Type
	edge.NodeAID = req.NodeAId
	edge.NodeAType = req.NodeAType
	edge.NodeBID = req.NodeBId
	edge.NodeBType = req.NodeBType
	result, err := h.env.Repos.Edges.Exists(edge)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeCheckExistsResponse{
		Exists: result,
	}, nil
}

// EdgeBatchRead
func (h *edgeHandler) BatchRead(ctx context.Context, req *graphpb.EdgeBatchReadRequest) (*graphpb.EdgeBatchReadResponse, error) {
	log.Printf("Received BatchRead request: %v", req)

	edges, err := h.env.Repos.Edges.BatchRead(req.Ids)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeBatchReadResponse{
		Edges: edgesToGrpc(edges),
	}, nil
}

// EdgeBatchReadByFilters
func (h *edgeHandler) BatchReadByFilters(ctx context.Context, req *graphpb.EdgeBatchReadByFiltersRequest) (*graphpb.EdgeBatchReadByFiltersResponse, error) {
	log.Printf("Received BatchReadByFilters request: %v", req)

	edgeReq := requests.NewEdgeRequest().BatchReadByFilters
	edgeReq.NodeAID = req.NodeAId
	edgeReq.NodeAType = req.NodeAType
	edgeReq.Types = req.Types
	edgeReq.NodeBID = req.NodeBId
	edgeReq.NodeBType = req.NodeBType
	edgeReq.Ordering = req.OrderBy
	edgeReq.OrderingDir = req.OrderDir
	edges, err := h.env.Repos.Edges.BatchReadByFilters(edgeReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeBatchReadByFiltersResponse{
		Edges: edgesToGrpc(edges),
	}, nil
}

// EdgeBatchEdit
func (h *edgeHandler) BatchEdit(ctx context.Context, req *graphpb.EdgeBatchEditRequest) (*graphpb.EdgeBatchEditResponse, error) {
	log.Printf("Received BatchEdit request: %v", req)

	edges := []*entities.AnahitaEdge{}
	for _, edgeReq := range req.Edges {
		edge := entities.NewAnahitaEdge()
		edge.ID = edgeReq.Id
		edge.Type = edgeReq.Type
		edge.NodeAID = edgeReq.NodeAId
		edge.NodeAType = edgeReq.NodeAType
		edge.NodeBID = edgeReq.NodeBId
		edge.NodeBType = edgeReq.NodeBType
		edge.Meta = edgeReq.Meta
		// Both, not one. Add carried CreatedBy and not ModifiedBy; this carried
		// ModifiedBy and not CreatedBy — and validateAdd requires created_by,
		// so BatchAdd could not succeed for any caller at all.
		edge.CreatedBy = edgeReq.CreatedBy
		edge.ModifiedBy = edgeReq.ModifiedBy
		edge.Ordering = edgeReq.Ordering
		if edgeReq.StartsAt != nil {
			edge.StartsAt = edgeReq.StartsAt.AsTime()
		}
		if edgeReq.EndsAt != nil {
			edge.EndsAt = edgeReq.EndsAt.AsTime()
		}
		edges = append(edges, edge)
	}

	result, err := h.env.Repos.Edges.BatchEdit(edges)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeBatchEditResponse{
		Edges: edgesToGrpc(result),
	}, nil
}

// EdgeBatchAdd
func (h *edgeHandler) BatchAdd(ctx context.Context, req *graphpb.EdgeBatchAddRequest) (*graphpb.EdgeBatchAddResponse, error) {
	log.Printf("Received BatchAdd request: %v", req)

	edges := []*entities.AnahitaEdge{}
	for _, edgeReq := range req.Edges {
		edge := entities.NewAnahitaEdge()
		edge.Type = edgeReq.Type
		edge.NodeAID = edgeReq.NodeAId
		edge.NodeAType = edgeReq.NodeAType
		edge.NodeBID = edgeReq.NodeBId
		edge.NodeBType = edgeReq.NodeBType
		edge.Meta = edgeReq.Meta
		// Both, not one. Add carried CreatedBy and not ModifiedBy; this carried
		// ModifiedBy and not CreatedBy — and validateAdd requires created_by,
		// so BatchAdd could not succeed for any caller at all.
		edge.CreatedBy = edgeReq.CreatedBy
		edge.ModifiedBy = edgeReq.ModifiedBy
		edge.Ordering = edgeReq.Ordering
		if edgeReq.StartsAt != nil {
			edge.StartsAt = edgeReq.StartsAt.AsTime()
		}
		if edgeReq.EndsAt != nil {
			edge.EndsAt = edgeReq.EndsAt.AsTime()
		}
		edges = append(edges, edge)
	}

	result, err := h.env.Repos.Edges.BatchAdd(edges)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeBatchAddResponse{
		Edges: edgesToGrpc(result),
	}, nil
}

// EdgeBatchDelete
func (h *edgeHandler) BatchDelete(ctx context.Context, req *graphpb.EdgeBatchDeleteRequest) (*graphpb.EdgeBatchDeleteResponse, error) {
	log.Printf("Received BatchDelete request: %v", req)

	deletedCount, err := h.env.Repos.Edges.BatchDelete(req.Ids)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeBatchDeleteResponse{
		DeletedCount: deletedCount,
	}, nil
}

// EdgeBatchDeleteByFilters
func (h *edgeHandler) BatchDeleteByFilters(ctx context.Context, req *graphpb.EdgeBatchDeleteByFiltersRequest) (*graphpb.EdgeBatchDeleteByFiltersResponse, error) {
	log.Printf("Received BatchDeleteByFilters request: %v", req)

	edgeReq := requests.NewEdgeRequest().BatchDeleteByFilters
	edgeReq.NodeAID = req.NodeAId
	edgeReq.NodeAType = req.NodeAType
	edgeReq.Types = req.Types
	edgeReq.NodeBID = req.NodeBId
	edgeReq.NodeBType = req.NodeBType

	deletedCount, err := h.env.Repos.Edges.BatchDeleteByFilters(edgeReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.EdgeBatchDeleteByFiltersResponse{
		DeletedCount: deletedCount,
	}, nil
}
