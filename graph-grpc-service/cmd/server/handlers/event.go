package handlers

import (
	"context"
	"errors"
	"log"
	"time"

	"anahita-libs/constants"
	"anahita-libs/proto/graphpb"
	"graph/data/repositories"
)

// Events: what only an event has, and who is going. Whether the viewer may
// see an event, answer it or call it off is asked by actor-service before
// it calls. What is guarded here is the number of places.

type eventGrpcHandler struct {
	graphpb.EventServiceServer
	env *Env
}

func NewEventGrpcHandler(env *Env) *eventGrpcHandler {
	return &eventGrpcHandler{env: env}
}

func eventToGrpc(details *repositories.EventDetails) *graphpb.EventDetails {
	if details == nil {
		return nil
	}

	out := &graphpb.EventDetails{
		NodeId:       details.NodeID,
		TimezoneName: details.TimezoneName,
		Capacity:     details.Capacity,
		OnlineUrl:    details.OnlineURL,
		GoingCount:   details.GoingCount,
		MaybeCount:   details.MaybeCount,
		ViewerRsvp:   details.ViewerRsvp,
	}
	if details.CancelledAt != nil {
		out.CancelledAt = details.CancelledAt.Unix()
	}

	return out
}

// Save writes an event's own details.
func (h *eventGrpcHandler) Save(ctx context.Context, req *graphpb.EventSaveRequest) (*graphpb.EventSaveResponse, error) {
	if req.Details == nil || req.Details.NodeId == 0 {
		return nil, errors.New("an event is required")
	}

	// A zone nobody has heard of would make every time shown for the event
	// wrong, quietly.
	if req.Details.TimezoneName != "" {
		if _, err := time.LoadLocation(req.Details.TimezoneName); err != nil {
			return nil, errors.New("no such time zone")
		}
	}

	if err := h.env.Repos.Events.Save(&repositories.EventDetails{
		NodeID:       req.Details.NodeId,
		TimezoneName: req.Details.TimezoneName,
		Capacity:     req.Details.Capacity,
		OnlineURL:    req.Details.OnlineUrl,
	}); err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.EventSaveResponse{}, nil
}

// Read returns the details of each event asked for.
func (h *eventGrpcHandler) Read(ctx context.Context, req *graphpb.EventReadRequest) (*graphpb.EventReadResponse, error) {
	events, err := h.env.Repos.Events.Read(req.Ids, req.PersonId)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	resp := &graphpb.EventReadResponse{Events: map[int64]*graphpb.EventDetails{}}
	for id, details := range events {
		resp.Events[id] = eventToGrpc(details)
	}

	return resp, nil
}

// Rsvp records a person's answer, or takes it back. Going to an event is
// following it, so both counts are brought up to date.
func (h *eventGrpcHandler) Rsvp(ctx context.Context, req *graphpb.EventRsvpRequest) (*graphpb.EventRsvpResponse, error) {
	if req.EventId == 0 || req.PersonId == 0 {
		return nil, errors.New("an event and a person are required")
	}

	result, err := h.env.Repos.Events.Rsvp(req.EventId, req.PersonId, req.Rsvp)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if result == constants.EventRsvpSaved {
		if err = h.env.Repos.SocialGraphCounts.UpdateLeaderCount(req.PersonId); err != nil {
			log.Println(err)
		}
		if err = h.env.Repos.SocialGraphCounts.UpdateFollowerCount(req.EventId); err != nil {
			log.Println(err)
		}
	}

	return &graphpb.EventRsvpResponse{Result: result}, nil
}

// Cancel calls an event off.
func (h *eventGrpcHandler) Cancel(ctx context.Context, req *graphpb.EventCancelRequest) (*graphpb.EventCancelResponse, error) {
	changed, err := h.env.Repos.Events.Cancel(req.EventId)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.EventCancelResponse{Changed: changed}, nil
}
