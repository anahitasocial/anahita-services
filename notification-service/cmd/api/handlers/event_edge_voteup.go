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
	var (
		err    error
		n      = entities.NewNotification()
		viewer = an_entities.NewViewer()
	)

	recipient, err := h.Srv.ActorsGRPC.Read(payload.OwnerID, viewer)
	if err != nil {
		return err
	}

	subject, err := h.Srv.ActorsGRPC.Read(payload.SubjectID, viewer)
	if err != nil {
		return err
	}

	if subject.ID == recipient.ID {
		// Do not notify self-voteup
		return nil
	}

	object, err := h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		return err
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	// Create notification
	n.Type = NotificationTypeVoteup
	n.Service = payload.SourceService
	n.RecipientID = recipient.ID
	n.Subject.ID = subject.ID
	n.Subject.Type = subject.Type
	n.Object.ID = object.ID
	n.Object.Type = object.Type
	n.Target.ID = target.ID
	n.Target.Type = target.Type

	if err = n.Validate(); err != nil {
		log.Println("eventFormatEdgeVoteupAdded: notification validation failed:", err)
		return err
	}

	n.ID, err = h.Repos.Notifications.Add(n)
	if err != nil {
		log.Println("eventFormatEdgeVoteupAdded: failed to create notification:", err)
		return err
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

	recipient, err := h.Srv.ActorsGRPC.Read(payload.OwnerID, viewer)
	if err != nil {
		return err
	}

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

	// Delete notification
	n.Service = payload.SourceService
	n.RecipientID = recipient.ID
	n.Subject.ID = subject.ID
	n.Object.ID = object.ID
	n.Target.ID = target.ID

	if err := h.Repos.Notifications.Delete(n); err != nil {
		return err
	}

	// Delete subscription
	if err = h.Srv.SubscriptionsGRPC.Delete(subject, object); err != nil {
		return err
	}

	return nil
}
