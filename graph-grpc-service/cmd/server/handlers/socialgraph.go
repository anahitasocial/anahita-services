package handlers

import (
	"anahita-libs/data/services"
	"anahita-libs/proto/graphpb"
	"context"
)

type socialgraphGrpcHandler struct {
	graphpb.SocialgraphServiceServer
	env *Env
}

func NewSocialgraphGrpcHandler(env *Env) *socialgraphGrpcHandler {
	return &socialgraphGrpcHandler{env: env}
}

// Browse relationships for a list of actor
func (h *socialgraphGrpcHandler) Browse(ctx context.Context, req *graphpb.SocialgraphBrowseRequest) (*graphpb.SocialgraphBrowseResponse, error) {
	var err error

	res, err := h.env.Repos.SocialgraphRel.GetMany(req.ActorAIds, req.ActorBId)
	if err != nil {
		return nil, err
	}

	relations := make(map[int64]*graphpb.Socialgraph)
	for actorAId, relation := range res {
		relations[actorAId] = services.RelationEntityToGrpc(relation)
	}

	return &graphpb.SocialgraphBrowseResponse{
		Relations: relations,
	}, nil
}

// Read a relationship between two users
func (h *socialgraphGrpcHandler) Read(ctx context.Context, req *graphpb.SocialgraphReadRequest) (*graphpb.SocialgraphReadResponse, error) {
	var err error

	// Fetch the relationship details from the repository
	res, err := h.env.Repos.SocialgraphRel.GetOne(req.ActorAId, req.ActorBId)
	if err != nil {
		return nil, err
	}

	relations := make(map[int64]*graphpb.Socialgraph)
	relations[req.ActorAId] = services.RelationEntityToGrpc(res)

	return &graphpb.SocialgraphReadResponse{
		Relation: relations[req.ActorAId],
	}, nil
}
