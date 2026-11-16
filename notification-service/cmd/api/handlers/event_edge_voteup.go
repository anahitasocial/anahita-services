package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"log"
	"notification/data/entities"
)

const NotificationTypeVoteup = "voteup"

// eventFormatEdgeVoteupAdded is the event format for edge.voteup.added
func (h *eventHandler) eventFormatEdgeVoteupAdded(payload *event.EventPayload) error {
	viewer := an_entities.NewViewer()

	subject, err := h.Srv.ActorsGRPC.Read(payload.SubjectID, viewer)
	if err != nil {
		return err
	}
	object, err := h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		return err
	}
	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	// Its author, or a channel's admins, whoever liked it (what is yours
	// tells you of everyone); and its subscribers, when they follow whoever
	// liked it (roadmap row 146). Whoever wrote something from another
	// server is told by their own server, which is sent the like.
	following, err := h.subscribersFollowing(object, subject.ID)
	if err != nil {
		return err
	}
	for _, recipient := range uniqueOthers(subject.ID, h.peopleBehind(payload.OwnerID), following) {
		if !h.mayRead(recipient, object.ID) {
			continue
		}
		n := entities.NewNotification()
		n.Type = NotificationTypeVoteup
		n.Service = payload.SourceService
		n.RecipientID = recipient
		n.Subject.ID = subject.ID
		n.Subject.Type = subject.Type
		n.Object.ID = object.ID
		n.Object.Type = object.Type
		n.Target.ID = target.ID
		n.Target.Type = target.Type

		if err := n.Validate(); err != nil {
			log.Println("eventFormatEdgeVoteupAdded: notification validation failed:", err)
			return err
		}
		if _, err := h.add(n); err != nil {
			log.Println("eventFormatEdgeVoteupAdded: failed to create notification:", err)
			return err
		}
	}

	return nil
}

// eventFormatEdgeVoteupDeleted is the event format for edge.voteup.deleted
func (h *eventHandler) eventFormatEdgeVoteupDeleted(payload *event.EventPayload) error {
	var (
		err    error
		n      = entities.NewNotification()
		viewer = an_entities.NewViewer()
	)

	subject, err := h.Srv.ActorsGRPC.Read(payload.SubjectID, viewer)
	if err != nil {
		return err
	}

	object, err := h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		return err
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	// The like's notification, and nothing else: a like makes no
	// subscription, so taking it back ends none.
	// Of everybody it was told to: its author, a channel's admins, and
	// its subscribers (no recipient named).
	n.Type = NotificationTypeVoteup
	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Object.ID = object.ID
	n.Target.ID = target.ID

	return h.Repos.Notifications.Delete(n)
}
