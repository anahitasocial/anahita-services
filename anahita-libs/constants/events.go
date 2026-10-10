package constants

import "time"

// Events: an actor people say they are going to.
//
// Saying so is following the event. The answer is in the follow edge's
// meta, written whole and compared whole so a query can match it without
// reading JSON. A follow with anything else there, made by following the
// event the ordinary way, counts as maybe.
const (
	EventRsvpGoing = "going"
	EventRsvpMaybe = "maybe"

	EventRsvpGoingMeta = `{"rsvp":"going"}`
	EventRsvpMaybeMeta = `{"rsvp":"maybe"}`
)

// What became of an answer.
const (
	// EventRsvpSaved: it is recorded, or taken back.
	EventRsvpSaved = "saved"
	// EventRsvpFull: every place is taken. Maybe is still open.
	EventRsvpFull = "full"
	// EventRsvpClosed: the event is over, or was called off.
	EventRsvpClosed = "closed"
	// EventRsvpMissing: there is no such event.
	EventRsvpMissing = "missing"
)

// The lists of events that are one person's.
const (
	// EventListUpcoming: what they said going or maybe to, still to come
	// or on now, soonest first.
	EventListUpcoming = "upcoming"
	// EventListPast: what they said going or maybe to, now over, latest
	// first.
	EventListPast = "past"
	// EventListHosting: what they look after, latest first.
	EventListHosting = "hosting"
	// EventListInvited: what they were invited to and have not answered,
	// still to come, soonest first.
	EventListInvited = "invited"
)

// EventArchiveAfter is how long after it ends an event is archived.
const EventArchiveAfter = 30 * 24 * time.Hour

// EventReminderBefore is how long before it starts whoever is going is
// reminded.
const EventReminderBefore = 24 * time.Hour

// EventRsvpMeta is what the follow edge holds for an answer, or "" for an
// answer there is no such thing as.
func EventRsvpMeta(rsvp string) string {
	switch rsvp {
	case EventRsvpGoing:
		return EventRsvpGoingMeta
	case EventRsvpMaybe:
		return EventRsvpMaybeMeta
	}
	return ""
}

// EventRsvpFromMeta reads an answer back. A follow that says nothing is a
// maybe: somebody who follows an event is at least interested.
func EventRsvpFromMeta(meta string) string {
	if meta == EventRsvpGoingMeta {
		return EventRsvpGoing
	}
	return EventRsvpMaybe
}
