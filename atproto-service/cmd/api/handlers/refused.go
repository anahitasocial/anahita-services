package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"anahita-libs/event"
	"anahita-libs/federation"
)

// Accounts on Bluesky refused one by one (docs/plans/done/refused-accounts.md).
// They are refused by administrators through activitypub-service, which
// keeps the list for both networks; this service removes the refused ones
// it keeps, and hears nothing more of them.

// ForgetRefused removes every refused account still kept here, with what
// is here of it, and answers with how many went.
func (h *Handlers) ForgetRefused() int {
	if h.RefusedAccounts == nil {
		return 0
	}
	forgotten := 0
	for _, did := range h.RefusedAccounts.Refused(federation.NetworkATProto) {
		kept, err := h.Actors.ReadByDID(did)
		if err != nil || kept == nil {
			continue
		}
		if err := h.forget(kept); err != nil {
			log.Printf("refused account %s was not removed: %v", did, err)
			continue
		}
		forgotten++
	}
	return forgotten
}

// RunRefusedSweep looks for refused accounts every so often: the list is
// read again each half minute, so one refused is gone within a minute.
func RunRefusedSweep(ctx context.Context, every time.Duration, round func() int) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if gone := round(); gone > 0 {
				log.Printf("%d refused accounts from Bluesky were removed", gone)
			}
		}
	}
}

// removePhotoPosts removes somebody's photo posts with what is stored of
// their pictures, before they themselves go: the graph's removal of them
// takes the posts, and would leave the pictures.
func (h *Handlers) removePhotoPosts(ownerID int64) {
	if h.PhotoPostsOf == nil || h.Keeper == nil {
		return
	}
	ids, err := h.PhotoPostsOf(ownerID)
	if err != nil {
		log.Printf("the photo posts of %d were not read: %v", ownerID, err)
		return
	}
	for _, id := range ids {
		if _, err := h.Keeper.RemovePost(id); err != nil {
			log.Printf("photo post %d of %d was not removed: %v", id, ownerID, err)
		}
	}
}

// announceGone tells the other services that somebody from Bluesky is gone,
// as a purge tells of somebody here.
func (h *Handlers) announceGone(id int64) {
	if h.Announce == nil || id <= 0 {
		return
	}
	name := fmt.Sprintf(event.EventFormatNodeActorPurged, "atproto", "person")
	payload := &event.EventPayload{Name: name, ObjectID: id, TargetID: id, SubjectID: id, SourceService: "atproto-service"}
	if err := h.Announce(payload, name); err != nil {
		log.Printf("saying that %d is gone: %v", id, err)
	}
}
