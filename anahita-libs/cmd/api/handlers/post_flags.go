package handlers

import (
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"

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
