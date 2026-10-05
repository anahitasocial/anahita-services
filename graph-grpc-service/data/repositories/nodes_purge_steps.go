package repositories

import (
	"context"
	"database/sql"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// The steps of a purge, in the order purge.Account runs them:
//
//	PreparePurge      writes down what there is to do
//	NextPurgeBatch    reads the next few hundred nodes to remove
//	DeletePurgeBatch  removes them, and everything pointing at them
//	FinishPurge       removes what pointed at the actor itself, and counts
//	                  again what other people's nodes lost
//
// It used to be one statement pair in one transaction under the ordinary
// three-second timeout. An account with a few thousand posts ran out of time,
// rolled back, and failed the same way on every later run.
//
// Now the list of work is a table, purge_work, and each batch is its own short
// transaction that removes its rows from the list as it goes. A purge that is
// interrupted — a restart, a timeout, a deadlock — leaves the rest of the list
// behind, and the next call carries on from it. Every step can be run twice.
//
// WHAT IS REMOVED. Everything the actor owns, everything the actor WROTE, and
// everything underneath either by parent_id. The first version walked owner_id
// only, so a post written into a group, a comment on somebody else's post and
// a photo uploaded to a group all survived, credited to a tombstone.
//
// What an actor "wrote" stops at content. A person who created a group, a
// hashtag or a place is its created_by too, and none of those is theirs to
// take with them: other people are in the group and other posts carry the tag.
//
// The walk downwards takes other people's replies with it — a comment
// somebody left on this actor's post is a child of that post. That is the
// ordinary meaning of deleting a post; what matters is that the person is told
// before they confirm, and ReadDeletionCounts counts this same set.

const (
	// purgeStatementTimeout is the limit for one statement of a purge. Each
	// touches a bounded number of rows, so this is generous on purpose: a
	// purge that gives up has to start that statement again.
	purgeStatementTimeout = 30 * time.Second

	// purgeChunk bounds a delete that has no natural batch, such as every
	// notification addressed to one person.
	purgeChunk = 5000

	purgeTaskDelete           = "delete"
	purgeTaskCountFollows     = "count_follows"
	purgeTaskCountVotes       = "count_votes"
	purgeTaskCountSubscribers = "count_subscribers"
	purgeTaskCountComments    = "count_comments"
	purgeTaskCountReposts     = "count_reposts"
)

// purgeContent is the set a purge removes, as a common table expression named
// `content`. Its two placeholders are both the actor's id.
//
// UNION rather than UNION ALL: a node can be reached twice, as something the
// actor wrote and as a child of something the actor owns.
const purgeContent = `
WITH RECURSIVE content (id) AS (
  SELECT id
  FROM nodes
  WHERE owner_id = ?

  UNION

  SELECT id
  FROM nodes
  WHERE created_by = ?
    AND type NOT LIKE 'node.actor.%'
    AND type NOT LIKE 'node.tag.%'

  UNION

  SELECT child.id
  FROM nodes child
  JOIN content parent ON child.parent_id = parent.id
)`

// PurgeBatch is the next nodes to remove.
type PurgeBatch struct {
	// First and Last bound the batch. purge_work is keyed by node id within an
	// actor, so the range is exactly these rows and nothing else.
	First int64
	Last  int64

	// Files are the nodes in the batch that carry an uploaded file, with only
	// the columns the image helpers read.
	Files []*entities.AnahitaNode
}

// PreparePurge writes down what a purge of this actor has to do.
//
// Run before anything is removed, because the second half of the list — whose
// counters to count again — can only be read while the edges and comments
// that point at those nodes still exist.
//
// Safe to run again on a purge already under way: it only adds, and what it
// would add a second time is what has not been dealt with yet.
func (r *nodesRepo) PreparePurge(actorID int64) error {
	if actorID <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	// The floor under the installation: purging destroys the account
	// immediately, with no grace period to notice during.
	if err := r.guardLastSuperAdmin(ctx, actorID); err != nil {
		return err
	}

	// The actor's own node is left out. It is tombstoned, not deleted.
	const listContent = `
INSERT IGNORE INTO purge_work (actor_id, task, node_id)` + purgeContent + `
SELECT ?, ?, id
FROM content
WHERE id <> ?`

	if _, err := r.db.ExecContext(ctx, listContent,
		actorID, actorID,
		actorID, purgeTaskDelete,
		actorID,
	); err != nil {
		return err
	}

	// Everybody the actor follows loses a follower, and everybody following
	// the actor loses a leader.
	const listLeaders = `
INSERT IGNORE INTO purge_work (actor_id, task, node_id)
SELECT ?, ?, node_b_id
FROM edges
WHERE type = ?
  AND node_a_id = ?`

	const listFollowers = `
INSERT IGNORE INTO purge_work (actor_id, task, node_id)
SELECT ?, ?, node_a_id
FROM edges
WHERE type = ?
  AND node_b_id = ?`

	for _, query := range []string{listLeaders, listFollowers} {
		if _, err := r.db.ExecContext(ctx, query,
			actorID, purgeTaskCountFollows,
			constants.EdgeFollowEntityType,
			actorID,
		); err != nil {
			return err
		}
	}

	// What the actor liked or subscribed to loses a vote or a subscriber.
	// listLeaders again, with a different edge and a different counter.
	for task, edgeType := range map[string]string{
		purgeTaskCountVotes:       constants.EdgeBaseLikeEntityType,
		purgeTaskCountSubscribers: constants.EdgeBaseSubscriptionEntityType,
	} {
		if _, err := r.db.ExecContext(ctx, listLeaders,
			actorID, task,
			edgeType,
			actorID,
		); err != nil {
			return err
		}
	}

	// A post that stays, under which one of the actor's comments or reposts
	// is about to go. A parent that is itself going is listed too, and counting
	// it again later simply matches no row.
	const listParents = `
INSERT IGNORE INTO purge_work (actor_id, task, node_id)
SELECT ?, ?, child.parent_id
FROM purge_work work
JOIN nodes child
  ON child.id = work.node_id
WHERE work.actor_id = ?
  AND work.task = ?
  AND child.type = ?
  AND child.parent_id IS NOT NULL`

	for task, childType := range map[string]string{
		purgeTaskCountComments: constants.NodeBaseCommentEntityType,
		purgeTaskCountReposts:  constants.NodeBaseRepostType,
	} {
		if _, err := r.db.ExecContext(ctx, listParents,
			actorID, task,
			actorID, purgeTaskDelete,
			childType,
		); err != nil {
			return err
		}
	}

	return nil
}

// NextPurgeBatch returns the next nodes to remove for this actor, lowest id
// first, or nil when none are left.
func (r *nodesRepo) NextPurgeBatch(actorID int64, limit int) (*PurgeBatch, error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	// LEFT JOIN: a node already gone is still a row of work to clear.
	const query = `
SELECT
  work.node_id,
  COALESCE(node.type, ''),
  COALESCE(node.service, ''),
  COALESCE(node.owner_id, 0),
  COALESCE(node.filename, ''),
  COALESCE(node.cover_filename, '')
FROM purge_work work
LEFT JOIN nodes node
  ON node.id = work.node_id
WHERE work.actor_id = ?
  AND work.task = ?
ORDER BY work.node_id
LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, actorID, purgeTaskDelete, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var batch *PurgeBatch
	withFiles := map[int64]*entities.AnahitaNode{}
	for rows.Next() {
		node := entities.NewAnahitaNode()
		// The service and the owner as well as the names: a photo is stored
		// under the profile it is on, so without them its path cannot be
		// built and its images would be left in storage.
		if err := rows.Scan(
			&node.ID,
			&node.Type,
			&node.Service,
			&node.OwnerID,
			&node.Filename,
			&node.CoverFilename,
		); err != nil {
			return nil, err
		}

		if batch == nil {
			batch = &PurgeBatch{First: node.ID}
		}
		batch.Last = node.ID

		if node.Filename != "" || node.CoverFilename != "" {
			batch.Files = append(batch.Files, node)
			withFiles[node.ID] = node
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if batch == nil {
		return nil, nil
	}

	// A photo post can hold several images, and the node names only the
	// first.
	const photoFilesOfBatch = `
SELECT
  file.node_id,
  file.filename
FROM photo_files file
JOIN purge_work work
  ON work.node_id = file.node_id
WHERE work.actor_id = ?
  AND work.task = ?
  AND work.node_id BETWEEN ? AND ?
ORDER BY file.node_id, file.position`

	fileRows, err := r.db.QueryContext(ctx, photoFilesOfBatch,
		actorID, purgeTaskDelete, batch.First, batch.Last,
	)
	if err != nil {
		return nil, err
	}
	defer fileRows.Close()

	for fileRows.Next() {
		file := &entities.PhotoFile{}
		if err := fileRows.Scan(&file.NodeID, &file.Filename); err != nil {
			return nil, err
		}
		if node, found := withFiles[file.NodeID]; found {
			node.Files = append(node.Files, file)
		}
	}

	if err := fileRows.Err(); err != nil {
		return nil, err
	}

	return batch, nil
}

// DeletePurgeBatch removes one batch: the nodes, every edge touching them,
// every notification about them, and their rows in the list of work.
//
// One transaction, so a batch is either done or still listed.
//
// The notifications belong to notification-service, and this is the one place
// the graph deletes them. That service removes a notification when it hears
// that a post was deleted, but a purge publishes nothing per post — an account
// can hold thousands — and the bus may drop what is published. Left behind,
// they are rows pointing at nothing, in a schema with no foreign keys to
// notice.
func (r *nodesRepo) DeletePurgeBatch(actorID int64, batch *PurgeBatch) (err error) {
	if batch == nil {
		return nil
	}

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

	// Every statement picks its rows the same way: by joining to this batch
	// of the list. The last four placeholders of each are the batch.
	const deleteEdgesFrom = `
DELETE edge
FROM edges edge
JOIN purge_work work
  ON work.node_id = edge.node_a_id
WHERE work.actor_id = ?
  AND work.task = ?
  AND work.node_id BETWEEN ? AND ?`

	const deleteEdgesTo = `
DELETE edge
FROM edges edge
JOIN purge_work work
  ON work.node_id = edge.node_b_id
WHERE work.actor_id = ?
  AND work.task = ?
  AND work.node_id BETWEEN ? AND ?`

	const deleteNotificationsAboutObject = `
DELETE notification
FROM notifications notification
JOIN purge_work work
  ON work.node_id = notification.object_id
WHERE work.actor_id = ?
  AND work.task = ?
  AND work.node_id BETWEEN ? AND ?`

	const deleteNotificationsAboutTarget = `
DELETE notification
FROM notifications notification
JOIN purge_work work
  ON work.node_id = notification.target_id
WHERE work.actor_id = ?
  AND work.task = ?
  AND work.node_id BETWEEN ? AND ?`

	const deleteNotificationsAboutComment = `
DELETE notification
FROM notifications notification
JOIN purge_work work
  ON work.node_id = notification.comment_id
WHERE work.actor_id = ?
  AND work.task = ?
  AND work.node_id BETWEEN ? AND ?`

	const deletePhotoFiles = `
DELETE file
FROM photo_files file
JOIN purge_work work
  ON work.node_id = file.node_id
WHERE work.actor_id = ?
  AND work.task = ?
  AND work.node_id BETWEEN ? AND ?`

	const deleteNodes = `
DELETE node
FROM nodes node
JOIN purge_work work
  ON work.node_id = node.id
WHERE work.actor_id = ?
  AND work.task = ?
  AND work.node_id BETWEEN ? AND ?`

	// Last, because every statement above finds its rows through these.
	const deleteWork = `
DELETE FROM purge_work
WHERE actor_id = ?
  AND task = ?
  AND node_id BETWEEN ? AND ?`

	for _, query := range []string{
		deleteEdgesFrom,
		deleteEdgesTo,
		deleteNotificationsAboutObject,
		deleteNotificationsAboutTarget,
		deleteNotificationsAboutComment,
		deletePhotoFiles,
		deleteNodes,
		deleteWork,
	} {
		if _, err = tx.ExecContext(ctx, query,
			actorID, purgeTaskDelete, batch.First, batch.Last,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// FinishPurge does what is left once the actor's content is gone: removes
// every edge and notification that points at the actor itself, then counts
// again the counters that other people's nodes keep.
//
// Not one transaction. Each step is safe to repeat, and the list of counters
// is cleared only after they have all been counted.
func (r *nodesRepo) FinishPurge(actorID int64) error {
	if actorID <= 0 {
		return nil
	}

	// The actor's follows, admin edges, votes, blocks and subscriptions.
	const deleteEdgesFromActor = `
DELETE FROM edges
WHERE node_a_id = ?
LIMIT ?`

	const deleteEdgesToActor = `
DELETE FROM edges
WHERE node_b_id = ?
LIMIT ?`

	// What the actor was told, and what others were told about the actor.
	const deleteNotificationsToActor = `
DELETE FROM notifications
WHERE recipient_id = ?
LIMIT ?`

	const deleteNotificationsByActor = `
DELETE FROM notifications
WHERE subject_id = ?
LIMIT ?`

	const deleteNotificationsAboutActor = `
DELETE FROM notifications
WHERE object_id = ?
LIMIT ?`

	const deleteNotificationsInActor = `
DELETE FROM notifications
WHERE target_id = ?
LIMIT ?`

	for _, query := range []string{
		deleteEdgesFromActor,
		deleteEdgesToActor,
		deleteNotificationsToActor,
		deleteNotificationsByActor,
		deleteNotificationsAboutActor,
		deleteNotificationsInActor,
	} {
		if err := r.deleteInChunks(query, actorID); err != nil {
			return err
		}
	}

	if err := r.recountAfterPurge(actorID); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	const clearWork = `
DELETE FROM purge_work
WHERE actor_id = ?`

	_, err := r.db.ExecContext(ctx, clearWork, actorID)
	return err
}

// deleteInChunks runs a `DELETE … WHERE column = ? LIMIT ?` until it removes
// nothing more, so one account with a great many rows is never one long
// statement holding its locks.
func (r *nodesRepo) deleteInChunks(query string, id int64) error {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
		result, err := r.db.ExecContext(ctx, query, id, purgeChunk)
		cancel()
		if err != nil {
			return err
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected < purgeChunk {
			return nil
		}
	}
}

// recountAfterPurge sets the counters on the nodes PreparePurge listed to what
// the tables now say.
//
// Counted, never decremented: a step that runs twice still lands on the right
// number, and a counter that was already wrong comes out right.
func (r *nodesRepo) recountAfterPurge(actorID int64) error {
	const countFollows = `
UPDATE nodes actor
JOIN purge_work work
  ON work.node_id = actor.id
SET
  actor.follower_count = (
    SELECT COUNT(*)
    FROM edges
    WHERE type = ?
      AND node_b_id = actor.id
  ),
  actor.leader_count = (
    SELECT COUNT(*)
    FROM edges
    WHERE type = ?
      AND node_a_id = actor.id
  )
WHERE work.actor_id = ?
  AND work.task = ?`

	const countVotes = `
UPDATE nodes liked
JOIN purge_work work
  ON work.node_id = liked.id
SET liked.vote_up_count = (
  SELECT COUNT(*)
  FROM edges
  WHERE type = ?
    AND node_b_id = liked.id
)
WHERE work.actor_id = ?
  AND work.task = ?`

	const countSubscribers = `
UPDATE nodes subscribed
JOIN purge_work work
  ON work.node_id = subscribed.id
SET subscribed.subscriber_count = (
  SELECT COUNT(*)
  FROM edges
  WHERE type = ?
    AND node_b_id = subscribed.id
)
WHERE work.actor_id = ?
  AND work.task = ?`

	// Comments and reposts are nodes, and MySQL will not read the table an
	// UPDATE is writing from a subquery. The children are counted into a
	// derived table first and joined in; a parent with none left is absent
	// from it, which is what the COALESCE is for.
	const countComments = `
UPDATE nodes parent
JOIN purge_work work
  ON work.node_id = parent.id
LEFT JOIN (
  SELECT child.parent_id, COUNT(*) AS total
  FROM nodes child
  JOIN purge_work listed
    ON listed.node_id = child.parent_id
  WHERE listed.actor_id = ?
    AND listed.task = ?
    AND child.type = ?
  GROUP BY child.parent_id
) counted
  ON counted.parent_id = parent.id
SET parent.comment_count = COALESCE(counted.total, 0)
WHERE work.actor_id = ?
  AND work.task = ?`

	const countReposts = `
UPDATE nodes parent
JOIN purge_work work
  ON work.node_id = parent.id
LEFT JOIN (
  SELECT child.parent_id, COUNT(*) AS total
  FROM nodes child
  JOIN purge_work listed
    ON listed.node_id = child.parent_id
  WHERE listed.actor_id = ?
    AND listed.task = ?
    AND child.type = ?
  GROUP BY child.parent_id
) counted
  ON counted.parent_id = parent.id
SET parent.repost_count = COALESCE(counted.total, 0)
WHERE work.actor_id = ?
  AND work.task = ?`

	steps := []struct {
		query string
		args  []any
	}{
		{countFollows, []any{
			constants.EdgeFollowEntityType,
			constants.EdgeFollowEntityType,
			actorID, purgeTaskCountFollows,
		}},
		{countVotes, []any{
			constants.EdgeBaseLikeEntityType,
			actorID, purgeTaskCountVotes,
		}},
		{countSubscribers, []any{
			constants.EdgeBaseSubscriptionEntityType,
			actorID, purgeTaskCountSubscribers,
		}},
		{countComments, []any{
			actorID, purgeTaskCountComments, constants.NodeBaseCommentEntityType,
			actorID, purgeTaskCountComments,
		}},
		{countReposts, []any{
			actorID, purgeTaskCountReposts, constants.NodeBaseRepostType,
			actorID, purgeTaskCountReposts,
		}},
	}

	for _, step := range steps {
		ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
		_, err := r.db.ExecContext(ctx, step.query, step.args...)
		cancel()
		if err != nil {
			return err
		}
	}

	return nil
}

// IsPurged reports whether an actor has already been reduced to a tombstone.
//
// False for a node that does not exist, which a caller has already ruled out
// by reading it.
func (r *nodesRepo) IsPurged(id int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	const query = `
SELECT purged_at IS NOT NULL
FROM nodes
WHERE id = ?`

	var purged bool
	err := r.db.QueryRowContext(ctx, query, id).Scan(&purged)
	if err == sql.ErrNoRows {
		return false, nil
	}

	return purged, err
}
