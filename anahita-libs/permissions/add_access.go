package permissions

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// AddAccessAllows reports whether the viewer meets an add_permissions access
// level on an actor, given the actor's relationship to the viewer.
//
// One place for what each level means. The comment, like, repost, medium and
// follower checks each used to carry their own switch over the values, every
// one of them handling admins, followers and registered and nothing else — so
// leaders and mutuals, which the person validator accepted, matched no case
// and quietly refused everybody but the owner.
//
// An actor's administrators meet every level. The owner and site
// administrators are the caller's to let through before asking: whether they
// may is a rule of the action, not of the level.
func AddAccessAllows(access string, rel *entities.ActorRelation, viewer *entities.Viewer) bool {
	if rel == nil {
		rel = &entities.ActorRelation{}
	}

	if rel.HasAdmin {
		return true
	}

	switch access {
	case constants.AnahitaAccessRegistered:
		return viewer != nil && viewer.Authenticated && viewer.ID > 0
	case constants.AnahitaAccessFollowers:
		// IsLeading is actorB -> actorA: the viewer follows the actor.
		return rel.IsLeading
	case constants.AnahitaAccessLeaders:
		// IsFollowing is actorA -> actorB: the actor follows the viewer.
		return rel.IsFollowing
	case constants.AnahitaAccessMutuals:
		return rel.IsMutual || (rel.IsLeading && rel.IsFollowing)
	}

	// admins is met by HasAdmin above; anything unrecognised is met by nobody.
	return false
}

// CommentAccess returns the level a viewer has to meet to comment on the
// owner's posts under service, or "" when being able to read the post is
// enough.
//
// The setting only means something on a profile anybody signed in can see.
// There it is registered or followers, registered when unset.
//
// A followers-only profile has followers-only comments whatever is stored.
// Said explicitly rather than left to the read check, because reading such a
// profile also admits the people it follows, and they are not its followers.
//
// On anything narrower still — leaders, mutuals, admins, myself — the
// profile's access already names who can see the post, and they can comment.
func CommentAccess(owner *entities.AnahitaNode, service string) string {
	if owner == nil {
		return ""
	}

	switch owner.GetAccess() {
	case constants.AnahitaAccessPublic, constants.AnahitaAccessRegistered:
		if owner.AddPermissionAccess(service, "comment") == constants.AnahitaAccessFollowers {
			return constants.AnahitaAccessFollowers
		}
		return constants.AnahitaAccessRegistered
	case constants.AnahitaAccessFollowers:
		return constants.AnahitaAccessFollowers
	}

	return ""
}
