package repositories

import (
	"context"
	"fmt"
	"strings"

	"anahita-libs/constants"
)

// Thread is what a list says of a note that opens its writer's chain
// (anahita-web roadmap row 126): how many notes the chain has, the first
// counted, and the second, which the list shows under it.
type Thread struct {
	Count  int64
	NextID int64
}

// ThreadDepthMax is the longest chain followed: far more than the ten notes
// a thread written in one go may have, and a stop for one added to for
// years.
const ThreadDepthMax = 200

// threadsQuery follows, from each note asked about, its writer's replies
// down the chain: a reply by the same writer to the note before. Only notes
// written here that answer nothing begin one. A reply that was deleted or
// turned off ends the chain there. Counted on each read, not kept: always
// right after an edit, a deletion or a purge, with nothing to keep in step.
const threadsQuery = `
WITH RECURSIVE chain (root_id, id, writer, depth) AS (
	SELECT
		node.id,
		node.id,
		node.created_by,
		1
	FROM nodes AS node
	WHERE node.id IN (%s)
		AND node.type = ?
		AND node.parent_id IS NULL
		AND node.uri IS NULL
	UNION ALL
	SELECT
		chain.root_id,
		reply.id,
		chain.writer,
		chain.depth + 1
	FROM chain
	JOIN nodes AS reply ON reply.parent_id = chain.id
		AND reply.created_by = chain.writer
	WHERE reply.type = ?
		AND reply.enabled = 1
		AND reply.deleted_at IS NULL
		AND chain.depth < ?
)
SELECT
	root_id,
	COUNT(*),
	COALESCE(MIN(CASE WHEN depth = 2 THEN id END), 0)
FROM chain
GROUP BY root_id
HAVING COUNT(*) > 1`

// Threads says, for each of the notes given that opens a chain of its
// writer's, how long the chain is and which note comes second. A note that
// opens none is not in the answer. One query for a page.
func (r *nodesRepo) Threads(ids []int64) (map[int64]Thread, error) {
	threads := map[int64]Thread{}
	if len(ids) == 0 {
		return threads, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	args := make([]any, 0, len(ids)+3)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, constants.NodeMediumNoteEntityType, constants.NodeMediumNoteEntityType, ThreadDepthMax)
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(threadsQuery, marks), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var root int64
		var thread Thread
		if err := rows.Scan(&root, &thread.Count, &thread.NextID); err != nil {
			return nil, err
		}
		threads[root] = thread
	}
	return threads, rows.Err()
}
