package services

import (
	"context"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// EventsGRPCInterface is what only an event has, beside the actor it is:
// its time zone, places and online address, whether it was called off, and
// who is going.
type EventsGRPCInterface interface {
	// Save writes an event's own details.
	Save(details *entities.EventDetails) error

	// Read returns the details of each event, with what this person
	// answered. An id that is not an event has no entry.
	Read(ids []int64, personID int64) (map[int64]*entities.EventDetails, error)

	// Rsvp records a person's answer (going or maybe), or takes it back
	// when rsvp is "". It returns saved, full, closed or missing.
	Rsvp(eventID int64, personID int64, rsvp string) (string, error)

	// Cancel calls an event off, and tells everybody who answered except
	// whoever called it off. False when it already was.
	Cancel(eventID int64, viewerID int64) (bool, error)

	// AnnounceChange tells everybody who answered that when the event is
	// has changed, except whoever changed it.
	AnnounceChange(eventID int64, viewerID int64) error

	// ListFor returns the ids of the events that are one person's, in the
	// order to show them, and how many there are. The filter is one of the
	// constants.EventList* names.
	ListFor(personID int64, filter string, limit uint64, offset uint64) ([]int64, uint64, error)

	// ListHostedBy returns the ids of the events a group hosts that this
	// viewer may see, upcoming or past, and how many there are.
	ListHostedBy(groupID int64, viewer *entities.Viewer, filter string, limit uint64, offset uint64) ([]int64, uint64, error)
}

type EventsGRPC struct {
	config GraphGRPCConfigInterface
}

func NewEventsGRPC(config GraphGRPCConfigInterface) EventsGRPCInterface {
	return &EventsGRPC{config: config}
}

// getClient returns the client, on the one connection to the service this
// process keeps (sharedConnection).
func (srv *EventsGRPC) getClient() (graphpb.EventServiceClient, error) {
	conn, err := sharedConnection(srv.config.GetGraphServiceAddr())
	if err != nil {
		return nil, err
	}
	return graphpb.NewEventServiceClient(conn), nil
}

func grpcToEventDetails(details *graphpb.EventDetails) *entities.EventDetails {
	if details == nil {
		return nil
	}

	out := &entities.EventDetails{
		NodeID:              details.NodeId,
		TimezoneName:        details.TimezoneName,
		Capacity:            details.Capacity,
		OnlineURL:           details.OnlineUrl,
		GoingCount:          details.GoingCount,
		MaybeCount:          details.MaybeCount,
		ViewerRsvp:          details.ViewerRsvp,
		HostID:              details.HostId,
		OpenToHostFollowers: details.OpenToHostFollowers,
	}
	if details.CancelledAt > 0 {
		at := time.Unix(details.CancelledAt, 0).UTC()
		out.CancelledAt = &at
	}

	return out
}

func (srv *EventsGRPC) Save(details *entities.EventDetails) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Save(context.Background(), &graphpb.EventSaveRequest{
		Details: &graphpb.EventDetails{
			NodeId:              details.NodeID,
			TimezoneName:        details.TimezoneName,
			Capacity:            details.Capacity,
			OnlineUrl:           details.OnlineURL,
			HostId:              details.HostID,
			OpenToHostFollowers: details.OpenToHostFollowers,
		},
	})

	return err
}

func (srv *EventsGRPC) Read(ids []int64, personID int64) (map[int64]*entities.EventDetails, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Read(context.Background(), &graphpb.EventReadRequest{Ids: ids, PersonId: personID})
	if err != nil {
		return nil, err
	}

	events := make(map[int64]*entities.EventDetails, len(resp.Events))
	for id, details := range resp.Events {
		events[id] = grpcToEventDetails(details)
	}

	return events, nil
}

func (srv *EventsGRPC) Rsvp(eventID int64, personID int64, rsvp string) (string, error) {
	client, err := srv.getClient()
	if err != nil {
		return "", err
	}

	resp, err := client.Rsvp(context.Background(), &graphpb.EventRsvpRequest{
		EventId:  eventID,
		PersonId: personID,
		Rsvp:     rsvp,
	})
	if err != nil {
		return "", err
	}

	return resp.Result, nil
}

func (srv *EventsGRPC) AnnounceChange(eventID int64, viewerID int64) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.AnnounceChange(context.Background(), &graphpb.EventAnnounceChangeRequest{
		EventId:  eventID,
		ViewerId: viewerID,
	})

	return err
}

func (srv *EventsGRPC) Cancel(eventID int64, viewerID int64) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}

	resp, err := client.Cancel(context.Background(), &graphpb.EventCancelRequest{EventId: eventID, ViewerId: viewerID})
	if err != nil {
		return false, err
	}

	return resp.Changed, nil
}

func (srv *EventsGRPC) ListFor(personID int64, filter string, limit uint64, offset uint64) ([]int64, uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, 0, err
	}

	resp, err := client.List(context.Background(), &graphpb.EventListRequest{
		PersonId: personID,
		Filter:   filter,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, 0, err
	}

	return resp.Ids, resp.Total, nil
}

func (srv *EventsGRPC) ListHostedBy(groupID int64, viewer *entities.Viewer, filter string, limit uint64, offset uint64) ([]int64, uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, 0, err
	}

	req := &graphpb.EventListRequest{
		HostId: groupID,
		Filter: filter,
		Limit:  limit,
		Offset: offset,
	}
	if viewer != nil {
		req.ViewerId = viewer.ID
		req.ViewerPersonType = viewer.PersonType
	}

	resp, err := client.List(context.Background(), req)
	if err != nil {
		return nil, 0, err
	}

	return resp.Ids, resp.Total, nil
}
