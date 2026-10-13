package handlers

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"activitypub/ap"
	"activitypub/data/repositories"
	"activitypub/remote"
)

// What somebody elsewhere posted before they were followed.
//
// A server sends a follower what is posted from then on. Somebody followed
// today would have an empty profile here until they next post, so their
// most recent public notes are read, once, from the list their server
// keeps of what they have posted. Each is held to the same rule as a post
// that is sent: theirs, public, not a reply.

const (
	// How many of their posts are read, and how many times their server is
	// asked in all, whatever it answers.
	backfillNotes   = 20
	backfillFetches = 25
)

// outbox is a list of what somebody posted, or a page of one.
type outbox struct {
	First        json.RawMessage   `json:"first"`
	OrderedItems []json.RawMessage `json:"orderedItems"`
	Items        []json.RawMessage `json:"items"`
}

// backfill reads the recent posts of somebody who has just been followed.
// Whatever goes wrong, nothing is lost: they are followed all the same, and
// what they post next arrives.
func (h *Handlers) backfill(kept *repositories.RemoteActor) {
	if h.Fetcher == nil || kept == nil {
		return
	}
	if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(kept.URI)) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	fetches := 0
	fetch := func(address string, into any) bool {
		// Only ever their own server, however the list is laid out.
		if fetches >= backfillFetches || !sameHost(address, kept.URI) {
			return false
		}
		fetches++
		_, err := h.Fetcher.Object(ctx, address, into)
		return err == nil
	}

	actor, err := remote.Fetch(ctx, h.Fetcher, kept.URI, h.Site.IsOurs)
	fetches++
	if err != nil || actor.Outbox == "" {
		return
	}

	var list outbox
	if !fetch(actor.Outbox, &list) {
		return
	}
	items := append(list.OrderedItems, list.Items...)

	// A list is usually given as a count and where its first page is.
	if len(items) == 0 && len(list.First) > 0 {
		var page outbox
		if list.First[0] == '{' {
			_ = json.Unmarshal(list.First, &page)
		} else {
			var address string
			if json.Unmarshal(list.First, &address) != nil || !fetch(address, &page) {
				return
			}
		}
		items = append(page.OrderedItems, page.Items...)
	}

	kept2 := 0
	for _, item := range items {
		if kept2 >= backfillNotes {
			break
		}

		// Each is something done (a Create with the post inside it, or with
		// only the post's address), or only an address.
		var activity ap.Activity
		if len(item) > 0 && item[0] == '{' {
			if json.Unmarshal(item, &activity) != nil || activity.Type != "Create" || activity.ActorID() != kept.URI {
				continue
			}
		} else {
			continue
		}

		note := activity.Note()
		if note == nil {
			var fetched ap.ReceivedNote
			if address := activity.ObjectID(); address == "" || !fetch(address, &fetched) {
				continue
			}
			note = &fetched
		}

		before := note.ID
		if err := h.keepNote(kept, note); err != nil {
			log.Printf("backfill of %s: %s: %v", kept.URI, before, err)
			continue
		}
		kept2++
	}
}
