package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/event"
	"anahita-libs/permissions"
	"notification/data/entities"
)

// People in a picture or a video (roadmap row 103). actor-service sends the
// events: the subject is the post's author, the object the post, the target
// the person tagged, the viewer whoever did it.
const (
	// NotificationTypeActorTagRequest is a person asked to be in a post,
	// answered from the notification while it waits.
	NotificationTypeActorTagRequest = "actor_tag_request"
	// NotificationTypeActorTag is a person tagged at once, as they chose.
	NotificationTypeActorTag = "actor_tag"
	// NotificationTypeActorTagAccept tells the post's author that a person
	// said yes. Its subject is that person.
	NotificationTypeActorTagAccept = "actor_tag_accept"
)

// tagNews is what a tag event is about, read once: the post, and whether
// the person tagged may be told. A person here, not the author themselves,
// never of a draft, and only of a post they may see.
func (h *eventHandler) tagNews(payload *event.EventPayload) (person, post *an_entities.AnahitaNode, ok bool, err error) {
	viewer := an_entities.NewViewer()
	person, err = h.Srv.ActorsGRPC.Read(payload.TargetID, viewer)
	if err != nil {
		return nil, nil, false, err
	}
	if !person.IsPerson() || person.IsRemote() || person.ID == payload.SubjectID {
		return nil, nil, false, nil
	}
	post, err = h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		return nil, nil, false, err
	}
	if post.Draft {
		return nil, nil, false, nil
	}
	reader := an_entities.NewViewer()
	reader.ID = person.ID
	reader.PersonType = person.PersonType
	reader.Authenticated = true
	if !permissions.CanReadByID(h.Srv.GraphNodeGRPC, post.ID, reader) {
		return nil, nil, false, nil
	}
	return person, post, true, nil
}

func (h *eventHandler) addTagNotification(kind string, payload *event.EventPayload, subjectID, recipientID int64, post *an_entities.AnahitaNode, person *an_entities.AnahitaNode) error {
	subject, err := h.Srv.ActorsGRPC.Read(subjectID, an_entities.NewViewer())
	if err != nil {
		return err
	}
	n := entities.NewNotification()
	n.Type = kind
	n.Service = payload.SourceService
	n.Subject.ID = subject.ID
	n.Subject.Type = subject.Type
	n.Object.ID = post.ID
	n.Object.Type = post.Type
	n.Target.ID = person.ID
	n.Target.Type = person.Type
	n.RecipientID = recipientID
	if err := n.Validate(); err != nil {
		return err
	}
	_, err = h.Repos.Notifications.Add(n)
	return err
}

// eventFormatEdgeTagRequested asks a person to be in a post: Accept or
// Decline, from the notification, while the tag waits.
func (h *eventHandler) eventFormatEdgeTagRequested(payload *event.EventPayload) error {
	person, post, ok, err := h.tagNews(payload)
	if err != nil || !ok {
		return err
	}
	return h.addTagNotification(NotificationTypeActorTagRequest, payload, payload.SubjectID, person.ID, post, person)
}

// eventFormatEdgeTagAdded is a tag made at once, which the person is told
// of, or a tag they approved, which the post's author is told of.
func (h *eventHandler) eventFormatEdgeTagAdded(payload *event.EventPayload) error {
	person, post, ok, err := h.tagNews(payload)
	if err != nil || !ok {
		return err
	}
	if payload.ViewerID == person.ID {
		// They said yes. What asked them has nothing left to answer, and
		// stays as it is; the author is told.
		author, err := h.Srv.ActorsGRPC.Read(payload.SubjectID, an_entities.NewViewer())
		if err != nil {
			return err
		}
		if !author.IsPerson() || author.IsRemote() {
			return nil
		}
		return h.addTagNotification(NotificationTypeActorTagAccept, payload, person.ID, author.ID, post, person)
	}
	return h.addTagNotification(NotificationTypeActorTag, payload, payload.SubjectID, person.ID, post, person)
}

// eventFormatEdgeTagDeleted takes back every word of a tag taken off, by
// either of them: asked, tagged, accepted.
func (h *eventHandler) eventFormatEdgeTagDeleted(payload *event.EventPayload) error {
	for _, item := range []struct {
		kind      string
		subjectID int64
		recipient int64
	}{
		{NotificationTypeActorTagRequest, payload.SubjectID, payload.TargetID},
		{NotificationTypeActorTag, payload.SubjectID, payload.TargetID},
		{NotificationTypeActorTagAccept, payload.TargetID, payload.SubjectID},
	} {
		n := entities.NewNotification()
		n.Type = item.kind
		n.Service = payload.SourceService
		n.RecipientID = item.recipient
		n.Subject.ID = item.subjectID
		n.Object.ID = payload.ObjectID
		n.Target.ID = payload.TargetID
		if err := h.Repos.Notifications.Delete(n); err != nil {
			return err
		}
	}
	return nil
}
