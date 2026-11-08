package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"errors"
	"graph/requests"
	"log"
	"time"
)

// Mutes (roadmap row 133). Nothing is published: no other service has
// anything to do about a mute, and the muted person is told nothing.
type socialgraphMuteGrpcHandler struct {
	graphpb.SocialgraphMuteServiceServer
	env *Env
}

func NewSocialgraphMuteGrpcHandler(env *Env) *socialgraphMuteGrpcHandler {
	return &socialgraphMuteGrpcHandler{env: env}
}

// Add mutes, or changes a mute already made: the mute is made again, from
// now, with what is asked.
func (h *socialgraphMuteGrpcHandler) Add(ctx context.Context, req *graphpb.SocialgraphMuteAddRequest) (*graphpb.SocialgraphMuteAddResponse, error) {
	if req.Source.GetId() == 0 || req.Target.GetId() == 0 {
		return nil, errors.New("source and target IDs are required")
	}
	if req.Source.Id == req.Target.Id {
		return nil, errors.New("source and target cannot be the same")
	}
	if req.Viewer.GetId() == 0 {
		return nil, errors.New("viewer ID is required")
	}

	source, err := h.env.Repos.Nodes.Read(req.Source.Id)
	if err != nil {
		return nil, err
	}
	target, err := h.env.Repos.Nodes.Read(req.Target.Id)
	if err != nil {
		return nil, err
	}

	if _, err := h.env.Repos.Edges.BatchDeleteByFilters(unmuteFilter(source, target)); err != nil {
		log.Println(err)
		return nil, err
	}

	edge := &entities.AnahitaEdge{
		Type:       constants.EdgeMuteEntityType,
		NodeAID:    source.ID,
		NodeAType:  source.Type,
		NodeBID:    target.ID,
		NodeBType:  target.Type,
		CreatedBy:  req.Viewer.Id,
		ModifiedBy: req.Viewer.Id,
	}
	if !req.HideNotifications {
		edge.Meta = constants.MuteMetaNotifications
	}
	if req.EndsAt > 0 {
		edge.EndsAt = time.Unix(req.EndsAt, 0).UTC()
	}
	if _, err := h.env.Repos.Edges.Add(edge); err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.SocialgraphMuteAddResponse{Success: true}, nil
}

// Delete unmutes.
func (h *socialgraphMuteGrpcHandler) Delete(ctx context.Context, req *graphpb.SocialgraphMuteDeleteRequest) (*graphpb.SocialgraphMuteDeleteResponse, error) {
	if req.Source.GetId() == 0 || req.Target.GetId() == 0 {
		return nil, errors.New("source and target IDs are required")
	}

	source, err := h.env.Repos.Nodes.Read(req.Source.Id)
	if err != nil {
		return nil, err
	}
	target, err := h.env.Repos.Nodes.Read(req.Target.Id)
	if err != nil {
		return nil, err
	}

	if _, err := h.env.Repos.Edges.BatchDeleteByFilters(unmuteFilter(source, target)); err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.SocialgraphMuteDeleteResponse{Success: true}, nil
}

// unmuteFilter names the one edge a mute is: from who muted to who is muted.
func unmuteFilter(source, target *entities.AnahitaNode) *requests.EdgeBatchDeleteByFiltersRequest {
	filter := requests.NewEdgeRequest().BatchDeleteByFilters
	filter.Types = []string{constants.EdgeMuteEntityType}
	filter.NodeAID = source.ID
	filter.NodeAType = source.Type
	filter.NodeBID = target.ID
	filter.NodeBType = target.Type
	return filter
}
