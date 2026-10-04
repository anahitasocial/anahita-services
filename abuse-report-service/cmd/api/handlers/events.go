package handlers

import (
	"log"
	"strings"

	"anahita-libs/event"
)

type eventsHandler struct {
	*Env
}

// Handle hears that a node was removed, and closes its open case: there is
// nothing left to decide about something that no longer exists.
//
// Posts and comments are gone the moment they are deleted. A person or a
// group is not: deleting one starts a 30-day wait during which it can be
// restored, so its case stays open until it is purged.
//
// This is a convenience, not the record. The bus can drop a message, and
// when it does the case simply stays open until an administrator opens it,
// which checks again.
func (h *eventsHandler) Handle(payload *event.EventPayload) error {
	if payload == nil || payload.ObjectID == 0 {
		return nil
	}

	gone := strings.Contains(payload.Name, ".purged.")
	if !strings.HasPrefix(payload.Name, "node.actor.") {
		gone = gone || strings.Contains(payload.Name, ".deleted.")
	}
	if !gone {
		return nil
	}

	closed, err := h.Repos.Cases.MarkTargetGone(payload.ObjectID)
	if err != nil {
		log.Printf("abuse report: could not record that node %d is gone: %v", payload.ObjectID, err)
		return err
	}
	if closed > 0 {
		log.Printf("abuse report: node %d is gone, closed its open case", payload.ObjectID)
	}

	return nil
}
