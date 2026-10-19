package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"fmt"
	"log"
	"notification/data/entities"
)

// eventFormatNodeMediumEdited
func (h *eventHandler) eventFormatNodeMediumEdited(payload *event.EventPayload) error {
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

	// A reply that was edited is nobody's news.
	if object.RootID > 0 {
		return nil
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	recipients, err := h.Srv.SubscriptionsGRPC.BrowseSources(target)
	if err != nil {
		return err
	}

	// Create notification
	n.Type = fmt.Sprintf("%s_edit", getEntityNameFromType(object.Type))
	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Subject.Type = subject.Type
	n.Object.ID = object.ID
	n.Object.Type = object.Type
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

	return nil
}

// eventFormatNodeMediumAdded
func (h *eventHandler) eventFormatNodeMediumAdded(payload *event.EventPayload) error {
	var (
		err    error
		n      = entities.NewNotification()
		viewer = an_entities.NewViewer()
	)

	subject, err := h.Srv.ActorsGRPC.Read(payload.SubjectID, viewer)
	if err != nil {
		return err
	}
	viewer.ID = subject.ID
	viewer.PersonType = subject.Type

	object, err := h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		return err
	}

	// A reply is a note, and arrives as one. It is not a new post for the
	// author's followers to be told about: it is news to the people it
	// answers, and to nobody else.
	if object.RootID > 0 {
		return h.replyAdded(payload, subject, object)
	}

	// A note that quotes a post is news to that post's author, as well as
	// being a new post like any other. Failing to tell them does not stop
	// the note's own notifications.
	if object.QuoteID > 0 {
		if err := h.quoteAdded(payload, subject, object); err != nil {
			log.Printf("telling the author of %d that it was quoted failed: %v", object.QuoteID, err)
		}
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	recipients, err := h.Srv.SubscriptionsGRPC.BrowseSources(target)
	if err != nil {
		return err
	}

	// Create notification
	n.Type = fmt.Sprintf("%s_add", getEntityNameFromType(object.Type))
	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Subject.Type = subject.Type
	n.Object.ID = object.ID
	n.Object.Type = object.Type
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

	return nil
}

// eventFormatNodeMediumDeleted removes everything that told of a post or a
// reply now gone, whoever deleted it: its author, or a moderator.
func (h *eventHandler) eventFormatNodeMediumDeleted(payload *event.EventPayload) error {
	return h.Repos.Notifications.DeleteAbout(payload.ObjectID)
}
