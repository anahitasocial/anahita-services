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

	// Add subscription
	if err = h.Srv.SubscriptionsGRPC.Add(subject, subject, viewer); err != nil {
		return err
	}

	return nil
}

// eventFormatNodeMediumDeleted
func (h *eventHandler) eventFormatNodeMediumDeleted(payload *event.EventPayload) error {
	var (
		err    error
		n      = entities.NewNotification()
		viewer = an_entities.NewViewer()
	)

	// If what went was a reply, the news of it goes with it, whoever was
	// told. First, and whatever else here fails: the rest looks for
	// notifications about a post, which a reply's are not.
	if err := h.Repos.Notifications.DeleteOfReply(payload.ObjectID); err != nil {
		log.Printf("removing the news of reply %d failed: %v", payload.ObjectID, err)
	}

	subject, err := h.Srv.ActorsGRPC.Read(payload.SubjectID, viewer)
	if err != nil {
		return err
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	n.Subject.ID = subject.ID
	n.Object.ID = payload.ObjectID
	n.Target.ID = target.ID

	n.Service = payload.SourceService
	if err = h.Repos.Notifications.Delete(n); err != nil {
		return err
	}

	// Notifications about comments, from before comments became replies,
	// are still stored under the name of the service that made them.
	n.Service = "comment-service"
	if err = h.Repos.Notifications.Delete(n); err != nil {
		return err
	}

	n.Service = "like-service"
	if err = h.Repos.Notifications.Delete(n); err != nil {
		return err
	}

	return nil
}
