package repositories

import (
	"context"
	"database/sql"
	"time"

	"anahita-libs/constants"
)

// DeletionCounts is what an actor stands to lose.
type DeletionCounts struct {
	Posts              int64
	Comments           int64
	Followers          int64
	Following          int64
	Admins             int64
	GroupsAdministered int64
	MemberSince        time.Time
}

// ReadDeletionCounts totals what deleting an actor would take with it.
//
// For the confirmation dialog, and it exists because "your posts will be
// removed" reads as boilerplate while "2,931 posts" does not. The figures are
// the whole point, so they are gathered before the deletion rather than
// reported after it.
//
// IT COUNTS WHAT THE PURGE DESTROYS, and the set is the purge's own:
// purgeContent, the same text PreparePurge runs. Everything the actor owns,
// everything the actor wrote, and everything underneath either.
//
// The two have disagreed before, in both directions. Counting only what the
// actor wrote missed the comments other people left on their posts, which go
// with the posts. Counting only what the actor owned was right while the purge
// did the same, and stopped being right when the purge learned to follow the
// actor's own posts and comments into groups and onto other people's posts.
// Sharing the text is what keeps a dialog whose entire job is to be believed
// from drifting again.
//
// One statement, and the tree is walked once. Six round trips would sit in
// front of a dialog somebody is waiting on, and six snapshots can disagree.
func (r *nodesRepo) ReadDeletionCounts(actorID int64) (*DeletionCounts, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if actorID <= 0 {
		return &DeletionCounts{}, nil
	}

	const query = purgeContent + `,
destroyed AS (
  SELECT
    COALESCE(SUM(n.type LIKE ?), 0) AS posts,
    COALESCE(SUM(n.type = ?), 0)    AS comments
  FROM content c
  JOIN nodes n ON n.id = c.id
  WHERE n.id <> ?
)
SELECT
  d.posts,
  d.comments,
  -- Actors following this one, and actors it follows. Both directions,
  -- because losing an audience and losing a feed are different losses.
  (SELECT COUNT(*) FROM edges e WHERE e.node_b_id = ? AND e.type = ?) AS followers,
  (SELECT COUNT(*) FROM edges e WHERE e.node_a_id = ? AND e.type = ?) AS following,
  -- Who administers this actor, and what this actor administers. An admin
  -- edge runs node_a (the administrator) -> node_b (the thing administered),
  -- so the two are the same edge type read from opposite ends: the first is
  -- the number that matters for a group, the second for a person.
  (SELECT COUNT(*) FROM edges e WHERE e.node_b_id = ? AND e.type = ?) AS admins,
  (SELECT COUNT(*) FROM edges e WHERE e.node_a_id = ? AND e.type = ?) AS groups_administered,
  (SELECT created_on FROM nodes WHERE id = ?) AS member_since
FROM destroyed d;
`

	var counts DeletionCounts
	// Nullable: an actor that does not exist yields NULL rather than no row,
	// because every term is a scalar subquery. Scanning into a plain time.Time
	// would fail the whole read for a bad id instead of returning zeroes.
	var memberSince sql.NullTime

	err := r.db.QueryRowContext(ctx, query,
		actorID, actorID,
		constants.NodeMediumPrefixEntityType+"%",
		constants.NodeBaseCommentEntityType,
		actorID,
		actorID, constants.EdgeFollowEntityType,
		actorID, constants.EdgeFollowEntityType,
		actorID, constants.EdgeActorAdminEntityType,
		actorID, constants.EdgeActorAdminEntityType,
		actorID,
	).Scan(
		&counts.Posts,
		&counts.Comments,
		&counts.Followers,
		&counts.Following,
		&counts.Admins,
		&counts.GroupsAdministered,
		&memberSince,
	)
	if err != nil {
		return nil, err
	}

	if memberSince.Valid {
		counts.MemberSince = memberSince.Time
	}

	return &counts, nil
}
