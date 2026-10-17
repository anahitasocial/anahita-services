package handlers

import (
	"anahita-libs/proto/graphpb"
	"context"
)

type hashtagHandler struct {
	graphpb.HashtagServiceServer
	env *Env
}

func NewHashtagHandler(env *Env) *hashtagHandler {
	return &hashtagHandler{env: env}
}

// Suggest is the hashtags in use offered to somebody writing: those that
// begin with what they typed, or those that are words of their text
// (repositories.HashtagSuggestionsRepoInterface).
func (h *hashtagHandler) Suggest(ctx context.Context, req *graphpb.HashtagSuggestRequest) (*graphpb.HashtagSuggestResponse, error) {
	if req.Viewer == nil || req.Viewer.Id <= 0 {
		return &graphpb.HashtagSuggestResponse{}, nil
	}
	suggestions, err := h.env.Repos.HashtagSuggestions.Suggest(req)
	if err != nil {
		return nil, err
	}
	return &graphpb.HashtagSuggestResponse{Suggestions: suggestions}, nil
}
