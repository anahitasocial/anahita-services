package handlers

import (
	"log"

	an_entities "anahita-libs/data/entities"

	"notification/data/entities"
)

// What a reader can do from a notification (roadmap row 97, part 2), worked
// out as the list is read, from the graph as it is now, never from the row:
// a request answered since has nothing left to answer, and somebody
// followed since is not offered again.
const (
	// Accept or decline a request to follow that is still waiting.
	ActionAnswerRequest = "answer_request"
	// Follow whoever did it, when the reader does not.
	ActionFollow = "follow"
	// Answer it: a reply, a mention or a quote.
	ActionReply = "reply"
)

// replyable are the kinds a reader can answer from where they read them.
var replyable = map[string]bool{
	NotificationTypeReplyAdd:          true,
	NotificationTypeActorMention:      true,
	NotificationTypeActorMentionReply: true,
	NotificationTypeQuoteAdd:          true,
}

// setActions gives each notification what its reader can do from it. One
// question to the graph for all the follows; one for each request, of
// which there are few. When the graph cannot be asked, a notification has
// no actions, and is shown all the same.
func (h *notificationHandler) setActions(items []*entities.Notification, viewer *an_entities.Viewer) {
	var followers []int64
	for _, item := range items {
		if item.Type == NotificationTypeActorFollow && item.Subject.ID > 0 && item.Subject.ID != viewer.ID {
			followers = append(followers, item.Subject.ID)
		}
	}
	var relations map[int64]*an_entities.ActorRelation
	if len(followers) > 0 {
		var err error
		relations, err = h.Srv.SocialgraphGRPC.Browse(followers, viewer.ID)
		if err != nil {
			log.Println("notifications: asking whom the viewer follows:", err)
		}
	}

	for _, item := range items {
		switch {
		case replyable[item.Type]:
			item.Actions = []string{ActionReply}
		case item.Type == NotificationTypeActorFollow:
			// Followed by somebody the reader does not follow: + Follow. Not
			// on a line of several, which would not say whom.
			if len(item.Others) > 0 {
				continue
			}
			rel, known := relations[item.Subject.ID]
			if known && rel != nil && !rel.IsLeading {
				item.Actions = []string{ActionFollow}
			}
		case item.Type == NotificationTypeActorFollowRequest:
			// Still waiting: the asker has asked the profile (the viewer, or
			// a group or an event they administer) and has no answer yet.
			rel, err := h.Srv.SocialgraphGRPC.Read(item.Target.ID, item.Subject.ID)
			if err != nil {
				log.Println("notifications: asking whether a request is waiting:", err)
				continue
			}
			if rel != nil && rel.HasRequested {
				item.Actions = []string{ActionAnswerRequest}
			}
		}
	}
}
