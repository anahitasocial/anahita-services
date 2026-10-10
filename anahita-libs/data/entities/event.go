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
	// That group, when the viewer may see it. Filled in by whoever sends
	// the event on.
	Host *AnahitaNode
}
