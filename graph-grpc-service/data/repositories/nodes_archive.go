package repositories

import (
	"context"
	"time"
)

// The two reversible-versus-not lifecycle transitions, kept apart from
// deletion because they mean different things.
//
//	SetEnabled     — the reversible switch. Suspension, or unpublishing.
//	ArchiveActor   — permanent. Nothing is destroyed; it stops being pushed.
//
// Archive is deliberately one-way. There is no UnarchiveActor here and that is
// the design, not an omission: the plan settled that archiving is for an actor
// that is FINISHED — a group whose project ended, a person who has left or
// died — and an undo would make it the soft option people reach for by
// mistake. A super administrator who must reverse one has the database; that
// friction is the point.

// ArchiveActor marks an actor archived.
//
// Idempotent by way of the `archived_at IS NULL` guard, so archiving twice does
// not move the timestamp and rewrite when it happened.
//
// Refuses a deleted actor. The two states answer different questions and an
// actor in both is a contradiction — deleted means "erased in thirty days",
// archived means "kept forever". Whichever was applied second would be a lie.
func (r *nodesRepo) ArchiveActor(id int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if id <= 0 {
		return false, nil
	}

	const query = `
UPDATE nodes
SET archived_at = ?
WHERE id = ? AND archived_at IS NULL AND deleted_at IS NULL;
`

	result, err := r.db.ExecContext(ctx, query, time.Now().UTC(), id)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// SetEnabled turns an actor on or off. Reversible, and the only thing that
// should ever write this column on its own.
//
// `enabled` was doing three jobs — suspension, deletion and "unpublished" —
// which is how a soft-deleted actor became indistinguishable from a
// deliberately suspended one. Deletion still clears it, because every login
// path already enforces it and that is what makes a deleted account stop
// resolving immediately; but disabling is now its own action rather than
// something you get by half-deleting.
//
// Refuses a deleted actor, so re-enabling one cannot be used to resurrect it
// around the restore flow — that path re-reads the grace period and this does
// not.
//
// Returns false when nothing changed: already in that state, deleted, or
// absent. The caller needs to tell that from success.
func (r *nodesRepo) SetEnabled(id int64, enabled bool) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if id <= 0 {
		return false, nil
	}

	const query = `
UPDATE nodes
SET enabled = ?
WHERE id = ? AND enabled <> ? AND deleted_at IS NULL;
`

	result, err := r.db.ExecContext(ctx, query, enabled, id, enabled)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}
