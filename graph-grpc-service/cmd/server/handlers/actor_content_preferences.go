package handlers

import (
	"anahita-libs/proto/graphpb"
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A person's choice of Show, Warn or Hide for each kind of content warning
// (roadmap row 122, phase 2). Whose they may read or change is the
// caller's to ask: their own alone.

func (h *actorHandler) ReadContentPreferences(ctx context.Context, req *graphpb.ContentPreferencesReadRequest) (*graphpb.ContentPreferences, error) {
	if req.GetPersonId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "a person is required")
	}
	choices, err := h.env.Repos.ContentPreferences.Read(req.PersonId)
	if err != nil {
		return nil, err
	}
	return &graphpb.ContentPreferences{Choices: choices}, nil
}

func (h *actorHandler) SaveContentPreferences(ctx context.Context, req *graphpb.ContentPreferencesSaveRequest) (*graphpb.ContentPreferences, error) {
	if req.GetPersonId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "a person is required")
	}
	choices, err := h.env.Repos.ContentPreferences.Save(req.PersonId, req.Choices)
	if err != nil {
		return nil, err
	}
	return &graphpb.ContentPreferences{Choices: choices}, nil
}
