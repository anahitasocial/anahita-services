package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"notification/data/entities"
)

const NotificationTypeAdminAdd = "admin_add"

// eventFormatEdgeAdminAdded is the event format for edge.admin.added
func (h *eventHandler) eventFormatEdgeAdminAdded(payload *event.EventPayload) error {
	var (
		err    error
		n      = entities.NewNotification()
		viewer = an_entities.NewViewer()
	)

	viewer.ID = payload.ViewerID

	subject, err := h.Srv.ActorsGRPC.Read(payload.SubjectID, viewer)
	if err != nil {
		return err
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	recipients, err := h.Srv.ActorAdminsGRPC.Browse(target.ID)
	if err != nil {
		return err
	}

	n.Type = NotificationTypeAdminAdd
	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Subject.Type = subject.Type
	n.Target.ID = target.ID
	n.Target.Type = target.Type

	for _, recipient := range recipients {
		if recipient.ID == subject.ID {
			continue // Skip the subject itself
		}

		n.RecipientID = recipient.ID

		if err = n.Validate(); err != nil {
			return err
		}

		n.ID, err = h.Repos.Notifications.Add(n)
		if err != nil {
			return err
		}
	}

	isSubscribed, err := h.Srv.SubscriptionsGRPC.IsSubscribedTo(subject.ID, target.ID)
	if err != nil {
		return err
	}

	// Create subscription
	if !isSubscribed {
		if err = h.Srv.SubscriptionsGRPC.Add(subject, target, viewer); err != nil {
			return err
		}
	}

	return nil
}

// eventFormatEdgeAdminDeleted is the event format for edge.admin.deleted
func (h *eventHandler) eventFormatEdgeAdminDeleted(payload *event.EventPayload) error {
	var (
		err    error
		n      = entities.NewNotification()
		viewer = an_entities.NewViewer()
	)

	subject, err := h.Srv.ActorsGRPC.Read(payload.SubjectID, viewer)
	if err != nil {
		return err
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	// Create notification
	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Target.ID = target.ID

	if err = h.Repos.Notifications.Delete(n); err != nil {
		return err
	}

	// Delete subscription
	if err = h.Srv.SubscriptionsGRPC.Delete(subject, target); err != nil {
		return err
	}

	return nil
}
