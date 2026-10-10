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
	// Where it is held, when it is somewhere. Sent to the same people as
	// the address to join online, and for the same reason: it may be
	// somebody's home. HasAddress says there is one all the same.
	Address    *EventAddress `json:"address,omitempty"`
	HasAddress bool          `json:"has_address"`
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
	// Whether everybody who follows the host may see that it is on and
	// answer it.
	OpenToHostFollowers bool `json:"open_to_host_followers,omitempty"`
}

// EventAddress is where an event is held, as it was typed: the node's own
// address fields. It is not a place on the site's map. Nobody else can find
// it, tag it or search for it.
type EventAddress struct {
	Street        string `json:"street,omitempty"`
	City          string `json:"city,omitempty"`
	StateProvince string `json:"state_province,omitempty"`
	PostalCode    string `json:"postal_code,omitempty"`
	Country       string `json:"country,omitempty"`
	// Where that is on a map, when whoever made the event asked for one
	// and the address was found. Left out otherwise.
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

// NewEventAddress reads an event's address off its node, or nil when it
// has none.
func NewEventAddress(node *entities.AnahitaNode) *EventAddress {
	if node == nil {
		return nil
	}

	address := &EventAddress{
		Street:        node.GeoAddress,
		City:          node.GeoCity,
		StateProvince: node.GeoStateProvince,
		PostalCode:    node.GeoPostalcode,
		Country:       node.GeoCountry,
	}
	if *address == (EventAddress{}) {
		return nil
	}

	// A point with no address beside it is not shown: the address is what
	// it is a point for.
	address.Latitude = node.GeoLatitude
	address.Longitude = node.GeoLongitude

	return address
}

// OneLine is the address as it is written on one line, for a calendar.
func (a *EventAddress) OneLine() string {
	if a == nil {
		return ""
	}

	line := ""
	for _, part := range []string{a.Street, a.City, a.StateProvince, a.PostalCode, a.Country} {
		if part == "" {
			continue
		}
		if line != "" {
			line += ", "
		}
		line += part
	}

	return line
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
		StartsAt:            node.StartsAt.UTC(),
		EndsAt:              node.EndsAt.UTC(),
		TimezoneName:        details.TimezoneName,
		Capacity:            details.Capacity,
		HasOnlineURL:        details.OnlineURL != "",
		CancelledAt:         details.CancelledAt,
		GoingCount:          details.GoingCount,
		MaybeCount:          details.MaybeCount,
		IsFull:              details.Capacity > 0 && details.GoingCount >= uint64(details.Capacity),
		ViewerRsvp:          details.ViewerRsvp,
		State:               EventState(node.StartsAt, node.EndsAt, details.CancelledAt != nil, time.Now()),
		Host:                NewActorBrief(details.Host),
		OpenToHostFollowers: details.OpenToHostFollowers,
	}

	address := NewEventAddress(node)
	event.HasAddress = address != nil

	// Where to turn up, online or in person, is for whoever is going and
	// whoever looks after the event. The one flag decides both.
	if node.EventShowsOnlineURL {
		event.OnlineURL = details.OnlineURL
		event.Address = address
	}

	return event
}
