package handlers

import (
	"context"
	"log"
	"time"

	"anahita-libs/constants"
	"anahita-libs/event"
)

// What is kept from other servers, held within bounds: the rules, and
// what they never touch, are in anahita-libs/federation/retention.go. They
// say which posts and which people are to go; here they go the way they
// would have gone had their own server said so, so that nothing is left
// pointing at them.

// Tidy removes posts and people that a rule says are to go. It answers
// with how many of each went.
func (h *Handlers) Tidy(posts, people []int64) (int, int) {
	var removedPosts, removedPeople int
	keeper := h.keeper()
	for _, id := range posts {
		was, err := keeper.RemovePost(id)
		if err != nil {
			log.Printf("post %d, due to go, was not removed: %v", id, err)
			continue
		}
		if was == nil {
			continue
		}
		removedPosts++
		if h.Announce == nil {
			continue
		}
		// Whoever was told of it is not left with news that leads nowhere.
		payload := &event.EventPayload{
			Name:          event.RemoteNoteDeleted,
			SourceService: "activitypub-service",
			OwnerID:       was.OwnerID,
			OwnerType:     constants.NodeActorActivityPubPersonEntityType,
			SubjectID:     was.OwnerID,
			SubjectType:   constants.NodeActorActivityPubPersonEntityType,
			ObjectID:      was.ID,
			ObjectType:    was.Type,
			ViewerID:      was.OwnerID,
		}
		if err := h.Announce(payload, event.RemoteNoteDeleted); err != nil {
			log.Printf("saying that %s is gone: %v", was.URI, err)
		}
	}
	for _, id := range people {
		kept, err := h.Actors.ReadByNode(id)
		if err != nil || kept == nil {
			continue
		}
		if err := h.forget(kept); err != nil {
			log.Printf("%s, of whom nothing is left here, was not removed: %v", kept.URI, err)
			continue
		}
		removedPeople++
	}
	return removedPosts, removedPeople
}

// RunTidy does a round now and then until `ctx` is done. `round` does one
// and says how many posts and people went.
func RunTidy(ctx context.Context, every time.Duration, round func() (int, int)) {
	timer := time.NewTimer(3 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if posts, people := round(); posts > 0 || people > 0 {
				log.Printf("%d posts and %d people from other servers that nobody here holds to were removed", posts, people)
			}
			timer.Reset(every)
		}
	}
}
