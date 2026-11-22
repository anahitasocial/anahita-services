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
		if payload.Action() != event.ActionDeleted {
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

	// Somebody followed from here moved their account, and the follower
	// follows the new one now, or has asked to (event_moved.go).
	if strings.HasPrefix(payload.Name, "federation.moved.") {
		return h.remoteMoved(payload)
	}

	// A note kept from another server, or one that was and is gone. A
	// change to it is news to nobody.
	if strings.HasPrefix(payload.Name, "federation.medium.") {
		if payload.Action() == event.ActionAdded {
			return h.remoteNoteAdded(payload)
		}
		// Taken away where it was written: the news of it goes too.
		if payload.Action() == event.ActionDeleted {
			return h.Repos.Notifications.DeleteAbout(payload.ObjectID)
		}
		return nil
	}

	// The site's administrators gave a post a content warning.
	if payload.Name == event.EventMediumWarned {
		return h.eventMediumWarned(payload)
	}

	if strings.Contains(payload.Name, "node.medium.") {
		// An edit tells nobody (the user, 2026-10-19): a name it newly
		// adds is a mention, told as one by actor-service.
		if payload.Action() == event.ActionAdded {
			err = h.eventFormatNodeMediumAdded(payload)
		} else if payload.Action() == event.ActionDeleted {
			err = h.eventFormatNodeMediumDeleted(payload)
		}
	}

	// Repost node events
	if strings.Contains(payload.Name, "node.repost.") {
		if payload.Action() == event.ActionAdded {
			err = h.eventFormatNodeRepostAdded(payload)
		} else if payload.Action() == event.ActionDeleted {
			err = h.eventFormatNodeRepostDeleted(payload)
		}
	}

	// Admin edge events
	if strings.Contains(payload.Name, "edge.admin.") {
		if payload.Action() == event.ActionAdded {
			err = h.eventFormatEdgeAdminAdded(payload)
		} else if payload.Action() == event.ActionDeleted {
			err = h.eventFormatEdgeAdminDeleted(payload)
		}
	}

	// A post naming somebody, and no longer naming them after an edit.
	if strings.HasPrefix(payload.Name, "edge.mention.") {
		if payload.Action() == event.ActionAdded {
			err = h.eventFormatEdgeMentionAdded(payload)
		} else if payload.Action() == event.ActionDeleted {
			err = h.eventFormatEdgeMentionDeleted(payload)
		}
	}

	// A person in a picture or a video: asked, tagged or approved, and
	// taken off (roadmap row 103).
	if strings.HasPrefix(payload.Name, "edge.tag.") {
		switch {
		case payload.Action() == event.ActionRequested:
			err = h.eventFormatEdgeTagRequested(payload)
		case payload.Action() == event.ActionAdded:
			err = h.eventFormatEdgeTagAdded(payload)
		case payload.Action() == event.ActionDeleted:
			err = h.eventFormatEdgeTagDeleted(payload)
		}
	}

	// A block: what the two were told of each other goes. The graph has
	// ended the follows and the subscriptions already.
	if strings.HasPrefix(payload.Name, "edge.block.") && payload.Action() == event.ActionAdded {
		err = h.Repos.Notifications.DeleteBetween(payload.ObjectID, payload.TargetID)
	}

	// Follow edge events
	if strings.Contains(payload.Name, "edge.follow.") {
		if payload.Action() == event.ActionAdded {
			err = h.eventFormatEdgeFollowAdded(payload)
		} else if payload.Action() == event.ActionDeleted {
			err = h.eventFormatEdgeFollowDeleted(payload)
		}
	}

	// Requests to follow. Not caught by the test above: that one ends in
	// a full stop, and this name goes on with a hyphen.
	if strings.Contains(payload.Name, "edge.follow-request.") {
		if payload.Action() == event.ActionAdded {
			err = h.eventFormatEdgeFollowRequestAdded(payload)
		} else if payload.Action() == event.ActionDeleted {
			err = h.eventFormatEdgeFollowRequestDeleted(payload)
		} else if payload.Action() == event.ActionAccepted {
			err = h.eventFormatEdgeFollowRequestAccepted(payload)
		}
	}

	// Invitations to follow an actor.
	if strings.Contains(payload.Name, "edge.invite.") {
		if payload.Action() == event.ActionAdded {
			err = h.eventFormatEdgeInviteAdded(payload)
		} else if payload.Action() == event.ActionAccepted {
			err = h.eventFormatEdgeInviteAccepted(payload)
		} else if payload.Action() == event.ActionDeclined || payload.Action() == event.ActionDeleted {
			err = h.eventFormatEdgeInviteEnded(payload)
		}
	}

	// Events, the kind people go to. The name starts with it, where the
	// others here are found anywhere in theirs.
	if strings.HasPrefix(payload.Name, "event.") {
		if payload.Action() == event.ActionAnswered {
			err = h.eventFormatEventAnswered(payload)
		} else if payload.Action() == event.ActionChanged {
			err = h.eventFormatEventTold(payload, NotificationTypeEventChanged)
		} else if payload.Action() == event.ActionCancelled {
			err = h.eventFormatEventTold(payload, NotificationTypeEventCancelled)
		} else if payload.Action() == event.ActionReminded {
			err = h.eventFormatEventTold(payload, NotificationTypeEventReminder)
		}
	}

	// Voteup edge events
	if strings.Contains(payload.Name, "edge.voteup.") {
		if payload.Action() == event.ActionAdded {
			err = h.eventFormatEdgeVoteupAdded(payload)
		} else if payload.Action() == event.ActionDeleted {
			err = h.eventFormatEdgeVoteupDeleted(payload)
		}
	}

	if err != nil {
		log.Println("Error handling event", payload.Name, err)
		return err
	}

	return nil
}
