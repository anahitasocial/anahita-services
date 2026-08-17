package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"notification/data/entities"
)

const (
	NotificationTypeCommentEdit = "comment_edit"
	NotificationTypeCommentAdd  = "comment_add"
)

// eventFormatNodeCommentEdited is the event format for node.comment.edited
func (h *eventHandler) eventFormatNodeCommentEdited(payload *event.EventPayload) error {
	var (
		err error
		n   = entities.NewNotification()
	)

	subject, err := h.Srv.GraphNodeGRPC.Read(payload.SubjectID)
	if err != nil {
		return err
	}

	object, err := h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		return err
	}

	parent, err := h.Srv.GraphNodeGRPC.Read(object.Parent.ID)
	if err != nil {
		return err
	}

	recipients, err := h.Srv.SubscriptionsGRPC.BrowseSources(parent)
	if err != nil {
		return err
	}

	// Create notification
	n.Type = NotificationTypeCommentEdit
	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Subject.Type = subject.Type
	n.Object.ID = object.ID
	n.Object.Type = object.Type
	n.Target.ID = parent.Owner.ID
	n.Target.Type = parent.Owner.Type

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

	return nil
}

// eventFormatNodeCommentAdded is the event format for node.comment.added
func (h *eventHandler) eventFormatNodeCommentAdded(payload *event.EventPayload) error {
	var (
		err error
		n   = entities.NewNotification()
	)

	subject, err := h.Srv.GraphNodeGRPC.Read(payload.SubjectID)
	if err != nil {
		return err
	}

	object, err := h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		return err
	}

	parent, err := h.Srv.GraphNodeGRPC.Read(object.Parent.ID)
	if err != nil {
		return err
	}

	recipients, err := h.Srv.SubscriptionsGRPC.BrowseSources(parent)
	if err != nil {
		return err
	}

	// Create notification
	n.Type = NotificationTypeCommentAdd
	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Subject.Type = subject.Type
	n.Object.ID = object.ID
	n.Object.Type = object.Type
	n.Target.ID = parent.Owner.ID
	n.Target.Type = parent.Owner.Type

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

	isSubscribed, err := h.Srv.SubscriptionsGRPC.IsSubscribedTo(object.Author.ID, parent.ID)
	if err != nil {
		return err
	}

	// Create subscription
	if !isSubscribed {
		viewer := an_entities.NewViewer()
		viewer.ID = payload.ViewerID
		viewer.UserType = payload.ViewerUserType

		objectAuthor := an_entities.NewAnahitaNode()
		objectAuthor.ID = object.Author.ID
		objectAuthor.Type = object.Author.Type

		if err = h.Srv.SubscriptionsGRPC.Add(objectAuthor, parent, viewer); err != nil {
			return err
		}
	}

	return nil
}

// eventFormatNodeCommentDeleted is the event format for node.comment.deleted
func (h *eventHandler) eventFormatNodeCommentDeleted(payload *event.EventPayload) error {
	var (
		err error
		n   = entities.NewNotification()
	)

	subject, err := h.Srv.GraphNodeGRPC.Read(payload.SubjectID)
	if err != nil {
		return err
	}

	object, err := h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		return err
	}

	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Object.ID = object.ID
	n.Target.ID = object.Owner.ID

	if err = h.Repos.Notifications.Delete(n); err != nil {
		return err
	}

	return nil
}
