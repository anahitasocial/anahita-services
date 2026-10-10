package responses

import (
	"time"

	"anahita-libs/data/entities"
)

// Event is what only an event has, on the actor it is.
type Event struct {
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	// The zone the times were given in, as IANA names it. The times
	// themselves are UTC.
	TimezoneName string `json:"timezone_name"`
	// How many may be going. 0 for no limit.
	Capacity uint32 `json:"capacity"`
	// Where to join online. Sent only to whoever is going and whoever
	// looks after the event; HasOnlineURL says there is one all the same.
	OnlineURL    string `json:"online_url,omitempty"`
	HasOnlineURL bool   `json:"has_online_url"`
	// Set when it was called off.
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
	GoingCount  uint64     `json:"going_count"`
	MaybeCount  uint64     `json:"maybe_count"`
	// Every place is taken. Maybe is still open.
	IsFull bool `json:"is_full"`
	// What the viewer has answered: going, maybe, or left out.
	ViewerRsvp string `json:"viewer_rsvp,omitempty"`
	// upcoming, happening, past or cancelled, as of when it was sent.
	State string `json:"state"`
	// The group hosting it, when there is one the viewer may see.
	Host *ActorBrief `json:"host,omitempty"`
}

const (
	EventStateUpcoming  = "upcoming"
	EventStateHappening = "happening"
	EventStatePast      = "past"
	EventStateCancelled = "cancelled"
)

// EventState says where an event is in its life at a moment.
func EventState(startsAt time.Time, endsAt time.Time, cancelled bool, now time.Time) string {
	switch {
	case cancelled:
		return EventStateCancelled
	case !now.Before(endsAt):
		return EventStatePast
	case !now.Before(startsAt):
		return EventStateHappening
	}
	return EventStateUpcoming
}

// NewEvent maps an event node to what is sent of it. Nil for a node that
// carries no event details.
func NewEvent(node *entities.AnahitaNode) *Event {
	if node == nil || node.Event == nil {
		return nil
	}

	details := node.Event
	event := &Event{
		StartsAt:     node.StartsAt.UTC(),
		EndsAt:       node.EndsAt.UTC(),
		TimezoneName: details.TimezoneName,
		Capacity:     details.Capacity,
		HasOnlineURL: details.OnlineURL != "",
		CancelledAt:  details.CancelledAt,
		GoingCount:   details.GoingCount,
		MaybeCount:   details.MaybeCount,
		IsFull:       details.Capacity > 0 && details.GoingCount >= uint64(details.Capacity),
		ViewerRsvp:   details.ViewerRsvp,
		State:        EventState(node.StartsAt, node.EndsAt, details.CancelledAt != nil, time.Now()),
		Host:         NewActorBrief(details.Host),
	}

	if node.EventShowsOnlineURL {
		event.OnlineURL = details.OnlineURL
	}

	return event
}
