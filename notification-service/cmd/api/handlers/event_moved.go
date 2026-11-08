package handlers

import (
	"anahita-libs/event"

	"notification/data/entities"
)

const (
	// Somebody followed moved their account elsewhere, and the follower
	// follows the new one now (roadmap row 124, part 2). The subject is the
	// old profile, the target the new one.
	NotificationTypeActorMoved = "actor_moved"
	// The same, where the new account approves its followers, and the
	// follower has asked.
	NotificationTypeActorMovedAsked = "actor_moved_asked"
)

// remoteMoved tells a follower that somebody they follow moved, and what
// was done for them: said by activitypub-service once for each follower.
func (h *eventHandler) remoteMoved(payload *event.EventPayload) error {
	kind := NotificationTypeActorMoved
	switch payload.Name {
	case event.RemoteMovedFollowed:
	case event.RemoteMovedAsked:
		kind = NotificationTypeActorMovedAsked
	default:
		return nil
	}
	if payload.SubjectID == 0 || payload.ObjectID == 0 || payload.TargetID == 0 {
		return nil
	}

	n := entities.NewNotification()
	n.Type = kind
	n.Service = payload.SourceService
	n.RecipientID = payload.TargetID
	n.Subject.ID = payload.SubjectID
	n.Subject.Type = payload.SubjectType
	n.Target.ID = payload.ObjectID
	n.Target.Type = payload.ObjectType
	if err := n.Validate(); err != nil {
		return err
	}
	_, err := h.add(n)
	return err
}
