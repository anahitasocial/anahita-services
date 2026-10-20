package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"log"

	"graph/data/validators"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AddThread makes several posts in one go, all or none (roadmap row 101):
// each part after the first a reply to the one before, the first a post or
// a reply to what `parent_id` names. They are written in one transaction,
// so a thread is never left half made, and announced in order once all are
// there, each with its place in the thread.
func (h *mediumHandler) AddThread(ctx context.Context, req *graphpb.MediumThreadAddRequest) (*graphpb.MediumThreadAddResponse, error) {
	if len(req.Parts) == 0 || len(req.Parts) > constants.ThreadPartsMax {
		return nil, status.Errorf(codes.InvalidArgument, "a thread has 1 to %d parts", constants.ThreadPartsMax)
	}

	viewer := entities.NewViewer()
	if req.Viewer != nil {
		viewer.ID = req.Viewer.Id
		viewer.PersonType = req.Viewer.PersonType
		viewer.Username = req.Viewer.Username
		viewer.Email = req.Viewer.Email
	}

	// What the first part answers, checked as Add checks a reply's.
	var parent *entities.AnahitaNode
	if req.ParentId > 0 {
		read, err := h.env.Repos.Nodes.Read(req.ParentId)
		if err != nil {
			return nil, err
		}
		if err := placeInThread(entities.NewAnahitaNode(), read); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		parent = read
	}

	v := validators.NewMediumValidator()
	parts := make([]*entities.AnahitaNode, 0, len(req.Parts))
	for _, each := range req.Parts {
		part := newMediumFromRequest(each)
		if err := v.ValidateAdd(part); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		parts = append(parts, part)
	}

	// A thread that begins a post on a group or an event with rules: as a
	// post, agreed to first (roadmap row 102, part 5c).
	if parent == nil && hasRules(parts[0].OwnerType) {
		if err := requireRulesAgreed(h.env, viewer.ID, parts[0].OwnerID); err != nil {
			return nil, err
		}
	}

	made, err := h.env.Repos.Nodes.AddThread(parent, parts)
	if err != nil {
		return nil, err
	}

	// The first part's locations, before it is announced.
	if len(req.LocationIds) > 0 {
		h.tagLocations(made[0], req.LocationIds, viewer.ID)
	}

	// Each part is one more reply to the one before it, and to the root.
	for _, part := range made {
		if part.RootID > 0 {
			if err := h.env.Repos.Replies.Recount(part.RootID, part.ParentID); err != nil {
				log.Printf("counting the replies of %d failed: %v", part.RootID, err)
			}
		}
	}

	// Announced in order, one after another.
	go func() {
		for position, part := range made {
			h.announceAdded(part, viewer, position+1)
		}
	}()

	response := &graphpb.MediumThreadAddResponse{}
	for _, part := range made {
		response.Parts = append(response.Parts, nodeToGrpc(part))
	}
	return response, nil
}
