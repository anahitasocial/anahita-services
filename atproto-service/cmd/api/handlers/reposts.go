package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
)

// What somebody on Bluesky reposted.
//
// A repost by somebody followed from here is kept as a repost by them,
// and is in the feed of whoever follows them as anybody's repost is. What
// was reposted is usually by somebody nobody here follows: it is read from
// Bluesky, with whoever wrote it, and kept for the repost's sake. That is
// the one way a post arrives without its author being followed, and it
// is the same for the fediverse.

// What a repost is said to be made with, as for one made here.
const repostService = "feed-service"

// reposted is somebody on Bluesky having reposted something.
func (h *Handlers) reposted(ctx context.Context, kept *repositories.RemoteActor, rkey string, record json.RawMessage) error {
	if h.Reposts == nil || h.RepostRecords == nil || !bsky.ValidRkey(rkey) || h.refused(kept.Handle) {
		return nil
	}
	followed, err := h.Actors.IsFollowedFromHere(kept.NodeID)
	if err != nil || !followed {
		return err
	}

	repost := &bsky.Repost{}
	if json.Unmarshal(record, repost) != nil {
		return nil
	}
	uri := repost.Subject.URI
	if _, _, ok := bsky.ParsePostURI(uri); !ok {
		return nil
	}

	post, err := h.Nodes.ReadByURI(uri)
	if err != nil {
		return err
	}
	if post == nil || post.ID == 0 {
		if post, err = h.fetchPost(ctx, uri); err != nil || post == nil {
			return err
		}
	}

	has, err := h.Reposts.IsReposted(kept.NodeID, post.ID)
	if err != nil {
		return err
	}
	if !has {
		viewer := entities.NewViewer()
		viewer.ID = kept.NodeID
		_, err = h.Reposts.Add(&services.RepostAddRequest{
			OwnerID: kept.NodeID, ParentID: post.ID, Service: repostService, Access: post.Access, Viewer: viewer,
		})
		if err != nil {
			return err
		}
	}
	return h.RepostRecords.Save(kept.NodeID, rkey, post.ID)
}

// unreposted is somebody on Bluesky having taken a repost back.
func (h *Handlers) unreposted(kept *repositories.RemoteActor, rkey string) error {
	if h.Reposts == nil || h.RepostRecords == nil || !bsky.ValidRkey(rkey) {
		return nil
	}
	parentID, err := h.RepostRecords.Take(kept.NodeID, rkey)
	if err != nil || parentID == 0 {
		return err
	}
	has, err := h.Reposts.IsReposted(kept.NodeID, parentID)
	if err != nil || !has {
		return err
	}
	return h.Reposts.Delete(&services.RepostDeleteRequest{ParentID: parentID, AuthorID: kept.NodeID})
}

// fetchPost reads a post from Bluesky and keeps it, with whoever wrote it:
// something reposted by somebody followed from here, that nobody here had
// seen. Only a post that begins a thread. Nil when there is nothing to
// keep.
func (h *Handlers) fetchPost(ctx context.Context, uri string) (*entities.AnahitaNode, error) {
	did, rkey, ok := bsky.ParsePostURI(uri)
	if !ok {
		return nil, nil
	}

	post, err := h.Bluesky.PostAt(ctx, uri)
	if err != nil || post == nil {
		// Gone, or Bluesky is away: nothing to show.
		log.Printf("a reposted post at %s could not be read: %v", uri, err)
		return nil, nil
	}
	if post.IsReply() {
		return nil, nil
	}

	// Whoever wrote it, as they are now.
	profile, err := h.Bluesky.Profile(ctx, did)
	if err != nil || profile == nil || profile.DID != did {
		log.Printf("whoever wrote %s could not be read: %v", uri, err)
		return nil, nil
	}
	if h.refused(profile.Handle) {
		return nil, nil
	}
	if _, err := h.keep(profile); err != nil {
		// Written by somebody here, on Bluesky: not kept as from elsewhere.
		if errors.Is(err, errConnectedHere) {
			return nil, nil
		}
		return nil, err
	}
	author, err := h.Actors.ReadByDID(did)
	if err != nil || author == nil {
		return nil, err
	}
	return h.storePost(author, rkey, post, 0)
}
