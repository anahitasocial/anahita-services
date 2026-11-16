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

	// Its author, or a channel's admins, whoever quoted it: what is yours
	// tells you of everyone. A post from another server: its author is told
	// by their own server, which is sent the quote (peopleBehind: nobody).
	for _, recipient := range uniqueOthers(subject.ID, h.peopleBehind(quoted.CreatedBy)) {
		// Told only when they may see the note that quotes them.
		if !h.mayRead(recipient, note.ID) {
			continue
		}
		if err := h.quoteTold(payload, subject, note, recipient); err != nil {
			return err
		}
	}
	return nil
}

// quoteTold tells one person that a note quotes what is theirs.
func (h *eventHandler) quoteTold(payload *event.EventPayload, subject *an_entities.AnahitaNode, note *an_entities.AnahitaNode, recipient int64) error {
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

	_, err := h.add(n)
	return err
}
