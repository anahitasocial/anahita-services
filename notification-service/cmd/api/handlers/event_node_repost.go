package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"notification/data/entities"
)

// eventFormatNodeRepostAdded
func (h *eventHandler) eventFormatNodeRepostAdded(payload *event.EventPayload) error {
	viewer := an_entities.NewViewer()

	subject, err := h.Srv.ActorsGRPC.Read(payload.SubjectID, viewer)
	if err != nil {
		return err
	}

	object, err := h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		return err
	}

	objectParent, err := h.Srv.GraphNodeGRPC.Read(object.Parent.ID)
	if err != nil {
		return err
	}

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}

	// Whoever reposted takes part in the post from now on: they are told
	// of what the people they follow do to it (roadmap row 146).
	defer h.subscribeTo(subject, objectParent)

	// Its author, or a channel's admins, whoever reposted it; and its
	// subscribers, when they follow whoever reposted it. Whoever wrote
	// something from another server is told by their own server, which is
	// sent the repost.
	following, err := h.subscribersFollowing(objectParent, subject.ID)
	if err != nil {
		return err
	}
	for _, recipient := range uniqueOthers(subject.ID, h.peopleBehind(objectParent.Author.ID), following) {
		if !h.mayRead(recipient, objectParent.ID) {
			continue
		}
		n := entities.NewNotification()
		n.Type = "repost_add"
		n.Service = payload.SourceService
		n.Subject.ID = subject.ID
		n.Subject.Type = subject.Type
		n.Object.ID = objectParent.ID
		n.Object.Type = objectParent.Type
		n.Target.ID = target.ID
		n.Target.Type = target.Type
		// The repost itself, which holds what was done, as a reply's
		// notification keeps the reply: taken back, it is gone before it
		// is told, and this is how its news is found.
		n.CommentID = object.ID
		n.RecipientID = recipient
		if err := n.Validate(); err != nil {
			return err
		}
		if _, err := h.add(n); err != nil {
			return err
		}
	}

	return nil
}

// eventFormatNodeRepostDeleted removes the news of a repost taken back. The
// repost is gone by now, and its notification is found by its id.
func (h *eventHandler) eventFormatNodeRepostDeleted(payload *event.EventPayload) error {
	return h.Repos.Notifications.DeleteAbout(payload.ObjectID)
}
