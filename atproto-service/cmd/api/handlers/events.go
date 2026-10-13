package handlers

import (
	"context"
	"log"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/event"
)

// What happens here that this service has to know of.
//
// Somebody here follows somebody on Bluesky the way they follow anybody:
// the same button, the same request to the same service. This hears that
// it happened, on the bus the services already talk over, reads what the
// account posted lately, and has the listener told to hear of it from now
// on.
//
// The bus can lose a message. What is lost is found a minute later by
// CatchUp, which asks the graph who is followed and does not rely on
// having been told.

// EventTopics is what this service listens for.
var EventTopics = []string{
	"edge.follow.*.*.v1",
	"edge.voteup.*.*.v1",
	"node.repost.*.*.*.v1",
	"node.medium.*.*.*.v1",
}

// HandleEvent is told of something that happened here.
func (h *Handlers) HandleEvent(payload *event.EventPayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	added := strings.HasSuffix(payload.Name, ".added.v1")
	deleted := strings.HasSuffix(payload.Name, ".deleted.v1")

	var err error
	switch {
	case strings.HasPrefix(payload.Name, "edge.follow."):
		err = h.followChanged(payload.TargetID, added)

	// Somebody liked something, or took a like back: who is the subject,
	// what the object.
	case strings.HasPrefix(payload.Name, "edge.voteup.") && added:
		err = h.localLiked(ctx, payload.SubjectID, payload.ObjectID)
	case strings.HasPrefix(payload.Name, "edge.voteup.") && deleted:
		err = h.localUnliked(ctx, payload.SubjectID, payload.ObjectID)

	case strings.HasPrefix(payload.Name, "node.repost.") && added:
		err = h.localReposted(ctx, payload.ObjectID)
	case strings.HasPrefix(payload.Name, "node.repost.") && deleted:
		err = h.localRemoved(ctx, payload.ObjectID)

	case strings.HasPrefix(payload.Name, "node.medium.") &&
		(payload.ObjectType == constants.NodeMediumNoteEntityType || payload.ObjectType == constants.NodeMediumPhotoEntityType):
		if deleted {
			err = h.localRemoved(ctx, payload.ObjectID)
		} else {
			err = h.localPosted(ctx, payload.ObjectID)
		}
	}
	if err != nil {
		log.Printf("event %s (%d): %v", payload.Name, payload.ObjectID, err)
	}
	return err
}

// followChanged is somebody here having followed somebody, or stopped.
func (h *Handlers) followChanged(targetID int64, followed bool) error {
	target, err := h.Nodes.Read(targetID)
	if err != nil || target == nil || target.Type != constants.NodeActorATProtoPersonEntityType {
		return err
	}
	kept, err := h.Actors.ReadByNode(target.ID)
	if err != nil || kept == nil {
		return err
	}

	if followed {
		h.later(func() { h.backfill(kept) })
	}
	// Followed or no longer followed: whose posts are wanted may have
	// changed either way.
	h.changed()
	return nil
}
