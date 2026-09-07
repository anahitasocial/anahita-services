package repositories

import (
	"anahita-libs/constants"
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
// Archive is handled by SetArchiveQuery rather than here, because unlike
// deleted and disabled it is NOT a simple exclusion — an archived person stays
// findable while an archived group does not. See that function.
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

// SetArchiveQuery excludes archived actors and their content from a PUSH
// surface — feeds, suggestions, anything delivered to somebody who did not ask.
//
// Archive is not deletion. The content is preserved and stays readable at its
// own URLs forever; what stops is being put in front of people. So this is
// deliberately NOT applied to a direct read, and applying it there would
// destroy the whole point of archiving rather than deleting.
//
// The owner half is the same derivation everything else uses: `medium.owner_id`
// is the actor a medium was posted TO, so archiving a group archives its posts
// without writing a flag onto 2,228 rows that could then disagree with it.
func SetArchiveQuery(sb sq.SelectBuilder, viewer *entities.Viewer) sq.SelectBuilder {
	if viewer != nil && viewer.IsAdmin() {
		return sb
	}

	return sb.
		Where(sq.Eq{"node.archived_at": nil}).
		Where(sq.Eq{"owner.archived_at": nil})
}

// SetArchiveSearchQuery excludes archived GROUPS and their content from search,
// while leaving archived PEOPLE in it.
//
// The rule looks fussy and is not. Two requirements share the word "archive"
// and want opposite things from search:
//
//   - a memorial — somebody who died — must stay findable. A memorial nobody
//     can find is not one, and this is the case archive-for-people exists to
//     serve.
//   - legacy cleanup — the PHP-era groups, 2,228 posts between four of them —
//     must stop competing with live spaces. That is the entire point of
//     archiving them.
//
// The node type already separates the two, so this costs no extra flag: a
// person is an identity people search for by name, a finished group is content
// that should stop surfacing.
//
// Note this filters on the OWNER's type as well as the node's. A post owned by
// an archived group leaves search with the group; the same post owned by an
// archived person stays, because it is part of what that person left behind.
func SetArchiveSearchQuery(sb sq.SelectBuilder, viewer *entities.Viewer) sq.SelectBuilder {
	if viewer != nil && viewer.IsAdmin() {
		return sb
	}

	person := constants.NodeActorPersonEntityType

	// Archived and not a person: out. Archived and a person: stays.
	sb = sb.Where(sq.Or{
		sq.Eq{"node.archived_at": nil},
		sq.Eq{"node.type": person},
	})

	// And the same question about whatever owns it.
	return sb.Where(sq.Or{
		sq.Eq{"owner.archived_at": nil},
		sq.Eq{"owner.type": person},
	})
}
