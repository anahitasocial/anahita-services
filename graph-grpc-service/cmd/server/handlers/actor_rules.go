package handlers

import (
	"anahita-libs/proto/graphpb"
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"graph/data/repositories"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A group's and an event's rules (roadmap row 102, part 5). Who may set them
// is the caller's to ask, as for any change to a profile; the limits are
// kept here, where they are stored.

func rulesToGrpc(rules *repositories.ProfileRules) *graphpb.ProfileRules {
	out := &graphpb.ProfileRules{ProfileId: rules.ProfileID, Version: rules.Version}
	if rules.UpdatedAt != nil {
		out.UpdatedAt = rules.UpdatedAt.UTC().Format(time.RFC3339)
	}
	for _, item := range rules.Items {
		out.Items = append(out.Items, &graphpb.ProfileRule{Title: item.Title, Body: item.Body})
	}
	return out
}

// ReadRules is a profile's rules, and with a person, the version they last
// agreed to.
func (h *actorHandler) ReadRules(ctx context.Context, req *graphpb.RulesReadRequest) (*graphpb.RulesReadResponse, error) {
	if req.GetProfileId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "a profile is required")
	}
	rules, err := h.env.Repos.ProfileRules.Read(req.ProfileId)
	if err != nil {
		return nil, err
	}
	response := &graphpb.RulesReadResponse{Rules: rulesToGrpc(rules)}
	if req.GetPersonId() > 0 {
		if response.AgreedVersion, err = h.env.Repos.ProfileRules.Agreed(req.PersonId, req.ProfileId); err != nil {
			return nil, err
		}
	}
	return response, nil
}

// SetRules replaces a profile's rules: at most ten, each a title of up to
// 100 letters and a line of up to 300; blank ones are left out.
func (h *actorHandler) SetRules(ctx context.Context, req *graphpb.RulesSetRequest) (*graphpb.RulesSetResponse, error) {
	if req.GetProfileId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "a profile is required")
	}
	var items []repositories.ProfileRule
	for _, each := range req.Items {
		title := strings.TrimSpace(each.GetTitle())
		body := strings.TrimSpace(each.GetBody())
		if title == "" {
			continue
		}
		if utf8.RuneCountInString(title) > repositories.ProfileRuleTitleMax || utf8.RuneCountInString(body) > repositories.ProfileRuleBodyMax {
			return nil, status.Error(codes.InvalidArgument, "a rule is too long")
		}
		items = append(items, repositories.ProfileRule{Title: title, Body: body})
	}
	rules, err := h.env.Repos.ProfileRules.Set(req.ProfileId, items)
	if errors.Is(err, repositories.ErrTooManyRules) {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err != nil {
		return nil, err
	}
	return &graphpb.RulesSetResponse{Rules: rulesToGrpc(rules)}, nil
}

// AgreeRules records the viewer agreeing to the version a profile's rules
// are at now; any other version is refused, so nobody agrees to rules they
// were not shown.
func (h *actorHandler) AgreeRules(ctx context.Context, req *graphpb.RulesAgreeRequest) (*graphpb.RulesAgreeResponse, error) {
	if req.GetProfileId() <= 0 || req.GetViewer().GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "a profile and a viewer are required")
	}
	rules, err := h.env.Repos.ProfileRules.Read(req.ProfileId)
	if err != nil {
		return nil, err
	}
	if rules.Version == 0 || req.Version != rules.Version {
		return nil, status.Error(codes.FailedPrecondition, "rules_not_current")
	}
	if err := h.env.Repos.ProfileRules.Agree(req.Viewer.Id, req.ProfileId, req.Version); err != nil {
		return nil, err
	}
	return &graphpb.RulesAgreeResponse{}, nil
}
