package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
)

// What people on Bluesky took back.
//
// An account is told when somebody likes or reposts what it posted,
// answers it, or follows it. It is not told when they think better of it.
// So what was kept here of such things is looked for on Bluesky again,
// now and then: who likes and has reposted each post that went out from
// here, whether each answer to one is still there, and whether each
// follower of somebody here still follows. What is no longer there is
// removed here, as it would be had it been undone here.
//
// Nothing is removed on a doubt. A list that could not be read, or was
// too long to read to its end, removes nobody; nor does an answer that
// says nothing either way. And what was kept in the last few minutes is
// left alone, since Bluesky's reader takes a moment to know of it.

// How many of each kind are looked at in one go.
const verifyMost = 500

// Verified is what people on Bluesky did that is kept here.
type Verified interface {
	LikedBy(ctx context.Context, uri string) (map[string]bool, bool, error)
	RepostedBy(ctx context.Context, uri string) (map[string]bool, bool, error)
	IsFollowing(ctx context.Context, did, other string) (bool, error)
}

// VerifyReactions looks on Bluesky for what is kept here, and removes
// what has been taken back.
func (h *Handlers) VerifyReactions() {
	if !h.canWrite() || h.Kept == nil || h.Verify == nil || h.Likes == nil || h.Reposts == nil || h.FollowEdges == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	if likes, err := h.Kept.Likes(verifyMost); err != nil {
		log.Println("the likes kept from Bluesky could not be read:", err)
	} else {
		h.verifyEach(ctx, likes, h.Verify.LikedBy, h.unlikedOnBluesky)
	}
	if reposts, err := h.Kept.Reposts(verifyMost); err != nil {
		log.Println("the reposts kept from Bluesky could not be read:", err)
	} else {
		h.verifyEach(ctx, reposts, h.Verify.RepostedBy, h.unrepostedOnBluesky)
	}
	h.verifyReplies(ctx)
	h.verifyFollows(ctx)
}

// RunVerify does that now and then, until `ctx` is done.
func (h *Handlers) RunVerify(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.VerifyReactions()
		}
	}
}

// verifyEach asks, once for each post, who has done something to it now,
// and has what is kept of anybody who is not among them removed.
func (h *Handlers) verifyEach(
	ctx context.Context,
	kept []*repositories.KeptReaction,
	who func(context.Context, string) (map[string]bool, bool, error),
	remove func(*repositories.KeptReaction) error,
) {
	type answer struct {
		by       map[string]bool
		complete bool
	}
	asked := map[string]*answer{}

	for _, each := range kept {
		if ctx.Err() != nil {
			return
		}
		uri := bsky.PostURI(each.PostDID, each.PostRKey)
		now, known := asked[uri]
		if !known {
			by, complete, err := who(ctx, uri)
			if err != nil {
				// Not read: nobody is found missing from it. A post
				// that is gone from Bluesky goes from here by its own
				// way.
				by, complete = nil, false
			}
			now = &answer{by: by, complete: complete}
			asked[uri] = now
		}
		if !now.complete || now.by[each.ActorDID] {
			continue
		}
		if err := remove(each); err != nil {
			log.Printf("what %s took back on Bluesky was not removed here: %v", each.ActorDID, err)
		}
	}
}

func (h *Handlers) unlikedOnBluesky(kept *repositories.KeptReaction) error {
	liker, err := h.Nodes.Read(kept.ActorID)
	if err != nil || liker == nil || liker.ID == 0 {
		return err
	}
	post, err := h.Nodes.Read(kept.PostID)
	if err != nil || post == nil || post.ID == 0 {
		return err
	}
	has, err := h.Likes.IsLiked(liker.ID, post.ID)
	if err != nil || !has {
		return err
	}
	if err := h.Likes.Delete(liker, post); err != nil {
		return err
	}

	// Said on the bus as like-service says it, so that the news of the
	// like goes with it.
	if h.Announce == nil {
		return nil
	}
	kind := strings.Split(post.Service, "-")[0]
	if kind == "" {
		kind = "text"
	}
	name := fmt.Sprintf(event.EventFormatEdgeVoteupDeleted, kind)
	payload := &event.EventPayload{
		Name: name, SourceService: "like-service",
		OwnerID: post.CreatedBy, SubjectID: liker.ID, ObjectID: post.ID, TargetID: post.CreatedBy, ViewerID: liker.ID,
	}
	h.later(func() {
		if err := h.Announce(payload, name); err != nil {
			log.Printf("saying that %s took a like of %d back: %v", liker.Alias, post.ID, err)
		}
	})
	return nil
}

