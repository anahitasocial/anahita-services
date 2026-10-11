package handlers

import (
	"log"
	"strings"

	"anahita-libs/event"
)

// What the service is told about by the rest of the installation: an actor
// removed for good. What it chose, and what was chosen about it, goes with
// it.
//
// The bus can lose a message, and nothing here depends on its not doing so:
// a row left behind names an id that is suggested to nobody and that nobody
// can be.

type eventsHandler struct {
	*Env
}

func (h *eventsHandler) Handle(payload *event.EventPayload) error {
	if payload == nil || payload.ObjectID == 0 {
		return nil
	}
	if !strings.HasPrefix(payload.Name, "node.actor.") || !strings.Contains(payload.Name, ".purged.") {
		return nil
	}

	if err := h.Repos.Preferences.Forget(payload.ObjectID); err != nil {
		log.Printf("recommendations: could not forget the choices of %d: %v", payload.ObjectID, err)
		return err
	}
	if err := h.Repos.Dismissals.Forget(payload.ObjectID); err != nil {
		log.Printf("recommendations: could not forget the dismissals of %d: %v", payload.ObjectID, err)
		return err
	}

	return nil
}
