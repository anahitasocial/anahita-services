package handlers

import (
	"anahita-libs/posts"
	"context"
	"log"
	"time"

	"atproto/data/repositories"
)

// The posts the bus lost. A post is written to Bluesky when its making is
// heard on the bus. When that word never comes (banu.io, 2026-11-22: the
// graph had lost the bus, and a channel's post stayed off Bluesky), the post
// is found here and handed to what would have heard it, which writes each
// once however often it is asked: what is written is remembered first
// (atproto_sent). The fediverse has the same sweep.

const (
	// How long the bus is given before a post is taken for lost.
	missedGrace = 2 * time.Minute
	// How far back a post is looked for. One that went unwritten for
	// longer is not written late: it would be shown as new.
	missedWindow = 48 * time.Hour
	// The most handed on in one look. The rest wait for the next.
	missedBatch = 200
)

// The kinds of post written to Bluesky (localPosted).
var missedKinds = posts.FederatingTypes()

// writeMissed writes the posts made a while ago whose making was not heard,
// and says how many of them it wrote. One found and not written is one that
// does not go: a quote of a post from the fediverse, say, or a video post
// still without its video.
func (h *Handlers) writeMissed(ctx context.Context, now time.Time) int {
	if h.Missed == nil || !h.canWrite() {
		return 0
	}
	found, err := h.Missed.PostsNotWritten(missedKinds, now.Add(-missedWindow), now.Add(-missedGrace), missedBatch)
	if err != nil {
		log.Printf("posts not written to Bluesky could not be looked for: %v", err)
		return 0
	}
	written := 0
	for _, id := range found {
		if err := h.localPosted(ctx, id); err != nil {
			log.Printf("a post not written to Bluesky (%d): %v", id, err)
			continue
		}
		if sent, err := h.Sent.Read(repositories.SentPost, id, 0); err == nil && sent != nil {
			written++
		}
	}
	if written > 0 {
		log.Printf("wrote %d posts to Bluesky whose making was not heard", written)
	}
	return written
}
