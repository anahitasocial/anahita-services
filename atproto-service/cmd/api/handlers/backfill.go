package handlers

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"atproto/bsky"
	"atproto/data/repositories"
)

// What somebody on Bluesky posted before they were followed.
//
// What is heard is what is posted from now on. Somebody followed today
// would have an empty profile here until they next post, so their most
// recent posts are read, once, from the network's reader. Each is held to
// the same rule as a post that is heard: theirs, and not a reply.

// How many of their posts are read.
const backfillPosts = 20

// backfill reads the recent posts of somebody who has just been followed.
// It is done once for each of them, by whichever copy of this service
// claims it; a reading that fails gives the claim back, to be tried again.
func (h *Handlers) backfill(kept *repositories.RemoteActor) {
	if kept == nil || h.refused(kept.Handle) {
		return
	}
	claimed, err := h.Actors.BeginBackfill(kept.NodeID)
	if err != nil || !claimed {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// Asked for with room to spare: what they reposted is in the list too,
	// and is not theirs.
	feed, err := h.Bluesky.AuthorFeed(ctx, kept.DID, 2*backfillPosts)
	if err != nil {
		log.Printf("the earlier posts of %s were not read: %v", kept.Handle, err)
		if err := h.Actors.UndoBackfill(kept.NodeID); err != nil {
			log.Printf("the earlier posts of %s will not be tried again: %v", kept.Handle, err)
		}
		return
	}

	// Oldest first, so that they are kept in the order they were written.
	read := 0
	var theirs []bsky.FeedItem
	for _, item := range feed {
		if read == backfillPosts {
			break
		}
		// Something reposted, or pinned out of its place: not a post of
		// theirs at this place in the list.
		if len(item.Reason) > 0 && string(item.Reason) != "null" {
			continue
		}
		if item.Post.Author.DID != kept.DID {
			continue
		}
		theirs = append(theirs, item)
		read++
	}

	for i := len(theirs) - 1; i >= 0; i-- {
		item := theirs[i]
		did, rkey, ok := bsky.ParsePostURI(item.Post.URI)
		if !ok || did != kept.DID {
			continue
		}
		post := &bsky.Post{}
		if err := json.Unmarshal(item.Post.Record, post); err != nil {
			continue
		}
		if err := h.keepPost(kept, rkey, post); err != nil {
			log.Printf("the earlier posts of %s: %s: %v", kept.Handle, item.Post.URI, err)
		}
	}
}

// CatchUp reads the earlier posts of whoever is followed and has not had
// them read: somebody whose following was said on the bus and not heard.
// And it has the listener told whose posts are wanted, for the same
// reason.
func (h *Handlers) CatchUp() {
	waiting, err := h.Actors.NotBackfilled(20)
	if err != nil {
		log.Println("who is followed on Bluesky could not be read:", err)
		return
	}
	for _, kept := range waiting {
		h.backfill(kept)
	}
	h.changed()
}

// RunCatchUp does that now and then, until `ctx` is done.
func (h *Handlers) RunCatchUp(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.CatchUp()
		}
	}
}
