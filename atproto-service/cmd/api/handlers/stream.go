package handlers

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"atproto/bsky"
)

// What is heard from Bluesky as it happens.
//
// Only the accounts followed from here are asked for, and what is heard
// is still held to that: whoever it is about must be somebody kept here,
// and a post of theirs is kept only while somebody here follows them.

// Wanted is whose posts are asked for: everybody on Bluesky that somebody
// here follows.
func (h *Handlers) Wanted() ([]string, error) {
	return h.Actors.Followed(bsky.MaxWanted)
}

// Heard deals with one thing heard.
func (h *Handlers) Heard(ctx context.Context, heard *bsky.Event) {
	if heard == nil || !bsky.ValidDID(heard.DID) {
		return
	}
	kept, err := h.Actors.ReadByDID(heard.DID)
	if err != nil || kept == nil {
		return
	}

	switch heard.Kind {
	case "commit":
		if heard.Commit == nil {
			return
		}
		switch heard.Commit.Collection {
		case bsky.PostCollection:
			if heard.Commit.Operation == "delete" {
				err = h.removePost(kept, heard.Commit.RKey)
				break
			}
			post := &bsky.Post{}
			if json.Unmarshal(heard.Commit.Record, post) != nil {
				return
			}
			err = h.keepPost(kept, heard.Commit.RKey, post)

		case bsky.RepostCollection:
			if heard.Commit.Operation == "delete" {
				err = h.unreposted(kept, heard.Commit.RKey)
				break
			}
			err = h.reposted(ctx, kept, heard.Commit.RKey, heard.Commit.Record)

		case bsky.ProfileCollection:
			// What they say of themselves changed.
			h.refreshSoon(heard.DID)
		}

	case "identity":
		// The handle they go by changed.
		h.refreshSoon(heard.DID)

	case "account":
		// Their account was closed. One that is only put away, or held
		// back by the network, may come back, and is left as it is.
		if heard.Account != nil && !heard.Account.Active && heard.Account.Status == "deleted" {
			err = h.forget(kept)
		}
	}
	if err != nil {
		log.Printf("heard %s of %s: %v", heard.Kind, kept.Handle, err)
	}
}

// refreshSoon reads somebody again in a moment: the network's reader
// hears of the change as this does, and is asked once it has.
func (h *Handlers) refreshSoon(did string) {
	h.later(func() {
		if h.Settle > 0 {
			time.Sleep(h.Settle)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*bsky.Timeout)
		defer cancel()
		if err := h.refresh(ctx, did); err != nil {
			log.Printf("%s changed and was not read again: %v", did, err)
		}
	})
}
