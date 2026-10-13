package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"notification/data/entities"
)

// NotificationTypeReplyAdd is somebody answering a post or a reply of yours.
const NotificationTypeReplyAdd = "reply_add"

// remoteNoteAdded is a note from another server having been kept here. It
// is said on the bus by a name of its own (anahita-libs/event/remote.go),
// so that nothing takes it for a post written here: its author's followers
// are not told of it, and nobody it names is told they were mentioned.
//
// One thing about it is news to somebody here: when it answers something
// of theirs. They are told as for a reply written here.
func (h *eventHandler) remoteNoteAdded(payload *event.EventPayload) error {
	reply, err := h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		return err
	}
	if reply == nil || reply.ID == 0 || reply.RootID <= 0 || !reply.IsRemote() {
		return nil
	}

	subject, err := h.Srv.ActorsGRPC.Read(reply.OwnerID, an_entities.NewViewer())
	if err != nil {
		return err
	}
	return h.replyAdded(payload, subject, reply)
}

// replyAdded tells the people a reply answers that it was made: the author of
// the post at the top of the thread, and the author of what was directly
// answered when that is somebody else.
//
// Nobody else. Not the replier's followers, who would otherwise be told about
// every answer anybody they follow gives anywhere.
//
// The notification points at the post at the top of the thread, which is
// where the reply is read. The reply's own id is kept in comment_id, the
// column that already means "the thing said under the object", so that
// removing the reply or purging its author can find the notification.
func (h *eventHandler) replyAdded(payload *event.EventPayload, subject *an_entities.AnahitaNode, reply *an_entities.AnahitaNode) error {
	root, err := h.Srv.GraphNodeGRPC.Read(reply.RootID)
	if err != nil {
		return err
	}

	// Whoever wrote something that came from another server is told by
	// their own server, which is sent the reply. A notification made for
	// them here would be read by nobody.
	var recipients []int64
	if !root.IsRemote() {
		recipients = append(recipients, root.CreatedBy)
	}

	if reply.ParentID != reply.RootID {
		parent, err := h.Srv.GraphNodeGRPC.Read(reply.ParentID)
		if err != nil {
			return err
		}
		if !parent.IsRemote() {
			recipients = append(recipients, parent.CreatedBy)
		}
	}

	told := map[int64]bool{}
	for _, recipient := range recipients {
		// Not yourself, not nobody, and not the same person twice when the
		// post and the reply answered are by one author.
		if recipient <= 0 || recipient == subject.ID || told[recipient] {
			continue
		}
		told[recipient] = true

		n := entities.NewNotification()
		n.Type = NotificationTypeReplyAdd
		n.Service = payload.SourceService
		n.Subject.ID = subject.ID
		n.Subject.Type = subject.Type
		n.Object.ID = root.ID
		n.Object.Type = root.Type
		n.Target.ID = root.OwnerID
		n.Target.Type = root.OwnerType
		n.CommentID = reply.ID
		n.RecipientID = recipient

		if err := n.Validate(); err != nil {
			return err
		}

		if _, err := h.Repos.Notifications.Add(n); err != nil {
			return err
		}
	}

	return nil
}
