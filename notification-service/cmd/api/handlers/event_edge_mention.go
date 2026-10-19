package handlers

import (
	"anahita-libs/cmd/api/permissions"
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"notification/data/entities"
)

const (
	// NotificationTypeActorMention is a person named in a post.
	NotificationTypeActorMention = "actor_mention"
	// NotificationTypeActorMentionReply is a person named in a reply.
	NotificationTypeActorMentionReply = "actor_mention_reply"
)

// eventFormatEdgeMentionAdded tells a person here that a post named them
// (the user, 2026-10-17). actor-service has already kept the mention only
// where the person lets this author name them (permissions.CanMention).
// Told only a person here, never the author themselves, never of a draft,
// and only when they may see the post: being named in a private group's
// post tells nobody outside it that the post is there. A group or an event
// named is told nothing.
func (h *eventHandler) eventFormatEdgeMentionAdded(payload *event.EventPayload) error {
	viewer := an_entities.NewViewer()

	target, err := h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return err
	}
	if !target.IsPerson() || target.IsRemote() || target.ID == payload.SubjectID {
		return nil
	}

	object, err := h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		return err
	}
	if object.Draft {
		return nil
	}

	reader := an_entities.NewViewer()
	reader.ID = target.ID
	reader.PersonType = target.PersonType
	reader.Authenticated = true
	if !permissions.CanReadByID(h.Srv.GraphNodeGRPC, object.ID, reader) {
		return nil
	}

	subject, err := h.Srv.ActorsGRPC.Read(payload.SubjectID, viewer)
	if err != nil {
		return err
	}

	n := entities.NewNotification()
	n.Type = NotificationTypeActorMention
	if object.RootID > 0 {
		n.Type = NotificationTypeActorMentionReply
	}
	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Subject.Type = subject.Type
	n.Object.ID = object.ID
	n.Object.Type = object.Type
	n.Target.ID = target.ID
	n.Target.Type = target.Type
	n.RecipientID = target.ID
	if err := n.Validate(); err != nil {
		return err
	}
	_, err = h.Repos.Notifications.Add(n)
	return err
}

// eventFormatEdgeMentionDeleted takes back the news of being named, when an
// edit no longer names them (roadmap row 96, part 2). Named again later,
// they are told again.
func (h *eventHandler) eventFormatEdgeMentionDeleted(payload *event.EventPayload) error {
	for _, kind := range []string{NotificationTypeActorMention, NotificationTypeActorMentionReply} {
		n := entities.NewNotification()
		n.Type = kind
		n.Service = payload.SourceService
		n.RecipientID = payload.TargetID
		n.Subject.ID = payload.SubjectID
		n.Object.ID = payload.ObjectID
		n.Target.ID = payload.TargetID
		if err := h.Repos.Notifications.Delete(n); err != nil {
			return err
		}
	}
	return nil
}
