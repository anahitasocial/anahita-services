package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"context"
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

// ThreadLimit is the most replies one thread is read with. The largest
// thread there was when this was written had 66.
const ThreadLimit = 500

// RepliesRepoInterface is what a thread needs beyond what any post does: to
// be read whole, to have replies hidden from it, to keep its counts, and to
// keep its shape when a reply in the middle of it is removed.
type RepliesRepoInterface interface {
	// Thread returns every reply under a post, oldest first, that this
	// viewer may see, whatever its depth. Removed replies that still have
	// replies under them come back as empty placeholders.
	Thread(rootID int64, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)

	// HiddenIDs returns the replies hidden from this post's thread.
	HiddenIDs(rootID int64) ([]int64, error)

	// SetHidden hides a reply from its thread, or shows it again.
	SetHidden(rootID int64, reply *entities.AnahitaNode, hidden bool, by int64) error

	// HasReplies reports whether anything answers this node.
	HasReplies(id int64) (bool, error)

	// Tombstone empties a reply and marks it removed, keeping the row so
	// that what was said under it still has something to hang from.
	Tombstone(id int64, by int64) error

	// Recount puts right the reply counts on a thread's root and on one
	// reply in it. Either may be 0 for "none".
	Recount(rootID int64, parentID int64) error

	// TouchedByBlock is what a block of `blocked` by `blocker` changes the
	// counts of: the blocker's threads the blocked person has replied in,
	// and the blocker's posts they have liked.
	TouchedByBlock(blockerID, blockedID int64) (roots []int64, liked []int64, err error)
}

func NewRepliesRepo(db *sql.DB) RepliesRepoInterface {
	return &repliesRepo{db: db}
}

type repliesRepo struct {
	db *sql.DB
}

func (r *repliesRepo) Thread(rootID int64, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	if viewer == nil {
		viewer = entities.NewViewer()
	}

	query := GetBaseSelectQuery().
		Where(sq.Eq{"node.root_id": rootID}).
		Where(sq.Eq{"node.enabled": true})

	// Not the gate a listing of posts uses: this one lets a removed reply
	// through, as the placeholder its own replies hang from. It is the only
	// place one is shown.
	if !viewer.IsAdmin() {
		query = query.Where(threadCondition(viewer))
	}

	query = query.OrderBy("node.created_at ASC", "node.id ASC").Limit(ThreadLimit)

	return FetchNodes(r.db, query)
}

// What is hidden in a thread: what whoever looks after it hid, and what
// somebody wrote whom its post's owner or writer has blocked. A block takes
// out of sight, for every reader, what the blocked person left on what is
// the blocker's, and gives it back if it is lifted.
const selectHiddenReplies = `
SELECT node_b_id
FROM edges
WHERE type = ?
  AND node_a_id = ?
UNION
SELECT reply.id
FROM nodes AS reply
JOIN nodes AS root
  ON root.id = reply.root_id
JOIN edges AS blocked
  ON blocked.type = ?
 AND blocked.node_b_id = reply.created_by
 AND blocked.node_a_id IN (root.owner_id, root.created_by)
WHERE reply.root_id = ?`

func (r *repliesRepo) HiddenIDs(rootID int64) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, selectHiddenReplies,
		constants.EdgeMediumHiddenReplyType, rootID,
		constants.EdgeBlockEntityType, rootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

const deleteHiddenReply = `
DELETE FROM edges
WHERE type = ?
  AND node_a_id = ?
  AND node_b_id = ?`

const insertHiddenReply = `
INSERT INTO edges (
  type,
  node_a_id,
  node_a_type,
  node_b_id,
  node_b_type,
  created_at,
  created_by,
  updated_at,
  modified_by
)
SELECT ?, root.id, root.type, ?, ?, UTC_TIMESTAMP(), ?, UTC_TIMESTAMP(), ?
FROM nodes root
WHERE root.id = ?`

