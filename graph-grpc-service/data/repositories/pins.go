package repositories

import (
	"context"
	"database/sql"
	"errors"

	"anahita-libs/constants"
)

// PinsRepoInterface is pinning a post to the profile it is on.
//
// A pinned post is an ordinary post with a flag and a date. It leads the
// profile's lists; who may see it does not change.
type PinsRepoInterface interface {
	// Pin pins a post. Pinning one that is pinned already changes nothing.
	// A profile that already has as many pins as it can gives up the
	// oldest of them: the pin moves.
	Pin(id int64, ownerID int64, by int64) error

	// Unpin takes the pin off.
	Unpin(id int64, by int64) error

	// PinnedIDs returns the posts pinned to a profile, most recently
	// pinned first.
	PinnedIDs(ownerID int64) ([]int64, error)
}

func NewPinsRepo(db *sql.DB) PinsRepoInterface {
	return &pinsRepo{db: db}
}

type pinsRepo struct {
	db *sql.DB
}

const selectPinned = `
SELECT id
FROM nodes
WHERE owner_id = ?
  AND pinned = 1
  AND deleted_at IS NULL
ORDER BY pinned_at DESC, id DESC`

func (r *pinsRepo) PinnedIDs(ownerID int64) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, selectPinned, ownerID)
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

// The count and the pin are one transaction, with the profile's pinned rows
// locked while it runs, so two posts pinned at the same moment cannot both
// take the last place. Oldest pin first, which is the one that gives way.
const lockPinned = `
SELECT id
FROM nodes
WHERE owner_id = ?
  AND pinned = 1
  AND deleted_at IS NULL
ORDER BY pinned_at ASC, id ASC
FOR UPDATE`

const selectPinnable = `
SELECT 1
FROM nodes
WHERE id = ?
  AND owner_id = ?
  AND root_id IS NULL
  AND deleted_at IS NULL
FOR UPDATE`

// Only a post at the top level: a reply is not pinned.
const pinNode = `
UPDATE nodes
SET pinned = 1,
    pinned_at = UTC_TIMESTAMP(),
    modified_by = ?,
    updated_at = UTC_TIMESTAMP()
WHERE id = ?
  AND owner_id = ?
  AND root_id IS NULL`

func (r *pinsRepo) Pin(id int64, ownerID int64, by int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Whether this is something that can be pinned here at all, asked
	// before anything gives way to it: a post at the top level, on this
	// profile, still there. Asking for anything else changes nothing.
	var pinnable int
	if err := tx.QueryRowContext(ctx, selectPinnable, id, ownerID).Scan(&pinnable); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	rows, err := tx.QueryContext(ctx, lockPinned, ownerID)
	if err != nil {
		return err
	}

	pinned := []int64{}
	for rows.Next() {
		var pinnedID int64
		if err := rows.Scan(&pinnedID); err != nil {
			rows.Close()
			return err
		}
		pinned = append(pinned, pinnedID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, pinnedID := range pinned {
		if pinnedID == id {
			return tx.Commit()
		}
	}

	// Room is made for it: the oldest pins give way until there is a place.
	for len(pinned) >= constants.MaxPinnedPosts && len(pinned) > 0 {
		if _, err := tx.ExecContext(ctx, unpinNode, by, pinned[0]); err != nil {
			return err
		}
		pinned = pinned[1:]
	}

	if _, err := tx.ExecContext(ctx, pinNode, by, id, ownerID); err != nil {
		return err
	}

	return tx.Commit()
}

const unpinNode = `
UPDATE nodes
SET pinned = 0,
    pinned_at = NULL,
    modified_by = ?,
    updated_at = UTC_TIMESTAMP()
WHERE id = ?`

func (r *pinsRepo) Unpin(id int64, by int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, unpinNode, by, id)
	return err
}
