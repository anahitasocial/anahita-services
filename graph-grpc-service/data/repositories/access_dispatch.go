package repositories

import (
	"strings"

	"anahita-libs/constants"
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// Picking the right access gate for a generic node browse.
//
// Nodes.Browse serves both families — the medium handler asks it for photos
// and notes, the actor handler for people and groups — and the two need
// DIFFERENT gates, which is why one could not simply be dropped in:
//
//   - an ACTOR's audience is measured against the actor itself. "Followers
//     only" means the people who follow THIS actor.
//   - a MEDIUM's is measured against its owner. "Followers only" on a photo
//     means the people who follow the actor it was posted to; a photo has no
//     followers of its own.
//
// Using the owner-based gate for an actor would ask whether the viewer follows
// the actor's owner. An actor has none, so every relation is false and a
// followers-only profile disappears from its own followers — wrong in the
// opposite direction from a leak, and just as wrong.
//
// THE GATE WAS NOT APPLIED AT ALL. Both call sites read
//
//	if req.AccessCheck {
//	    // query = SetAccessQuery(query, req.Viewer)
//	}
//
// so browsing returned everything regardless of access, while reading the same
// node directly was correctly refused. Verified against the development data:
// 598 non-public media and 78 non-public actors were listed to a logged-out
// visitor, including one restricted to administrators.

// allActorTypes reports whether every requested type is an actor type.
//
// Requires at least one: an empty list means "no type filter", which cannot be
// assumed to be actors and takes the owner-based gate below.
func allActorTypes(types []string) bool {
	if len(types) == 0 {
		return false
	}

	for _, t := range types {
		if !strings.HasPrefix(t, constants.NodeActorPrefixEntityType) {
			return false
		}
	}

	return true
}

// setAccessQueryForTypes applies the gate matching what is being browsed.
//
// Anything that is not unambiguously a list of actors gets the owner-based
// gate. That is right for media, and for a mixed or unfiltered browse it errs
// towards hiding rather than showing — the safe direction for a predicate
// whose job is to keep things private.
func setAccessQueryForTypes(sb sq.SelectBuilder, viewer *entities.Viewer, types []string) sq.SelectBuilder {
	if viewer == nil {
		viewer = entities.NewViewer()
	}

	if allActorTypes(types) {
		return SetActorAccessQuery(sb, viewer)
	}

	return SetMediumAccessQuery(sb, viewer)
}
