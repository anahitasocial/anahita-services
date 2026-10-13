package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"notification/data/entities"
)

// eventFormatNodeRepostAdded
func (h *eventHandler) eventFormatNodeRepostAdded(payload *event.EventPayload) error {
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

	objectParent, err := h.Srv.GraphNodeGRPC.Read(object.Parent.ID)
	if err != nil {
		return err
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	recipient, err := h.Srv.ActorsGRPC.Read(objectParent.Author.ID, viewer)
	if err != nil {
		return err
	}

	// Whoever wrote something from another server is told by their own
	// server, which is sent the repost. Nothing is made for them here.
	if recipient.IsRemote() {
		return nil
	}

	// Create notification
	n.Type = "repost_add"
	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Subject.Type = subject.Type
	n.Object.ID = objectParent.ID
	n.Object.Type = objectParent.Type
	n.Target.ID = target.ID
	n.Target.Type = target.Type

	if recipient.ID == subject.ID {
		return nil // Skip the subject itself
	}

	n.RecipientID = recipient.ID
	if err = n.Validate(); err != nil {
		return err
	}

	n.ID, err = h.Repos.Notifications.Add(n)
	if err != nil {
		return err
	}

	return nil
}

// eventFormatNodeRepostDeleted
func (h *eventHandler) eventFormatNodeRepostDeleted(payload *event.EventPayload) error {
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

	n.Subject.ID = subject.ID
	n.Object.ID = object.Parent.ID
	n.Target.ID = target.ID
	n.Service = payload.SourceService
	if err = h.Repos.Notifications.Delete(n); err != nil {
		return err
	}

	return nil
}
