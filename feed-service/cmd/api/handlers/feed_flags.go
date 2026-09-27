package handlers

import (
	"log"

	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
)

// setFeedFlags answers, for every post in a feed, whether the viewer may
// comment on it and like it — onto the post itself, or for a repost onto the
// post it reposts, which is what the feed's buttons act on.
//
// The answers come from the checks comment-service and like-service enforce.
// Both need each post's owner as the actor read returns it, and the owner's
// relationship to the viewer, so those are read once for the whole page: one
// batch read of the distinct owners, one relationship lookup.
//
// On any failure the answers are left unset rather than guessed, which the web
// app reads as "not answered here" and falls back on.
func setFeedFlags(env *Env, nodes []*entities.AnahitaNode, viewer *entities.Viewer) {
	posts := make([]*entities.AnahitaNode, 0, len(nodes))
	ownerIDs := []int64{}
	seen := map[int64]bool{}

	for _, node := range nodes {
		post := node
		if !node.IsMedium() {
			post = node.Parent
		}
		if post == nil || !post.IsMedium() || post.Authorized == nil || post.OwnerID <= 0 {
			continue
		}
		posts = append(posts, post)
		if !seen[post.OwnerID] {
			seen[post.OwnerID] = true
			ownerIDs = append(ownerIDs, post.OwnerID)
		}
	}

	if len(posts) == 0 {
		return
	}

	req := services.NewActorGRPCRequest().BatchRead
	req.IDs = ownerIDs
	owners, err := env.Srv.ActorsGRPC.BatchRead(req, viewer)
	if err != nil {
		log.Println("feed flags: reading owners:", err)
		return
	}
	ownersByID := make(map[int64]*entities.AnahitaNode, len(owners))
	for _, owner := range owners {
		ownersByID[owner.ID] = owner
	}

	rels, err := env.Srv.SocialgraphGRPC.Browse(ownerIDs, viewer.ID)
	if err != nil {
		log.Println("feed flags: reading relationships:", err)
		return
	}

	for _, post := range posts {
		owner, ok := ownersByID[post.OwnerID]
		if !ok {
			continue
		}

		// A copy with the owner attached, so the owner the feed shows is
		// left exactly as the feed read returned it.
		withOwner := *post
		withOwner.Owner = owner

		comment := permissions.CanComment(viewer, &withOwner, rels[post.OwnerID])
		like := permissions.CanLike(viewer, &withOwner, rels[post.OwnerID])
		post.Authorized.Comment = &comment
		post.Authorized.Like = &like
	}
}