func (h *Handlers) unrepostedOnBluesky(kept *repositories.KeptReaction) error {
	has, err := h.Reposts.IsReposted(kept.ActorID, kept.PostID)
	if err != nil || !has {
		return err
	}
	if err := h.Reposts.Delete(&services.RepostDeleteRequest{ParentID: kept.PostID, AuthorID: kept.ActorID}); err != nil {
		return err
	}
	if h.Announce == nil {
		return nil
	}
	post, err := h.Nodes.Read(kept.PostID)
	if err != nil || post == nil || post.ID == 0 {
		return err
	}

	// The author here was told of the repost; the news of it goes with it.
	payload := &event.EventPayload{
		Name: event.RemoteRepostDeleted, SourceService: repostService,
		SubjectID: kept.ActorID, SubjectType: constants.NodeActorATProtoPersonEntityType,
		ObjectID: post.ID, ObjectType: post.Type,
		TargetID: kept.ActorID, TargetType: constants.NodeActorATProtoPersonEntityType, ViewerID: kept.ActorID,
	}
	h.later(func() {
		if err := h.Announce(payload, event.RemoteRepostDeleted); err != nil {
			log.Printf("saying that a repost of %d was taken back: %v", post.ID, err)
		}
	})
	return nil
}

// verifyReplies removes the answers that are gone from Bluesky.
func (h *Handlers) verifyReplies(ctx context.Context) {
	replies, err := h.Kept.Replies(verifyMost)
	if err != nil {
		log.Println("the answers kept from Bluesky could not be read:", err)
		return
	}
	for _, each := range replies {
		if ctx.Err() != nil {
			return
		}
		_, rkey, ok := bsky.ParsePostURI(each.URI)
		if !ok {
			continue
		}
		// Only the reader saying it is not there removes it.
		if _, err := h.Bluesky.PostAt(ctx, each.URI); !errors.Is(err, bsky.ErrNobody) {
			continue
		}
		actor, err := h.Actors.ReadByNode(each.ActorID)
		if err != nil || actor == nil {
			continue
		}
		if err := h.removePost(actor, rkey); err != nil {
			log.Printf("an answer taken back on Bluesky (%s) was not removed here: %v", each.URI, err)
		}
	}
}

// verifyFollows removes the follows of people here that are no longer
// follows on Bluesky.
func (h *Handlers) verifyFollows(ctx context.Context) {
	follows, err := h.Kept.Follows(verifyMost)
	if err != nil {
		log.Println("the follows kept from Bluesky could not be read:", err)
		return
	}
	for _, each := range follows {
		if ctx.Err() != nil {
			return
		}
		still, err := h.Verify.IsFollowing(ctx, each.ActorDID, each.PersonDID)
		if err != nil || still {
			continue
		}
		follower, err := h.Nodes.Read(each.ActorID)
		if err != nil || follower == nil || follower.ID == 0 {
			continue
		}
		person, err := h.Nodes.Read(each.PersonID)
		if err != nil || person == nil || person.ID == 0 {
			continue
		}
		viewer := entities.NewViewer()
		viewer.ID = follower.ID
		remove := services.NewSocialgraphFollowRequest().Delete
		remove.Source, remove.Target, remove.Viewer = follower, person, viewer
		if err := h.FollowEdges.Delete(remove); err != nil {
			log.Printf("%s no longer follows %s on Bluesky, and still does here: %v", follower.Alias, person.Alias, err)
		}
	}
}
