package handlers

import (
	"context"
	"errors"

	"anahita-libs/federation"
	"anahita-libs/proto/graphpb"
)

// The servers a person blocked for themselves (roadmap row 152). Kept
// here; what a block ends, its follows, is ended by socialgraph-service
// through the ordinary removal of a follow, so that the other server is
// told as for any follow that ends.
type socialgraphServerBlockGrpcHandler struct {
	graphpb.SocialgraphServerBlockServiceServer
	env *Env
}

func NewSocialgraphServerBlockGrpcHandler(env *Env) *socialgraphServerBlockGrpcHandler {
	return &socialgraphServerBlockGrpcHandler{env: env}
}

// serverOf is a domain as it is kept: lowercase, without a scheme, a port
// or a path, as the site's own refused servers are.
func serverOf(personID int64, domain string) (string, error) {
	domain = federation.CleanDomain(domain)
	if personID <= 0 || domain == "" {
		return "", errors.New("a person and a server are required")
	}
	return domain, nil
}

func (h *socialgraphServerBlockGrpcHandler) Add(ctx context.Context, req *graphpb.ServerBlockAddRequest) (*graphpb.ServerBlockAddResponse, error) {
	domain, err := serverOf(req.PersonId, req.Domain)
	if err != nil {
		return nil, err
	}
	if err := h.env.Repos.ServerBlocks.Add(req.PersonId, domain); err != nil {
		return nil, err
	}
	return &graphpb.ServerBlockAddResponse{Success: true}, nil
}

func (h *socialgraphServerBlockGrpcHandler) Delete(ctx context.Context, req *graphpb.ServerBlockDeleteRequest) (*graphpb.ServerBlockDeleteResponse, error) {
	domain, err := serverOf(req.PersonId, req.Domain)
	if err != nil {
		return nil, err
	}
	if err := h.env.Repos.ServerBlocks.Delete(req.PersonId, domain); err != nil {
		return nil, err
	}
	return &graphpb.ServerBlockDeleteResponse{Success: true}, nil
}

func (h *socialgraphServerBlockGrpcHandler) Browse(ctx context.Context, req *graphpb.ServerBlockBrowseRequest) (*graphpb.ServerBlockBrowseResponse, error) {
	if req.PersonId <= 0 {
		return nil, errors.New("a person is required")
	}
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	start := req.Start
	if start < 0 {
		start = 0
	}
	rows, total, err := h.env.Repos.ServerBlocks.Browse(req.PersonId, uint64(start), uint64(limit))
	if err != nil {
		return nil, err
	}
	resp := &graphpb.ServerBlockBrowseResponse{Total: total}
	for _, row := range rows {
		resp.Blocks = append(resp.Blocks, &graphpb.ServerBlock{Domain: row.Domain, CreatedAt: row.CreatedAt.Unix()})
	}
	return resp, nil
}

func (h *socialgraphServerBlockGrpcHandler) Follows(ctx context.Context, req *graphpb.ServerFollowsRequest) (*graphpb.ServerFollowsResponse, error) {
	domain, err := serverOf(req.PersonId, req.Domain)
	if err != nil {
		return nil, err
	}
	pairs, err := h.env.Repos.ServerBlocks.Follows(req.PersonId, domain)
	if err != nil {
		return nil, err
	}
	resp := &graphpb.ServerFollowsResponse{}
	for _, pair := range pairs {
		resp.Follows = append(resp.Follows, &graphpb.ServerFollow{SourceId: pair[0], TargetId: pair[1]})
	}
	return resp, nil
}
