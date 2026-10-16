package repositories

import (
	"context"
	"database/sql"
	"strings"

	"anahita-libs/constants"
)

// CitationsRepoInterface keeps nodes.citation_count in step with the
// citations there are (edge.medium.citation.v1, from an article or a topic
// to a post or a profile it shows within itself).
//
// A post is cited by as many people as cite it: each person once, however
// many of their posts cite it, and never its own author. A person, a group
// or an event is cited by as many people as cite it, its profile or any of
// its posts, never itself. Only a citation in a published post that anybody
// may read counts: not a draft, not one taken down or switched off, not one
// with a closer audience. (The user, 2026-10-16: a post cited more, and its
// author and owner, count for more in search and recommendations.)
//
// Counted over the rows, never added to and taken from, so a count is right
// whatever happened before it.
type CitationsRepoInterface interface {
	// Recount puts right the counts of these nodes, and of the people and
	// profiles their posts are by and on.
	Recount(ids []int64) error

	// CitedBy is what a post cites: the nodes its citations run to, whose
	// counts change with it.
	CitedBy(citingID int64) ([]int64, error)
}

func NewCitationsRepo(db *sql.DB) CitationsRepoInterface {
	return &citationsRepo{db: db}
}

type citationsRepo struct {
	db *sql.DB
}

// What a citing post has to be to count.
const citingCounts = `
  AND citing.enabled = 1
  AND citing.deleted_at IS NULL
  AND citing.draft = 0
  AND citing.access = 'public'`

const countPostCitations = `
SELECT COUNT(DISTINCT citing.created_by)
FROM edges cite
JOIN nodes citing
  ON citing.id = cite.node_a_id
JOIN nodes cited
  ON cited.id = cite.node_b_id
WHERE cite.type = ?
  AND cite.node_b_id = ?
  AND citing.created_by <> cited.created_by` + citingCounts

const countActorCitations = `
SELECT COUNT(DISTINCT citing.created_by)
FROM edges cite
JOIN nodes citing
  ON citing.id = cite.node_a_id
JOIN nodes target
  ON target.id = cite.node_b_id
WHERE cite.type = ?
  AND (target.id = ? OR target.owner_id = ? OR target.created_by = ?)
  AND citing.created_by <> ?
  AND citing.owner_id <> ?` + citingCounts

const readCitedNode = `
SELECT type, COALESCE(owner_id, 0), COALESCE(created_by, 0)
FROM nodes
WHERE id = ?`

const setCitationCount = `
UPDATE nodes
SET citation_count = ?
WHERE id = ?`

const readCitedBy = `
SELECT node_b_id
FROM edges
WHERE type = ?
  AND node_a_id = ?`

func (r *citationsRepo) Recount(ids []int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	done := map[int64]bool{}
	actors := []int64{}
	for _, id := range ids {
		if id <= 0 || done[id] {
			continue
		}
		done[id] = true

		var kind string
		var ownerID, createdBy int64
		err := r.db.QueryRowContext(ctx, readCitedNode, id).Scan(&kind, &ownerID, &createdBy)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}

		if strings.HasPrefix(kind, "node.actor.") {
			actors = append(actors, id)
			continue
		}

		var count int64
		if err := r.db.QueryRowContext(ctx, countPostCitations, constants.EdgeMediumCitationType, id).Scan(&count); err != nil {
			return err
		}
		if _, err := r.db.ExecContext(ctx, setCitationCount, count, id); err != nil {
			return err
		}
		// Whose it is, and where it is, count it too.
		actors = append(actors, ownerID, createdBy)
	}

	for _, id := range actors {
		if id <= 0 || done[-id] {
			continue
		}
		done[-id] = true
		var count int64
		if err := r.db.QueryRowContext(ctx, countActorCitations,
			constants.EdgeMediumCitationType, id, id, id, id, id).Scan(&count); err != nil {
			return err
		}
		if _, err := r.db.ExecContext(ctx, setCitationCount, count, id); err != nil {
			return err
		}
	}

	return nil
}

func (r *citationsRepo) CitedBy(citingID int64) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, readCitedBy, constants.EdgeMediumCitationType, citingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
