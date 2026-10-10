package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"notification/data/entities"
)

const (
	// NotificationTypeEventRsvp is somebody saying they are going to an
	// event you look after.
	NotificationTypeEventRsvp = "event_rsvp"
	// NotificationTypeEventChanged is an event you answered moving to
	// another time.
	NotificationTypeEventChanged = "event_changed"
	// NotificationTypeEventCancelled is an event you answered being called
	// off.
	NotificationTypeEventCancelled = "event_cancelled"
	// NotificationTypeEventReminder is an event you are going to starting
	// within a day.
	NotificationTypeEventReminder = "event_reminder"
)

// In these the target is always the event.

// eventFormatEventAnswered tells whoever looks after an event that somebody
// is going. The object is that person.
func (h *eventHandler) eventFormatEventAnswered(payload *event.EventPayload) error {
	var (
		n      = entities.NewNotification()
		viewer = an_entities.NewViewer()
	)

	viewer.ID = payload.ViewerID

	person, err := h.Srv.ActorsGRPC.Read(payload.ObjectID, viewer)
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

	n.Type = NotificationTypeEventRsvp
	n.Service = payload.SourceService
	n.Subject.ID = person.ID
	n.Subject.Type = person.Type
	n.Subject.CreatedBy = viewer.ID
	n.Target.ID = target.ID
	n.Target.Type = target.Type

	// Said once for each person and event: somebody who goes back and
	// forth between going and maybe is not announced each time.
	if err = h.Repos.Notifications.Delete(n); err != nil {
		return err
	}

	for _, recipient := range recipients {
		// Not to themselves, when it is their own event.
		if recipient.ID == person.ID {
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

// eventFormatEventTold tells one person something about an event they
// answered: it moved, it was called off, or it is nearly time. The object
// is that person. Its subject is the event itself, which is what the news
// is about, whoever it was that changed it.
func (h *eventHandler) eventFormatEventTold(payload *event.EventPayload, kind string) error {
	var (
		n      = entities.NewNotification()
		viewer = an_entities.NewViewer()
	)

	viewer.ID = payload.ViewerID

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	n.Type = kind
	n.Service = payload.SourceService
	n.Subject.ID = target.ID
	n.Subject.Type = target.Type
	n.Subject.CreatedBy = viewer.ID
	n.Target.ID = target.ID
	n.Target.Type = target.Type
	n.RecipientID = payload.ObjectID

	if err = n.Validate(); err != nil {
		return err
	}

	// One of each kind for an event and a person. An event moved three
	// times says so once, with the time it has now on its page.
	if err = h.Repos.Notifications.Delete(n); err != nil {
		return err
	}

	_, err = h.Repos.Notifications.Add(n)

	return err
}
