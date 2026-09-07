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
	GroupsAdministered int64
	MemberSince        time.Time
}

// ReadDeletionCounts totals what deleting an actor would take with it.
//
// For the confirmation dialog, and it exists because "your posts will be
// removed" reads as boilerplate while "4,826 posts" does not. The figures are
// the whole point, so they are gathered before the deletion rather than
// reported after it.
//
// One statement rather than six round trips. Six separate queries against a
// database this size is six network waits in front of a dialog somebody is
// waiting on, and they cannot disagree with each other if they are one
// snapshot.
//
// Scalar subqueries rather than joins: each count is over a different table on
// a different key, and joining them would multiply rows together into totals
// that are products rather than counts — the classic fan-out that turns 200
// followers and 20 groups into 4,000 of something.
func (r *nodesRepo) ReadDeletionCounts(actorID int64) (*DeletionCounts, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if actorID <= 0 {
		return &DeletionCounts{}, nil
	}

	const query = `
SELECT
  (SELECT COUNT(*) FROM nodes m
     WHERE m.created_by = ? AND m.type LIKE ? AND m.deleted_at IS NULL) AS posts,
  (SELECT COUNT(*) FROM nodes c
     WHERE c.created_by = ? AND c.type = ?) AS comments,
  (SELECT COUNT(*) FROM edges e
     WHERE e.node_b_id = ? AND e.type = ?) AS followers,
  (SELECT COUNT(*) FROM edges e
     WHERE e.node_a_id = ? AND e.type = ?) AS following,
  (SELECT COUNT(*) FROM edges e
     WHERE e.node_a_id = ? AND e.type = ?) AS groups_administered,
  (SELECT created_on FROM nodes WHERE id = ?) AS member_since;
`

	var counts DeletionCounts
	// Nullable: an actor that does not exist yields NULL rather than no row,
	// because every term is a scalar subquery. Scanning into a plain time.Time
	// would fail the whole read for a bad id instead of returning zeroes.
	var memberSince sql.NullTime

	err := r.db.QueryRowContext(ctx, query,
		actorID, constants.NodeMediumPrefixEntityType+"%",
		actorID, constants.NodeBaseCommentEntityType,
		actorID, constants.EdgeFollowEntityType,
		actorID, constants.EdgeFollowEntityType,
		actorID, constants.EdgeActorAdminEntityType,
		actorID,
	).Scan(
		&counts.Posts,
		&counts.Comments,
		&counts.Followers,
		&counts.Following,
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
