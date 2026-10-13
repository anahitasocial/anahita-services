package handlers

import (
	"strings"

	"anahita-libs/constants"
	"anahita-libs/event"
)

// What happens here that this service has to know of.
//
// Somebody here follows somebody on Bluesky the way they follow anybody:
// the same button, the same request to the same service. This hears that
// it happened, on the bus the services already talk over, reads what the
// account posted lately, and has the listener told to hear of it from now
// on.
//
// The bus can lose a message. What is lost is found a minute later by
// CatchUp, which asks the graph who is followed and does not rely on
// having been told.

// EventTopics is what this service listens for.
var EventTopics = []string{
	"edge.follow.*.*.v1",
}

// HandleEvent is told of something that happened here.
func (h *Handlers) HandleEvent(payload *event.EventPayload) error {
	if !strings.HasPrefix(payload.Name, "edge.follow.") {
		return nil
	}

	target, err := h.Nodes.Read(payload.TargetID)
	if err != nil || target == nil || target.Type != constants.NodeActorATProtoPersonEntityType {
		return err
	}
	kept, err := h.Actors.ReadByNode(target.ID)
	if err != nil || kept == nil {
		return err
	}

	if strings.HasSuffix(payload.Name, ".added.v1") {
		h.later(func() { h.backfill(kept) })
	}
	// Followed or no longer followed: whose posts are wanted may have
	// changed either way.
	h.changed()
	return nil
}
