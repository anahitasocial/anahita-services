package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"notification/data/entities"
)

// NotificationTypeReplyAdd is somebody answering a post or a reply of yours.
const NotificationTypeReplyAdd = "reply_add"

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

	recipients := []int64{root.CreatedBy}

	if reply.ParentID != reply.RootID {
		parent, err := h.Srv.GraphNodeGRPC.Read(reply.ParentID)
		if err != nil {
			return err
		}
		recipients = append(recipients, parent.CreatedBy)
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
