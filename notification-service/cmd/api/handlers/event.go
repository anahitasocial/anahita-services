package handlers

import (
	"anahita-libs/event"
	"log"
	"strings"
)

type eventHandler struct {
	*Env
}

// NewEventHandler creates a new event handler
func NewEventHandler(env *Env) *eventHandler {
	return &eventHandler{
		env,
	}
}

// notificationHandler handles the notification events
func (h *eventHandler) HandleEvents(payload *event.EventPayload) error {
	var err error

	log.Println("Notification service event received", payload.Name)

	// Medium node events
	if strings.Contains(payload.Name, "node.medium.") {
		if strings.Contains(payload.Name, ".edited.v1") {
			err = h.eventFormatNodeMediumEdited(payload)
		} else if strings.Contains(payload.Name, ".added.v1") {
			err = h.eventFormatNodeMediumAdded(payload)
		} else if strings.Contains(payload.Name, ".deleted.v1") {
			err = h.eventFormatNodeMediumDeleted(payload)
		}
	}

	// Comment node events
	if strings.Contains(payload.Name, "node.comment.") {
		if strings.Contains(payload.Name, ".edited.") {
			err = h.eventFormatNodeCommentEdited(payload)
		} else if strings.Contains(payload.Name, ".added.v1") {
			err = h.eventFormatNodeCommentAdded(payload)
		} else if strings.Contains(payload.Name, ".deleted.v1") {
			err = h.eventFormatNodeCommentDeleted(payload)
		}
	}

	// Repost node events
	if strings.Contains(payload.Name, "node.repost.") {
		if strings.Contains(payload.Name, ".added.v1") {
			err = h.eventFormatNodeRepostAdded(payload)
		} else if strings.Contains(payload.Name, ".deleted.v1") {
			err = h.eventFormatNodeRepostDeleted(payload)
		}
	}

	// Admin edge events
	if strings.Contains(payload.Name, "edge.admin.") {
		if strings.Contains(payload.Name, ".added.v1") {
			err = h.eventFormatEdgeAdminAdded(payload)
		} else if strings.Contains(payload.Name, ".deleted.v1") {
			err = h.eventFormatEdgeAdminDeleted(payload)
		}
	}

	// Follow edge events
	if strings.Contains(payload.Name, "edge.follow.") {
		if strings.Contains(payload.Name, ".added.v1") {
			err = h.eventFormatEdgeFollowAdded(payload)
		} else if strings.Contains(payload.Name, ".deleted.v1") {
			err = h.eventFormatEdgeFollowDeleted(payload)
		}
	}

	// Voteup edge events
	if strings.Contains(payload.Name, "edge.voteup.") {
		if strings.Contains(payload.Name, ".added.v1") {
			err = h.eventFormatEdgeVoteupAdded(payload)
		} else if strings.Contains(payload.Name, ".deleted.v1") {
			err = h.eventFormatEdgeVoteupDeleted(payload)
		}
	}

	if err != nil {
		log.Println("Error handling event", payload.Name, err)
		return err
	}

	return nil
}
