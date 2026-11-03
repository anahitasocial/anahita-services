package entities

import "time"

// EventDetails is what only an event has, beside the node every actor is,
// and how many have answered.
type EventDetails struct {
	NodeID int64
	// The zone the times were given in, as IANA names it.
	TimezoneName string
	// How many may be going. 0 for no limit.
	Capacity uint32
	// Where to join when it is online. Not for everybody: see
	// permissions.CanSeeOnlineURL.
	OnlineURL string
	// Set when it was called off.
	CancelledAt *time.Time
	GoingCount  uint64
	MaybeCount  uint64
	// What the viewer has answered: going, maybe, or "".
	ViewerRsvp string
	// The group hosting it, or 0.
	HostID int64
	// Whether everybody who follows the host may see that it is on and
	// answer it.
	OpenToHostFollowers bool
	// That group, when the viewer may see it. Filled in by whoever sends
	// the event on.
	Host *AnahitaNode
}

// EventDiscoverFilter narrows the list of what is on (GET /events/). The
// zero value is every event the viewer may see.
type EventDiscoverFilter struct {
	// On at some time between these: each nil for no bound.
	From *time.Time
	To   *time.Time
	// Only the ones with an address to join online.
	Online bool
	// Only the ones somebody the viewer follows hosts, looks after or said
	// going or maybe to.
	Following bool
}
