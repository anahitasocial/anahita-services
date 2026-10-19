package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"notification/data/entities"
)

// NotificationTypeQuoteAdd is somebody quoting a post of yours.
const NotificationTypeQuoteAdd = "quote_add"

// quoteAdded tells the author of a post that a note quotes it.
//
// The notification points at the note that quotes, which is what there is to
// go and read, and from where the author can take their post out of it.
// Nobody is told about quoting their own post.
//
// Somebody the author has blocked is not heard from here, because they
// cannot quote the author in the first place: see permissions.CanQuote.
func (h *eventHandler) quoteAdded(payload *event.EventPayload, subject *an_entities.AnahitaNode, note *an_entities.AnahitaNode) error {
	quoted, err := h.Srv.GraphNodeGRPC.Read(note.QuoteID)
	if err != nil {
		return err
	}

	// A post from another server: its author is told by their own server,
	// which is sent the quote. Nothing is made for them here.
	if quoted.IsRemote() {
		return nil
	}

	recipient := quoted.CreatedBy
	if recipient <= 0 || recipient == subject.ID {
		return nil
	}

	n := entities.NewNotification()
	n.Type = NotificationTypeQuoteAdd
	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Subject.Type = subject.Type
	n.Object.ID = note.ID
	n.Object.Type = note.Type
	n.Target.ID = note.OwnerID
	n.Target.Type = note.OwnerType
	n.RecipientID = recipient

	if err := n.Validate(); err != nil {
		return err
	}

	_, err = h.Repos.Notifications.Add(n)
	return err
}
