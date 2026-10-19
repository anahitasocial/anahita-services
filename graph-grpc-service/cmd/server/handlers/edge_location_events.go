package handlers

import (
	"fmt"
	"log"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"
)

// announceLocationTag says on the bus that a post was given a location, or
// had one taken off, one at a time from its page, so that a post that went
// to other servers goes again with its locations as they are now
// (activitypub-service, as an Update; roadmap row 82, part 3b). A post's
// locations chosen as it was made are tied in its Add and announced with it.
//
// The object is the post, the target the location.
func (h *edgeHandler) announceLocationTag(edge *entities.AnahitaEdge, format string) {
	if edge == nil || edge.Type != constants.EdgeTagLocationType || h.env.Publisher == nil {
		return
	}
	go func() {
		name := fmt.Sprintf(format, "location")
		payload := &event.EventPayload{
			Name:          name,
			SourceService: "graph-grpc-service",
			SubjectID:     edge.CreatedBy,
			SubjectType:   constants.NodeActorPersonEntityType,
			ObjectID:      edge.NodeBID,
			ObjectType:    edge.NodeBType,
			TargetID:      edge.NodeAID,
			TargetType:    edge.NodeAType,
			ViewerID:      edge.CreatedBy,
		}
		if err := h.env.Publisher.Publish(payload, name); err != nil {
			log.Printf("announcing a location tag on %d: %v", edge.NodeBID, err)
		}
	}()
}
