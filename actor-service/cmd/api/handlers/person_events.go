package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/embeds"
	"anahita-libs/event"
	"anahita-libs/site"
	"fmt"
	"log"
	"strings"
)

func (h *personHandler) HandleEvents(payload *event.EventPayload) error {
	action := strings.Split(payload.Name, ".")[4]

	if action == "added" || action == "edited" {
		if err := h.updateTags(payload); err != nil {
			log.Println(err)
			return err
		}
	}

	return nil
}

func (h *personHandler) updateTags(payload *event.EventPayload) error {
	viewer := entities.NewViewer()
	viewer.ID = payload.ViewerID
	viewer.PersonType = payload.ViewerPersonType

	item, err := h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		log.Println(err)
		return err
	}

	// Who it names: by @handle, and by a profile's address on a line of
	// its own, embedded, which is a mention too, not a citation (the user,
	// 2026-10-17).
	named := append([]string{}, item.Mentions...)
	named = append(named, embeds.Find(item.Body, site.FromEnv().IsOurs).Profiles...)

	added, removed, err := h.Srv.Mentions.Update(item, named, viewer)
	if err != nil {
		log.Println(err)
		return err
	}

	// Each newly named, told of it by notification-service, which decides
	// whether there is anybody to tell. Only for a post: a person naming
	// somebody in what they say of themselves tells nobody.
	if !item.IsMedium() {
		return nil
	}
	for _, edge := range added {
		routingKey := fmt.Sprintf(event.EventFormatEdgeMentionAdded, "actor-service")
		payload := &event.EventPayload{
			Name:             routingKey,
			SourceService:    "actor-service",
			SubjectID:        item.GetAuthorID(),
			ObjectID:         item.ID,
			TargetID:         edge.NodeAID,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		if err := h.Publisher.Publish(payload, routingKey); err != nil {
			log.Println(err)
		}
	}

	// Each no longer named: the news of it goes (roadmap row 96, part 2).
	for _, id := range removed {
		routingKey := fmt.Sprintf(event.EventFormatEdgeMentionDeleted, "actor-service")
		payload := &event.EventPayload{
			Name:             routingKey,
			SourceService:    "actor-service",
			SubjectID:        item.GetAuthorID(),
			ObjectID:         item.ID,
			TargetID:         id,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		if err := h.Publisher.Publish(payload, routingKey); err != nil {
			log.Println(err)
		}
	}

	return nil
}
