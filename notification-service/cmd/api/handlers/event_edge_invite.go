package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"notification/data/entities"
)

const (
	// NotificationTypeActorInvite is being invited to follow an actor. The
	// web app puts Accept and Decline on it.
	NotificationTypeActorInvite = "actor_invite"
	// NotificationTypeActorInviteAccept is somebody you invited saying yes.
	NotificationTypeActorInviteAccept = "actor_invite_accept"
)

// In an invitation's events the object is the person invited, the target
// the actor, and the owner whoever invited them.

// eventFormatEdgeInviteAdded tells the person invited: its subject is
// whoever invited them, and its target what they are invited to.
func (h *eventHandler) eventFormatEdgeInviteAdded(payload *event.EventPayload) error {
	var (
		n      = entities.NewNotification()
		viewer = an_entities.NewViewer()
	)

	viewer.ID = payload.ViewerID

	inviter, err := h.Srv.ActorsGRPC.Read(payload.OwnerID, viewer)
	if err != nil {
		return err
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	n.Type = NotificationTypeActorInvite
	n.Service = payload.SourceService
	n.Subject.ID = inviter.ID
	n.Subject.Type = inviter.Type
	n.Subject.CreatedBy = viewer.ID
	n.Target.ID = target.ID
	n.Target.Type = target.Type
	n.RecipientID = payload.ObjectID

	if err = n.Validate(); err != nil {
		return err
	}

	_, err = h.Repos.Notifications.Add(n)

	return err
}

// eventFormatEdgeInviteEnded takes the invitation out of the invited
// person's notifications: they said no, it was taken back, or it lapsed.
// There is nothing left for its buttons to do.
func (h *eventHandler) eventFormatEdgeInviteEnded(payload *event.EventPayload) error {
	n := entities.NewNotification()

	n.Type = NotificationTypeActorInvite
	n.Service = payload.SourceService
	n.Subject.ID = payload.OwnerID
	n.Target.ID = payload.TargetID
	n.RecipientID = payload.ObjectID

	return h.Repos.Notifications.Delete(n)
}

// eventFormatEdgeInviteAccepted removes the invitation, as above, and tells
// whoever invited them that they said yes.
func (h *eventHandler) eventFormatEdgeInviteAccepted(payload *event.EventPayload) error {
	if err := h.eventFormatEdgeInviteEnded(payload); err != nil {
		return err
	}

	var (
		n      = entities.NewNotification()
		viewer = an_entities.NewViewer()
	)

	viewer.ID = payload.ViewerID

	invitee, err := h.Srv.ActorsGRPC.Read(payload.ObjectID, viewer)
	if err != nil {
		return err
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	// Nobody to tell when the person who invited them is gone.
	if payload.OwnerID <= 0 {
		return nil
	}

	n.Type = NotificationTypeActorInviteAccept
	n.Service = payload.SourceService
	n.Subject.ID = invitee.ID
	n.Subject.Type = invitee.Type
	n.Subject.CreatedBy = viewer.ID
	n.Target.ID = target.ID
	n.Target.Type = target.Type
	n.RecipientID = payload.OwnerID

	if err = n.Validate(); err != nil {
		return err
	}

	_, err = h.Repos.Notifications.Add(n)

	return err
}
