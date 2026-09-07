package repositories

import (
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// SetLifecycleQuery excludes rows that should not be found at all — because
// the node is deleted or disabled, or because the actor that owns it is.
//
// Separate from the access query on purpose. Access answers "may this viewer
// see it"; lifecycle answers "is it there". They happen to share an
// administrator bypass, which is why they were easy to conflate, but a
// disabled group is not a privacy setting and should not be expressed as one.
//
// THE GAP THIS CLOSES. Search filtered neither, for the node or its owner:
// deleting a group left the group itself and every one of its posts in search
// results for the whole grace period, and disabling one did nothing at all.
// Verified against the running stack — a group with deleted_at set and
// enabled = 0 still came back from /search, along with its post.
//
// The owner half is the same shape access already uses, and the reason is the
// same: a post inherits its container's state. `medium.owner_id` is the actor
// a medium was posted TO, so a group's deletion is its posts' deletion.
//
// Archive slots in here when the column exists — `archived_at` on both the
// node and the owner — which is why this is a function rather than four lines
// in the search query.
func SetLifecycleQuery(sb sq.SelectBuilder, viewer *entities.Viewer) sq.SelectBuilder {
	// Administrators see everything, matching SetSearchAccessQuery. Moderation
	// needs to find the thing it is moderating.
	if viewer != nil && viewer.IsAdmin() {
		return sb
	}

	// Deleted, on the node and on its owner.
	//
	// Written as IS NULL rather than a COALESCE because the owner join is a
	// LEFT JOIN: a node with no owner produces NULL here and passes, which is
	// the correct reading — it is not owned by a deleted actor.
	sb = sb.Where(sq.Eq{"node.deleted_at": nil}).
		Where(sq.Eq{"owner.deleted_at": nil})

	// Disabled, on the node and on its owner.
	//
	// COALESCE for the owner, because here NULL means "no owner row" and must
	// read as enabled rather than as disabled — the opposite default from
	// deleted_at above, and the reason these are not one predicate.
	sb = sb.Where(sq.Eq{"node.enabled": true}).
		Where("COALESCE(owner.enabled, 1) = 1")

	return sb
}
