package handlers

import (
	"anahita-libs/event"
	"log"
	"notification/data/entities"
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
	// A repost of something here, by somebody on another server, taken
	// back there: the news of it goes. The payload names who reposted and
	// what, since the repost itself is gone.
	if strings.HasPrefix(payload.Name, "federation.repost.") {
		if !strings.HasSuffix(payload.Name, ".deleted.v1") {
			return nil
		}
		gone := entities.NewNotification()
		gone.Type = "repost_add"
		gone.Service = payload.SourceService
		gone.Subject.ID = payload.SubjectID
		gone.Object.ID = payload.ObjectID
		gone.Target.ID = payload.TargetID
		return h.Repos.Notifications.Delete(gone)
	}

	// A note kept from another server, or one that was and is gone. A
	// change to it is news to nobody.
	if strings.HasPrefix(payload.Name, "federation.medium.") {
		if strings.HasSuffix(payload.Name, ".added.v1") {
			return h.remoteNoteAdded(payload)
		}
		// Taken away where it was written: the news of it goes too.
		if strings.HasSuffix(payload.Name, ".deleted.v1") {
			return h.Repos.Notifications.DeleteAbout(payload.ObjectID)
		}
		return nil
	}

	if strings.Contains(payload.Name, "node.medium.") {
		if strings.Contains(payload.Name, ".edited.v1") {
			err = h.eventFormatNodeMediumEdited(payload)
		} else if strings.Contains(payload.Name, ".added.v1") {
			err = h.eventFormatNodeMediumAdded(payload)
		} else if strings.Contains(payload.Name, ".deleted.v1") {
			err = h.eventFormatNodeMediumDeleted(payload)
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

	// A post naming somebody, and no longer naming them after an edit.
	if strings.HasPrefix(payload.Name, "edge.mention.") {
		if strings.HasSuffix(payload.Name, ".added.v1") {
			err = h.eventFormatEdgeMentionAdded(payload)
		} else if strings.HasSuffix(payload.Name, ".deleted.v1") {
			err = h.eventFormatEdgeMentionDeleted(payload)
		}
	}

	// A block: what the two were told of each other goes. The graph has
	// ended the follows and the subscriptions already.
	if strings.HasPrefix(payload.Name, "edge.block.") && strings.HasSuffix(payload.Name, ".added.v1") {
		err = h.Repos.Notifications.DeleteBetween(payload.ObjectID, payload.TargetID)
	}

	// Follow edge events
	if strings.Contains(payload.Name, "edge.follow.") {
		if strings.Contains(payload.Name, ".added.v1") {
			err = h.eventFormatEdgeFollowAdded(payload)
		} else if strings.Contains(payload.Name, ".deleted.v1") {
			err = h.eventFormatEdgeFollowDeleted(payload)
		}
	}

	// Requests to follow. Not caught by the test above: that one ends in
	// a full stop, and this name goes on with a hyphen.
	if strings.Contains(payload.Name, "edge.follow-request.") {
		if strings.Contains(payload.Name, ".added.v1") {
			err = h.eventFormatEdgeFollowRequestAdded(payload)
		} else if strings.Contains(payload.Name, ".deleted.v1") {
			err = h.eventFormatEdgeFollowRequestDeleted(payload)
		} else if strings.Contains(payload.Name, ".accepted.v1") {
			err = h.eventFormatEdgeFollowRequestAccepted(payload)
		}
	}

	// Invitations to follow an actor.
	if strings.Contains(payload.Name, "edge.invite.") {
		if strings.Contains(payload.Name, ".added.v1") {
			err = h.eventFormatEdgeInviteAdded(payload)
		} else if strings.Contains(payload.Name, ".accepted.v1") {
			err = h.eventFormatEdgeInviteAccepted(payload)
		} else if strings.Contains(payload.Name, ".declined.v1") || strings.Contains(payload.Name, ".deleted.v1") {
			err = h.eventFormatEdgeInviteEnded(payload)
		}
	}

	// Events, the kind people go to. The name starts with it, where the
	// others here are found anywhere in theirs.
	if strings.HasPrefix(payload.Name, "event.") {
		if strings.HasSuffix(payload.Name, ".answered.v1") {
			err = h.eventFormatEventAnswered(payload)
		} else if strings.HasSuffix(payload.Name, ".changed.v1") {
			err = h.eventFormatEventTold(payload, NotificationTypeEventChanged)
		} else if strings.HasSuffix(payload.Name, ".cancelled.v1") {
			err = h.eventFormatEventTold(payload, NotificationTypeEventCancelled)
		} else if strings.HasSuffix(payload.Name, ".reminded.v1") {
			err = h.eventFormatEventTold(payload, NotificationTypeEventReminder)
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
