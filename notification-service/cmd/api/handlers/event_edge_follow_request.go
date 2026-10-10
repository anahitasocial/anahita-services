package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"notification/data/entities"
)

const (
	// NotificationTypeActorFollowRequest is somebody asking to follow a
	// profile you look after.
	NotificationTypeActorFollowRequest = "actor_follow_request"
	// NotificationTypeActorFollowRequestAccept is a profile you asked to
	// follow saying yes.
	NotificationTypeActorFollowRequestAccept = "actor_follow_request_accept"
)

// eventFormatEdgeFollowRequestAdded tells whoever looks after the profile
// that somebody asked to follow it: the person themselves, or a group's
// administrators.
//
// The event's object is whoever asked, and its target the profile asked.
func (h *eventHandler) eventFormatEdgeFollowRequestAdded(payload *event.EventPayload) error {
	var (
		n      = entities.NewNotification()
		viewer = an_entities.NewViewer()
	)

	viewer.ID = payload.ViewerID

	subject, err := h.Srv.ActorsGRPC.Read(payload.ObjectID, viewer)
	if err != nil {
		return err
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	n.Type = NotificationTypeActorFollowRequest
	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Subject.Type = subject.Type
	n.Subject.CreatedBy = viewer.ID
	n.Target.ID = target.ID
	n.Target.Type = target.Type

	if target.IsPerson() {
		n.RecipientID = target.ID
		if err = n.Validate(); err != nil {
			return err
		}

		_, err = h.Repos.Notifications.Add(n)

		return err
	}

	recipients, err := h.Srv.ActorAdminsGRPC.Browse(target.ID)
	if err != nil {
		return err
	}

	for _, recipient := range recipients {
		if recipient.ID == subject.ID {
			continue
		}

		n.RecipientID = recipient.ID
		if err = n.Validate(); err != nil {
			return err
		}

		if _, err = h.Repos.Notifications.Add(n); err != nil {
			return err
		}
	}

	return nil
}

// eventFormatEdgeFollowRequestDeleted takes the asking back out of
// everybody's notifications: it was accepted, ignored or withdrawn, and
// there is nothing left to answer.
func (h *eventHandler) eventFormatEdgeFollowRequestDeleted(payload *event.EventPayload) error {
	n := entities.NewNotification()

	n.Type = NotificationTypeActorFollowRequest
	n.Service = payload.SourceService
	n.Subject.ID = payload.ObjectID
	n.Target.ID = payload.TargetID

	return h.Repos.Notifications.Delete(n)
}

// eventFormatEdgeFollowRequestAccepted tells whoever asked that they now
// follow the profile. Its subject is the profile, which is what said yes
// as far as they are concerned, whichever administrator pressed the button.
func (h *eventHandler) eventFormatEdgeFollowRequestAccepted(payload *event.EventPayload) error {
	var (
		n      = entities.NewNotification()
		viewer = an_entities.NewViewer()
	)

	viewer.ID = payload.ViewerID

	asker, err := h.Srv.ActorsGRPC.Read(payload.ObjectID, viewer)
	if err != nil {
		return err
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	n.Type = NotificationTypeActorFollowRequestAccept
	n.Service = payload.SourceService
	n.Subject.ID = target.ID
	n.Subject.Type = target.Type
	n.Subject.CreatedBy = viewer.ID
	n.Target.ID = target.ID
	n.Target.Type = target.Type
	n.RecipientID = asker.ID

	if err = n.Validate(); err != nil {
		return err
	}

	_, err = h.Repos.Notifications.Add(n)

	return err
}
