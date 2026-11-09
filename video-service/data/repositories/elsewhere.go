package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	an_const "anahita-libs/constants"

	"video/constants"
)

// Videos and sounds from other networks, copied here the first time
// somebody plays them (roadmap rows 120 and 137,
// docs/plans/audio-and-video.md). One row for each post, table
// media_elsewhere.

// The states of a copy.
const (
	ElsewhereWaiting = "waiting"
	ElsewhereWorking = "working"
	ElsewhereReady   = "ready"
	ElsewhereFailed  = "failed"
	// Copied once, and removed after REMOTE_VIDEO_DAYS: played again, it is
	// copied again.
	ElsewhereRemoved = "removed"
)

// Copy is one post's file from elsewhere, and how its copying stands.
type Copy struct {
	NodeID   int64
	Kind     string
	Source   string
	Status   string
	Reason   string
	Attempts int
	PlayedAt time.Time
}

// ElsewhereInterface is the copies of files from elsewhere.
type ElsewhereInterface interface {
	// Ask is somebody pressing play: a copy is asked for when there is none,
	// or one that was removed or failed for a reason that may pass; one
	// waiting or made is left, and said to be played now. It answers with
	// the copy as it is.
	Ask(nodeID int64, kind, source string) (*Copy, error)
	// Read is how a post's copy stands, or nil.
	Read(nodeID int64) (*Copy, error)
	// Claim takes the next copy waiting, or nil when none is.
	Claim() (*Copy, error)
	// Finish says how a copy ended: ready, or failed with why.
	Finish(nodeID int64, status, reason string) error
	// Unplayed is the copies made and not played since `before`, to remove.
	Unplayed(before time.Time, limit int) ([]*Copy, error)
}

type ElsewhereRepo struct {
	db       *sql.DB
	timeout  time.Duration
	now      func() time.Time
	ClaimFor time.Duration
}

func NewElsewhere(db *sql.DB) *ElsewhereRepo {
	return &ElsewhereRepo{db: db, timeout: an_const.DBTimeout, now: func() time.Time { return time.Now().UTC() }, ClaimFor: 30 * time.Minute}
}

func (r *ElsewhereRepo) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), r.timeout)
}

// A failure for a reason that may pass is tried again when asked again, as
// is a copy whose worker stopped on its last try; a file too large or too
// long, or not a video, is not. (3 is constants.MaxAttempts.)
const elsewhereAsk = `
INSERT INTO media_elsewhere (node_id, kind, source, status, played_at, created_at, updated_at)
VALUES (?, ?, ?, 'waiting', ?, ?, ?)
ON DUPLICATE KEY UPDATE
    played_at  = VALUES(played_at),
    attempts   = IF(status = 'removed' OR (status = 'failed' AND reason IN ('failed', 'gone')) OR (status = 'working' AND attempts >= 3), 0, attempts),
    reason     = IF(status = 'removed' OR (status = 'failed' AND reason IN ('failed', 'gone')) OR (status = 'working' AND attempts >= 3), '', reason),
    kind       = IF(status = 'removed' OR (status = 'failed' AND reason IN ('failed', 'gone')) OR (status = 'working' AND attempts >= 3), VALUES(kind), kind),
    source     = IF(status = 'removed' OR (status = 'failed' AND reason IN ('failed', 'gone')) OR (status = 'working' AND attempts >= 3), VALUES(source), source),
    updated_at = IF(status = 'removed' OR (status = 'failed' AND reason IN ('failed', 'gone')) OR (status = 'working' AND attempts >= 3), VALUES(updated_at), updated_at),
    status     = IF(status = 'removed' OR (status = 'failed' AND reason IN ('failed', 'gone')) OR (status = 'working' AND attempts >= 3), 'waiting', status)`

func (r *ElsewhereRepo) Ask(nodeID int64, kind, source string) (*Copy, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	now := r.now()
	if _, err := r.db.ExecContext(ctx, elsewhereAsk, nodeID, kind, source, now, now, now); err != nil {
		return nil, err
	}
	return r.Read(nodeID)
}

const elsewhereRead = `
SELECT node_id, kind, source, status, reason, attempts, played_at
FROM media_elsewhere
WHERE node_id = ?`

func scanCopy(row interface{ Scan(...any) error }) (*Copy, error) {
	c := &Copy{}
	err := row.Scan(&c.NodeID, &c.Kind, &c.Source, &c.Status, &c.Reason, &c.Attempts, &c.PlayedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *ElsewhereRepo) Read(nodeID int64) (*Copy, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	return scanCopy(r.db.QueryRowContext(ctx, elsewhereRead, nodeID))
}

// Waiting, or taken by a worker that has not been heard of for a while.
const elsewhereNext = `
SELECT node_id
FROM media_elsewhere
WHERE (status = 'waiting' OR (status = 'working' AND claimed_at < ?))
  AND attempts < ?
ORDER BY updated_at
LIMIT 1
FOR UPDATE SKIP LOCKED`

const elsewhereClaim = `
UPDATE media_elsewhere
SET status     = 'working',
    claimed_at = ?,
    attempts   = attempts + 1,
    updated_at = ?
WHERE node_id = ?`

func (r *ElsewhereRepo) Claim() (*Copy, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	now := r.now()
	var id int64
	err = tx.QueryRowContext(ctx, elsewhereNext, now.Add(-r.ClaimFor), constants.MaxAttempts).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, elsewhereClaim, now, now, id); err != nil {
		return nil, err
	}
	c, err := scanCopy(tx.QueryRowContext(ctx, elsewhereRead, id))
	if err != nil {
		return nil, err
	}
	return c, tx.Commit()
}

const elsewhereFinish = `
UPDATE media_elsewhere
SET status     = ?,
    reason     = ?,
    claimed_at = NULL,
    updated_at = ?
WHERE node_id = ?`

func (r *ElsewhereRepo) Finish(nodeID int64, status, reason string) error {
	ctx, cancel := r.ctx()
	defer cancel()
	_, err := r.db.ExecContext(ctx, elsewhereFinish, status, reason, r.now(), nodeID)
	return err
}

const elsewhereUnplayed = `
SELECT node_id, kind, source, status, reason, attempts, played_at
FROM media_elsewhere
WHERE status = 'ready'
  AND played_at < ?
ORDER BY played_at
LIMIT ?`

func (r *ElsewhereRepo) Unplayed(before time.Time, limit int) ([]*Copy, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	rows, err := r.db.QueryContext(ctx, elsewhereUnplayed, before.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var copies []*Copy
	for rows.Next() {
		c, err := scanCopy(rows)
		if err != nil {
			return nil, err
		}
		copies = append(copies, c)
	}
	return copies, rows.Err()
}