// SetHidden removes any edge there is and, to hide, writes one. Doing both
// makes hiding twice the same as hiding once.
func (r *repliesRepo) SetHidden(rootID int64, reply *entities.AnahitaNode, hidden bool, by int64) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, deleteHiddenReply,
		constants.EdgeMediumHiddenReplyType, rootID, reply.ID,
	); err != nil {
		return err
	}

	if hidden {
		if _, err = tx.ExecContext(ctx, insertHiddenReply,
			constants.EdgeMediumHiddenReplyType, reply.ID, reply.Type, by, by, rootID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

const countRepliesTo = `
SELECT COUNT(*)
FROM nodes
WHERE parent_id = ?
  AND root_id IS NOT NULL`

func (r *repliesRepo) HasReplies(id int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	var count int
	if err := r.db.QueryRowContext(ctx, countRepliesTo, id).Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}

// What is said goes; who said it, when, and where in the thread it was stay,
// so the replies under it are still under something.
const tombstoneReply = `
UPDATE nodes
SET body = '',
    name = '',
    meta = '',
    deleted_at = UTC_TIMESTAMP(),
    modified_by = ?,
    updated_at = UTC_TIMESTAMP()
WHERE id = ?
  AND root_id IS NOT NULL`

func (r *repliesRepo) Tombstone(id int64, by int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, tombstoneReply, by, id)
	return err
}

// A post's count is everything said under it: the replies in its thread that
// are neither removed nor hidden.
//
// Written as a count over the rows, not as "add one", so it is right whatever
// happened before it: this count used to be kept by adding and subtracting,
// through an edit that was refused for want of a modifier, and was simply
// wrong.
const recountThread = `
UPDATE nodes root
SET root.comment_count = (
  SELECT COUNT(*)
  FROM (
    SELECT reply.id
    FROM nodes reply
    WHERE reply.root_id = ?
      AND reply.enabled = 1
      AND reply.deleted_at IS NULL
      AND NOT EXISTS (
        SELECT 1
        FROM edges hidden
        WHERE hidden.type = ?
          AND hidden.node_a_id = reply.root_id
          AND hidden.node_b_id = reply.id
      )
      AND NOT EXISTS (
        SELECT 1
        FROM edges blocked
        JOIN nodes thread_root
          ON thread_root.id = reply.root_id
        WHERE blocked.type = ?
          AND blocked.node_b_id = reply.created_by
          AND blocked.node_a_id IN (thread_root.owner_id, thread_root.created_by)
      )
  ) said
)
WHERE root.id = ?`

// A reply's count is the replies made directly to it.
const recountReply = `
UPDATE nodes parent
SET parent.comment_count = (
  SELECT COUNT(*)
  FROM (
    SELECT reply.id
    FROM nodes reply
    WHERE reply.parent_id = ?
      AND reply.root_id IS NOT NULL
      AND reply.enabled = 1
      AND reply.deleted_at IS NULL
  ) said
)
WHERE parent.id = ?
  AND parent.root_id IS NOT NULL`

const selectRootsRepliedByBlocked = `
SELECT DISTINCT reply.root_id
FROM nodes AS reply
JOIN nodes AS root
  ON root.id = reply.root_id
WHERE reply.created_by = ?
  AND (root.owner_id = ? OR root.created_by = ?)`

const selectPostsLikedByBlocked = `
SELECT DISTINCT liked.node_b_id
FROM edges AS liked
JOIN nodes AS post
  ON post.id = liked.node_b_id
WHERE liked.type = ?
  AND liked.node_a_id = ?
  AND (post.owner_id = ? OR post.created_by = ?)`

func (r *repliesRepo) TouchedByBlock(blockerID, blockedID int64) ([]int64, []int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	read := func(query string, args ...any) ([]int64, error) {
		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	}

	roots, err := read(selectRootsRepliedByBlocked, blockedID, blockerID, blockerID)
	if err != nil {
		return nil, nil, err
	}
	liked, err := read(selectPostsLikedByBlocked, constants.EdgeBaseLikeEntityType, blockedID, blockerID, blockerID)
	return roots, liked, err
}

func (r *repliesRepo) Recount(rootID int64, parentID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	if rootID > 0 {
		if _, err := r.db.ExecContext(ctx, recountThread,
			rootID, constants.EdgeMediumHiddenReplyType,
			constants.EdgeBlockEntityType,
			rootID,
		); err != nil {
			return err
		}
	}

	if parentID > 0 && parentID != rootID {
		if _, err := r.db.ExecContext(ctx, recountReply, parentID, parentID); err != nil {
			return err
		}
	}

	return nil
}
