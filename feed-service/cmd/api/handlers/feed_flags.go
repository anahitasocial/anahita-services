package handlers

import (
	"log"

	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
)

// setFeedFlags answers, for every post in a feed, whether the viewer may
// reply to it and like it — onto the post itself, or for a repost onto the
// post it reposts, which is what the feed's buttons act on.
//
// The answers come from the rules for replying and for liking.
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

		comment := permissions.CanReplyToPost(viewer, &withOwner, rels[post.OwnerID])
		like := permissions.CanLike(viewer, &withOwner, rels[post.OwnerID])
		post.Authorized.Comment = &comment
		post.Authorized.Like = &like
	}
}

// setReplyFeedFlags answers, for every reply in a profile's list of replies,
// what the viewer may do with it: reply to it, like it, edit it, remove it,
// hide it.
//
// A reply is judged by the post at the top of its thread and never by
// itself, so setFeedFlags would get every one of these wrong: it would ask
// the reply's author's profile. The roots are read here, once for the page,
// with their owners and both relationships the rule needs.
//
// Each reply is also given its root, with the profile the thread is on, to
// be sent with it: a reply is owned by whoever wrote it, and the list has to
// be able to say where it was said.
//
// The roots are read without a viewer. That is safe here and nowhere else:
// every reply in the list has already passed the access gate, which admits a
// reply only when the viewer may see its root.
//
// On any failure the answers are left unset rather than guessed.
func setReplyFeedFlags(env *Env, replies []*entities.AnahitaNode, viewer *entities.Viewer) {
	rootIDs := []int64{}
	seen := map[int64]bool{}
	for _, reply := range replies {
		if reply.RootID > 0 && !seen[reply.RootID] {
			seen[reply.RootID] = true
			rootIDs = append(rootIDs, reply.RootID)
		}
	}
	if len(rootIDs) == 0 {
		return
	}

	rootReq := services.NewGraphNodeRequest().BatchRead
	rootReq.IDs = rootIDs
	roots, err := env.Srv.GraphNodeGRPC.BatchRead(rootReq)
	if err != nil {
		log.Println("reply flags: reading roots:", err)
		return
	}

	rootsByID := make(map[int64]*entities.AnahitaNode, len(roots))
	actorIDs := []int64{}
	ownerIDs := []int64{}
	seenActor := map[int64]bool{}
	seenOwner := map[int64]bool{}
	for _, root := range roots {
		rootsByID[root.ID] = root
		if root.OwnerID > 0 && !seenOwner[root.OwnerID] {
			seenOwner[root.OwnerID] = true
			ownerIDs = append(ownerIDs, root.OwnerID)
		}
		for _, id := range []int64{root.OwnerID, root.GetAuthorID()} {
			if id > 0 && !seenActor[id] {
				seenActor[id] = true
				actorIDs = append(actorIDs, id)
			}
		}
	}
	if len(ownerIDs) == 0 {
		return
	}

	ownerReq := services.NewActorGRPCRequest().BatchRead
	ownerReq.IDs = ownerIDs
	owners, err := env.Srv.ActorsGRPC.BatchRead(ownerReq, viewer)
	if err != nil {
		log.Println("reply flags: reading owners:", err)
		return
	}
	ownersByID := make(map[int64]*entities.AnahitaNode, len(owners))
	for _, owner := range owners {
		ownersByID[owner.ID] = owner
	}

	rels, err := env.Srv.SocialgraphGRPC.Browse(actorIDs, viewer.ID)
	if err != nil {
		log.Println("reply flags: reading relationships:", err)
		return
	}

	// The roots as they are sent, each with its owner as the actor read
	// returned it, and the images of both made ready.
	sent := make(map[int64]*entities.AnahitaNode, len(roots))
	sentList := make([]*entities.AnahitaNode, 0, len(roots))
	for _, root := range roots {
		owner, ok := ownersByID[root.OwnerID]
		if !ok {
			continue
		}
		withOwner := *root
		withOwner.Owner = owner
		sent[root.ID] = &withOwner
		sentList = append(sentList, &withOwner)
	}
	batchPresignPaths(env, sentList)

	for _, reply := range replies {
		withOwner, ok := sent[reply.RootID]
		if !ok {
			continue
		}
		reply.Root = withOwner

		if reply.Authorized == nil {
			continue
		}
		root := withOwner

		ctx := &permissions.ReplyAccessContext{
			Viewer:              viewer,
			Reply:               reply,
			Root:                withOwner,
			RootOwnerViewerRel:  rels[root.OwnerID],
			RootAuthorViewerRel: rels[root.GetAuthorID()],
		}

		hide := permissions.CanHideReply(ctx)
		canReply := permissions.CanReply(ctx)
		like := permissions.CanLikeReply(ctx)

		reply.Authorized.Edit = permissions.CanEditReply(ctx)
		reply.Authorized.Delete = permissions.CanDeleteReply(ctx)
		reply.Authorized.Hide = &hide
		reply.Authorized.Comment = &canReply
		reply.Authorized.Like = &like
	}
}
