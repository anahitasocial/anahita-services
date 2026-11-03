package responses

import "anahita-libs/responses"

// The shapes of the kinds of profile other than a person, as the reference
// shows them (docs/api/actor-service.json). Each is what every profile is
// sent, ActorNode, without what only another kind has: they describe, and
// the handlers answer with ActorNode itself, so what is sent cannot differ
// from them by more than the fields left out, which that kind never has.

// GroupNode is a group, as it is sent: followed by people, looked after by
// its admins, and able to host events.
type GroupNode struct {
	responses.ActorNode

	_ struct{} `json:"-" apidoc:"omit=event,quote_policy"`
}

// EventNode is an event, as it is sent: a group with a time and a place, and
// `event` saying when it is, how many may go and are going, and what the
// viewer answered.
type EventNode struct {
	responses.ActorNode

	_ struct{} `json:"-" apidoc:"omit=quote_policy"`
}

// WebsiteNode is a website, as it is sent: a site's feed, followed as a
// profile, whose domain was proved when it was connected.
type WebsiteNode struct {
	responses.ActorNode

	_ struct{} `json:"-" apidoc:"omit=event,quote_policy"`
}
