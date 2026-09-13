package repositories

import (
	"context"
	"time"
)

// Soft deletion for actors.
//
// Deleting a profile used to be a hard DELETE of the actor node, everything it
// owned, every descendant of those by parent_id, and every edge touching any of
// them — in a schema with no foreign keys, no audit record and no way back. A
// misclick was permanent, and support had nothing to offer somebody who
// regretted it.
//
// It is now two steps separated by a grace period:
//
//	SoftDeleteActor  — marks and disables, destroys nothing
//	PurgeByOwner     — the old behaviour, run by the sweeper afterwards
//
// Only the actor node is touched here. Its content stays exactly where it is,
// which is what lets RestoreActor bring the profile back whole rather than
// bringing back an empty shell.

// SoftDeleteActor marks an actor deleted and disables it.
//
// `enabled` is cleared alongside `deleted_at` because that flag is already
// enforced at every login path in auth-service — password, TOTP and passkey —
// so clearing it stops the account resolving immediately without any of those
// paths needing to learn about deletion.
//
// Idempotent by way of the `deleted_at IS NULL` guard: deleting twice does not
// move the clock forward and quietly extend the grace period.
func (r *nodesRepo) SoftDeleteActor(id int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	// The floor under the installation: deleting the last super administrator leaves nobody who can administer the site once the grace period expires and the sweeper runs.
	if err := r.guardLastSuperAdmin(ctx, id); err != nil {
		return false, err
	}

	if id <= 0 {
		return false, nil
	}

	const query = `
UPDATE nodes
SET deleted_at = ?, enabled = 0
WHERE id = ? AND deleted_at IS NULL;
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

// RestoreActor reverses a soft delete.
//
// Restores `enabled` as well, since SoftDeleteActor cleared it. That does mean
// restoring an account an administrator had separately disabled would re-enable
// it — worth knowing, but the alternative is remembering the prior value, and
// an account that comes back still locked out looks exactly like a restore that
// failed.
//
// Returns false when there was nothing to restore, so a caller can tell "the
// grace period had already run out" from "restored".
func (r *nodesRepo) RestoreActor(id int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if id <= 0 {
		return false, nil
	}

	const query = `
UPDATE nodes
SET deleted_at = NULL, enabled = 1
WHERE id = ? AND deleted_at IS NOT NULL;
`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// BrowseDeletedBefore returns the ids of actors deleted before the cutoff.
//
// The sweeper's queue. Ordered oldest first so a backlog drains in the order it
// accumulated, and limited so one run cannot take the database down with it.
func (r *nodesRepo) BrowseDeletedBefore(cutoff time.Time, limit uint64) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	const query = `
SELECT id
FROM nodes
WHERE deleted_at IS NOT NULL AND deleted_at < ?
ORDER BY deleted_at ASC
LIMIT ?;
`

	rows, err := r.db.QueryContext(ctx, query, cutoff.UTC(), limit)
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}
