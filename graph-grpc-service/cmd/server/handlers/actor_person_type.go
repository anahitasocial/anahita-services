package handlers

import (
	"context"
	"errors"
	"log"

	"anahita-libs/constants"
	"anahita-libs/proto/graphpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"graph/data/repositories"
)

// SetPersonType changes a person's type (the user, 2026-10-18; the floor
// decided 2026-09-07, docs/plans/actor-profile-management.md).
//
// Who may make which change is constants.MayChangePersonType: an
// administrator makes administrators of registered people, and the rest is a
// super administrator's. It is checked here, against the type the person has
// now, the viewer's type travelling with the request, so no caller can skip
// it; auth-service asks the same first, to answer before asking for a proof.
//
// Nobody changes their own type. That is the floor: a super administrator can
// only be demoted by another, so the last one never can be, and no count is
// needed. A lone one who wants to leave promotes somebody, who then demotes
// them.
func (h *actorHandler) SetPersonType(ctx context.Context, req *graphpb.ActorSetPersonTypeRequest) (*graphpb.ActorSetPersonTypeResponse, error) {
	if req.Viewer == nil || req.Viewer.Id <= 0 {
		return nil, status.Error(codes.Unauthenticated, "a person's type is changed by somebody")
	}
	viewerType := viewerPersonType(req.Viewer)
	if viewerType != constants.PersonTypeAdmin && viewerType != constants.PersonTypeSuperAdmin {
		return nil, status.Error(codes.PermissionDenied, "person_type_not_allowed")
	}
	if req.Viewer.Id == req.Id {
		return nil, status.Error(codes.FailedPrecondition, "own_person_type")
	}
	if !constants.IsGivenPersonType(req.PersonType) {
		return nil, status.Error(codes.InvalidArgument, repositories.ErrNotAPersonType.Error())
	}

	previous, err := h.env.Repos.Nodes.PersonTypeOf(req.Id)
	if errors.Is(err, repositories.ErrNotAPerson) {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	if err != nil {
		return nil, err
	}

	if previous == req.PersonType {
		return &graphpb.ActorSetPersonTypeResponse{Changed: false, Previous: previous}, nil
	}
	if !constants.MayChangePersonType(viewerType, previous, req.PersonType) {
		return nil, status.Error(codes.PermissionDenied, "person_type_not_allowed")
	}

	changed, err := h.env.Repos.Nodes.SetPersonType(req.Id, previous, req.PersonType)
	if err != nil {
		return nil, err
	}

	if changed {
		actor, err := h.env.Repos.Nodes.Read(req.Id)
		if err != nil {
			log.Println("actor set-person-type: audit read failed:", err)
		} else if err := h.env.Repos.NodeDeletionLogs.Add(
			actor.ID, actor.Alias, actor.Type, req.Viewer.Id, personTypeAction(req.PersonType),
		); err != nil {
			log.Println("actor set-person-type: audit write failed:", err)
		}
		log.Printf("actor set-person-type: %d from %s to %s (by person=%d, %s)",
			req.Id, previous, req.PersonType, req.Viewer.Id, viewerType)
	}

	return &graphpb.ActorSetPersonTypeResponse{Changed: changed, Previous: previous}, nil
}

// personTypeAction is the audit action for being given a type.
func personTypeAction(personType string) string {
	switch personType {
	case constants.PersonTypeSuperAdmin:
		return repositories.DeletionActionMadeSuperAdmin
	case constants.PersonTypeAdmin:
		return repositories.DeletionActionMadeAdmin
	default:
		return repositories.DeletionActionMadeRegistered
	}
}
