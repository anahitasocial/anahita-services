package handlers

import (
	"context"
	"log"
	"sync"
	"time"

	"atproto/bsky"

	"anahita-libs/videos"
)

// A Bluesky post's video, which is not kept here when the post arrives:
// the post says where it is, the author's own server, and video-service
// copies it from there the first time somebody plays it (roadmap row 120,
// docs/plans/audio-and-video.md).

// pdsFor is how long an account's server is remembered: accounts move
// between servers seldom, and a post that names an old one fails to copy,
// and leads to its page.
const pdsFor = 6 * time.Hour

// PDSFinder finds the server that holds an account.
type PDSFinder func(ctx context.Context, did string) (string, error)

// CachedPDS remembers what a finder found, for a while, so that a feed of
// one author's posts asks once.
func CachedPDS(find PDSFinder) PDSFinder {
	type found struct {
		pds string
		at  time.Time
	}
	var (
		mu   sync.Mutex
		seen = map[string]found{}
	)
	return func(ctx context.Context, did string) (string, error) {
		mu.Lock()
		if f, ok := seen[did]; ok && time.Since(f.at) < pdsFor {
			mu.Unlock()
			return f.pds, nil
		}
		mu.Unlock()

		pds, err := find(ctx, did)
		if err != nil {
			return "", err
		}
		mu.Lock()
		if len(seen) > 10000 {
			seen = map[string]found{}
		}
		seen[did] = found{pds: pds, at: time.Now()}
		mu.Unlock()
		return pds, nil
	}
}

// videoElsewhere is where a post's video is, or nil: none, marked as not
// to be shown plainly (kept with none of its media until content warnings,
// row 122), or its author's server not found.
func (h *Handlers) videoElsewhere(post *bsky.Post, did, rkey string) *videos.Elsewhere {
	cid := post.VideoCID()
	if cid == "" || post.IsSensitive() || h.PDS == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pds, err := h.PDS(ctx, did)
	if err != nil {
		log.Printf("the server of %s, for a video: %v", did, err)
		return nil
	}
	return &videos.Elsewhere{
		Kind:   videos.ElsewhereVideo,
		Source: bsky.BlobURL(pds, did, cid),
		Page:   bsky.PostPage(did, rkey),
		Site:   "bsky.app",
	}
}
