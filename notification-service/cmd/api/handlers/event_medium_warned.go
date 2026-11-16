package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"notification/data/entities"
)

// NotificationTypeMediumWarned tells an author that the site's
// administrators gave their post a content warning (roadmap row 122,
// phase 4), which they may not take off.
const NotificationTypeMediumWarned = "medium_warned"

// eventMediumWarned tells the author of a post, a person here, that the
// site's administrators gave it a content warning. The administrator is not
// named: it was the site that decided, and the author is its subject, so
// muting one of them does not hide it. No choice turns it off.
func (h *eventHandler) eventMediumWarned(payload *event.EventPayload) error {
	viewer := an_entities.NewViewer()

	author, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}
	if !author.IsPerson() || author.IsRemote() || author.ID == payload.SubjectID {
		return nil
	}

	n := entities.NewNotification()
	n.Type = NotificationTypeMediumWarned
	n.Service = payload.SourceService
	n.Subject.ID = author.ID
	n.Subject.Type = author.Type
	n.Object.ID = payload.ObjectID
	n.Object.Type = payload.ObjectType
	n.Target.ID = author.ID
	n.Target.Type = author.Type
	n.RecipientID = author.ID
	if err := n.Validate(); err != nil {
		return err
	}
	_, err = h.add(n)
	return err
}
