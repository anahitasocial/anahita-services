package handlers

import (
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"

	"github.com/labstack/echo/v4"
)

// requestOwnersKey holds the owners read so far in this request, by id.
const requestOwnersKey = "anahita.post_flags.owners"

// SetPostFlags answers whether the viewer may comment on and like post, onto
// post.Authorized, from the checks comment-service and like-service enforce.
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

	owner, err := requestOwner(c, actors, post.OwnerID, viewer)
	if err != nil {
		return
	}

	withOwner := *post
	withOwner.Owner = owner

	comment := permissions.CanComment(viewer, &withOwner, rel)
	like := permissions.CanLike(viewer, &withOwner, rel)
	post.Authorized.Comment = &comment
	post.Authorized.Like = &like
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
