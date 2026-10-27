package handlers

import (
	"log"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/permissions"

	"github.com/labstack/echo/v4"
)

// requestOwnersKey holds the owners read so far in this request, by id.
const requestOwnersKey = "anahita.post_flags.owners"

// SetPostFlags answers whether the viewer may reply to and like post, onto
// post.Authorized, from the rules for replying and for liking.
//
// Both need the post's owner as the actor read returns it — its access level,
// and its add_permissions with the actor_features defaults merged in. A medium
// read carries only the owner's id, so the owner is read here: once per owner
// per request, since a list is usually one profile's posts.
//
// The answers are worked out on a copy of the post with that owner attached.
// post.Owner is left as it was, so the owner a response shows does not change.
//
// rel is the owner's relationship to the viewer. When the owner cannot be read
// both answers are left unset, which the web app reads as "not answered here".
func SetPostFlags(c echo.Context, actors services.ActorsGRPCInterface, post *entities.AnahitaNode, viewer *entities.Viewer, rel *entities.ActorRelation) {
	if post == nil || post.Authorized == nil || post.OwnerID <= 0 {
		return
	}

	// Whether what it holds may be downloaded. Said only of a post that
	// holds something, and needs nothing but the post itself.
	if permissions.HoldsAFile(post) {
		download := permissions.CanDownload(viewer, post)
		post.Authorized.Download = &download
	}

	owner, err := requestOwner(c, actors, post.OwnerID, viewer)
	if err != nil {
		return
	}

	withOwner := *post
	withOwner.Owner = owner

	comment := permissions.CanReplyToPost(viewer, &withOwner, rel)
	like := permissions.CanLike(viewer, &withOwner, rel)
	quote := permissions.CanQuotePost(viewer, &withOwner, rel)
	post.Authorized.Comment = &comment
	post.Authorized.Like = &like
	pin := permissions.CanPin(viewer, &withOwner, rel)
	post.Authorized.Quote = &quote
	post.Authorized.Pin = &pin
	blockFromOwner := permissions.CanBlockFromOwner(viewer, owner, post.GetAuthorID(), rel)
	post.Authorized.BlockFromOwner = &blockFromOwner
}

func requestOwner(c echo.Context, actors services.ActorsGRPCInterface, id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	owners, _ := c.Get(requestOwnersKey).(map[int64]*entities.AnahitaNode)
	if owners == nil {
		owners = map[int64]*entities.AnahitaNode{}
		c.Set(requestOwnersKey, owners)
	}

	if owner, ok := owners[id]; ok {
		return owner, nil
	}

	owner, err := actors.Read(id, viewer)
	if err != nil {
		return nil, err
	}

	owners[id] = owner
	return owner, nil
}

// SetListPostFlags answers for each post in a list what SetPostFlags answers
// for one post on its own page: whether the viewer may reply to it, like it,
// quote it, and the rest. A list that left them out had the web app read
// every one as "no": Quote was switched off on the viewer's own public posts.
//
// How the viewer stands with each owner is read once for the whole list.
// A failure to read it is logged, and the answers are worked out as for
// somebody with no relation to the owner.
func SetListPostFlags(c echo.Context, actors services.ActorsGRPCInterface, relations RelationReader, posts []*entities.AnahitaNode, viewer *entities.Viewer) {
	if len(posts) == 0 {
		return
	}

	rels := map[int64]*entities.ActorRelation{}
	if relations != nil && viewer != nil && viewer.ID > 0 {
		seen := map[int64]bool{}
		ownerIDs := make([]int64, 0, len(posts))
		for _, post := range posts {
			if post != nil && post.OwnerID > 0 && !seen[post.OwnerID] {
				seen[post.OwnerID] = true
				ownerIDs = append(ownerIDs, post.OwnerID)
			}
		}
		if read, err := relations.Browse(ownerIDs, viewer.ID); err != nil {
			log.Println("list post flags: reading relationships:", err)
		} else if read != nil {
			rels = read
		}
	}

	for _, post := range posts {
		if post == nil {
			continue
		}
		if post.Authorized == nil {
			post.Authorized = &entities.AnahitaNodeAuthorized{}
		}
		SetPostFlags(c, actors, post, viewer, rels[post.OwnerID])
	}
}
