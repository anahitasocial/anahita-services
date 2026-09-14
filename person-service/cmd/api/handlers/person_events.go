package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/event"
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

	if err := h.Srv.Mentions.Update(item, item.Mentions, viewer); err != nil {
		log.Println(err)
		return err
	}

	return nil
}
