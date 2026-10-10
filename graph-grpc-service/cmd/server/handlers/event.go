package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
	// Every zone's rules, built into the program: the image this runs in
	// has no zone database of its own. See actor-service's event handler.
	_ "time/tzdata"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"
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
		NodeId:              details.NodeID,
		TimezoneName:        details.TimezoneName,
		Capacity:            details.Capacity,
		OnlineUrl:           details.OnlineURL,
		GoingCount:          details.GoingCount,
		MaybeCount:          details.MaybeCount,
		ViewerRsvp:          details.ViewerRsvp,
		HostId:              details.HostID,
		OpenToHostFollowers: details.OpenToHostFollowers,
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
		NodeID:              req.Details.NodeId,
		TimezoneName:        req.Details.TimezoneName,
		Capacity:            req.Details.Capacity,
		OnlineURL:           req.Details.OnlineUrl,
		HostID:              req.Details.HostId,
		OpenToHostFollowers: req.Details.OpenToHostFollowers,
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

		// Whoever looks after the event hears that somebody is coming.
		// Not of a maybe, nor of somebody leaving.
		if req.Rsvp == constants.EventRsvpGoing {
			h.publish(event.EventFormatEventAnswered, req.EventId, req.PersonId, req.PersonId)
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

	// Everybody who answered is told, once: only when this is what called
	// it off.
	if changed {
		h.tell(event.EventFormatEventCancelled, req.EventId, req.ViewerId, false)
	}

	return &graphpb.EventCancelResponse{Changed: changed}, nil
}

// AnnounceChange tells everybody who answered an event that when it is has
// changed. Whoever changed it decides that it has; this only tells.
func (h *eventGrpcHandler) AnnounceChange(ctx context.Context, req *graphpb.EventAnnounceChangeRequest) (*graphpb.EventAnnounceChangeResponse, error) {
	if req.EventId == 0 {
		return nil, errors.New("an event is required")
	}

	told := h.tell(event.EventFormatEventChanged, req.EventId, req.ViewerId, false)

	return &graphpb.EventAnnounceChangeResponse{Told: uint64(told)}, nil
}

// tell publishes one event for each person who answered an event, or only
// the ones going, leaving out whoever did the thing being told. It answers
// how many were told.
func (h *eventGrpcHandler) tell(format string, eventID int64, by int64, goingOnly bool) int {
	people, err := h.env.Repos.Events.AttendeeIDs(eventID, goingOnly)
	if err != nil {
		log.Println("telling an event's people:", err)
		return 0
	}

	told := 0
	for _, personID := range people {
		if personID == by {
			continue
		}
		h.publish(format, eventID, personID, by)
		told++
	}

	return told
}

// publish tells the other services something about an event. The object is
// whoever it is about or for, the subject whoever did it.
func (h *eventGrpcHandler) publish(format string, eventID int64, objectID int64, by int64) {
	if h.env.Publisher == nil {
		return
	}

	payload := EventNotice(format, eventID, objectID, by)

	go func() {
		log.Println("Publishing event", payload.Name)
		h.env.Publisher.Publish(payload, payload.Name)
	}()
}

// EventNotice is the event for something that happened to an event. The
// sweep uses it too, for reminders.
func EventNotice(format string, eventID int64, objectID int64, by int64) *event.EventPayload {
	routingKey := fmt.Sprintf(format, "actor")

	return &event.EventPayload{
		Name:          routingKey,
		SourceService: "graph-grpc-service",
		SubjectID:     by,
		ObjectID:      objectID,
		TargetID:      eventID,
		ViewerID:      by,
	}
}

// List returns the ids of the events that are one person's.
func (h *eventGrpcHandler) List(ctx context.Context, req *graphpb.EventListRequest) (*graphpb.EventListResponse, error) {
	limit := req.Limit
	if limit == 0 || limit > constants.MaxPageSize {
		limit = constants.DefaultPageSize
	}

	// A group's events, as the viewer may see them.
	if req.HostId > 0 {
		viewer := entities.NewViewer()
		viewer.ID = req.ViewerId
		viewer.PersonType = req.ViewerPersonType

		ids, total, err := h.env.Repos.Events.ListHostedBy(req.HostId, viewer, req.Filter, limit, req.Offset)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		return &graphpb.EventListResponse{Ids: ids, Total: total}, nil
	}

	if req.PersonId == 0 {
		return nil, errors.New("a person or a host is required")
	}

	ids, total, err := h.env.Repos.Events.ListFor(req.PersonId, req.Filter, limit, req.Offset)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.EventListResponse{Ids: ids, Total: total}, nil
}
