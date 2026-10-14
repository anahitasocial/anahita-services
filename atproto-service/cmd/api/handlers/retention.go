package handlers

import (
	"context"
	"log"
	"time"

	"anahita-libs/event"
)

// What is kept from Bluesky, held within bounds: the rules, and what they
// never touch, are in anahita-libs/federation/retention.go. They say which
// posts and which people are to go; here they go the way they would have
// gone had Bluesky said so, so that nothing is left pointing at them.

// Tidy removes posts and people that a rule says are to go. It answers
// with how many of each went.
func (h *Handlers) Tidy(posts, people []int64) (int, int) {
	var removedPosts, removedPeople int
	if h.Keeper == nil {
		return 0, 0
	}
	for _, id := range posts {
		was, err := h.Keeper.RemovePost(id)
		if err != nil {
			log.Printf("post %d, due to go, was not removed: %v", id, err)
			continue
		}
		if was == nil {
			continue
		}
		removedPosts++
		h.announce(event.RemoteNoteDeleted, was, was.OwnerID)
	}
	for _, id := range people {
		kept, err := h.Actors.ReadByNode(id)
		if err != nil || kept == nil {
			continue
		}
		// Followed since they were found: they stay.
		if followed, err := h.Actors.IsFollowedFromHere(id); err != nil || followed {
			continue
		}
		if err := h.forget(kept); err != nil {
			log.Printf("%s, of whom nothing is left here, was not removed: %v", kept.Handle, err)
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
				log.Printf("%d posts and %d people from Bluesky that nobody here holds to were removed", posts, people)
			}
			timer.Reset(every)
		}
	}
}
