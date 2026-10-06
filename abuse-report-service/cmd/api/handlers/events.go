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
// nothing left to decide about something that no longer exists. When the node
// was a person, the reports that person filed are removed as well.
//
// Posts and replies are gone the moment they are deleted. A person or a
// group is not: deleting one starts a 30-day wait during which it can be
// restored, so its case stays open until it is purged.
//
// Closing the case is a convenience, not the record. The bus can drop a
// message, and when it does the case simply stays open until an administrator
// opens it, which checks again. Nothing checks again for a purged person's
// reports: if that message is lost they stay, naming an account that is now
// only a tombstone.
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

	// A purged person's own reports go with them: what they wrote in one is
	// theirs, and a purge removes everything a person wrote.
	if strings.HasPrefix(payload.Name, "node.actor.") {
		removed, err := h.Repos.Cases.RemoveReportsBy(payload.ObjectID)
		if err != nil {
			log.Printf("abuse report: could not remove the reports filed by %d: %v", payload.ObjectID, err)
			return err
		}
		if removed > 0 {
			log.Printf("abuse report: actor %d was purged, removed %d report(s) it filed", payload.ObjectID, removed)
		}
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
