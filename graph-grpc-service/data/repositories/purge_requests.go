package repositories

import (
	"context"
	"database/sql"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// The queue of accounts a super administrator has asked to be removed now.
//
// Asking and purging are separate. A request hides the account and adds a row
// here; whoever holds the sweeper's lock then works through the rows. So a
// request for hundreds of accounts answers at once, and the rows are its
// progress.

const (
	PurgeRequestQueued = "queued"
	PurgeRequestDone   = "done"
	PurgeRequestFailed = "failed"

	// PurgeRequestMaxAttempts is how many times a purge is tried before the
	// request is marked failed. A purge that is merely interrupted carries on
	// from where it stopped, so this is only reached by one that cannot work.
	PurgeRequestMaxAttempts = 3

	purgeErrorMaxLength = 255
)

// PurgeRequest is one account in the queue.
type PurgeRequest struct {
	ID          int64
	BatchID     string
	ActorID     int64
	ActorAlias  string
	ActorType   string
	Status      string
	Attempts    int
	Error       string
	RequestedBy int64
}

type PurgeRequestsRepoInterface interface {
	// Add queues an actor under a batch, and reports whether it did. False
	// means the actor is already queued or already purged; a request that
	// failed before is replaced.
	Add(batchID string, actor *entities.AnahitaNode, requestedBy int64) (bool, error)

	// NextQueued returns the oldest requests still waiting.
	NextQueued(limit int) ([]*PurgeRequest, error)

	MarkDone(id int64) error

	// MarkAttemptFailed records a failed attempt. The request stays queued
	// until it has failed PurgeRequestMaxAttempts times.
	MarkAttemptFailed(request *PurgeRequest, reason string) error

	// BrowseBatch returns every request of one batch, in the order asked.
	BrowseBatch(batchID string) ([]*PurgeRequest, error)
}

func NewPurgeRequestsRepo(db *sql.DB) PurgeRequestsRepoInterface {
	return &purgeRequestsRepo{db: db, dbTimeout: constants.DBTimeout}
}

type purgeRequestsRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
}

const purgeRequestColumns = `
  id,
  batch_id,
  actor_id,
  actor_alias,
  actor_type,
  status,
  attempts,
  error,
  requested_by`

func (r *purgeRequestsRepo) Add(batchID string, actor *entities.AnahitaNode, requestedBy int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	// Asking again is how a failed request is retried.
	const clearFailed = `
DELETE FROM purge_requests
WHERE actor_id = ?
  AND status = ?`

	if _, err := r.db.ExecContext(ctx, clearFailed, actor.ID, PurgeRequestFailed); err != nil {
		return false, err
	}

	// IGNORE: actor_id is unique, so an actor already queued or done is left
	// as it is and no row is added.
	const insert = `
INSERT IGNORE INTO purge_requests (
  batch_id,
  actor_id,
  actor_alias,
  actor_type,
  status,
  requested_by,
  requested_at
)
VALUES (?, ?, ?, ?, ?, ?, ?)`

	result, err := r.db.ExecContext(ctx, insert,
		batchID,
		actor.ID, actor.Alias, actor.Type,
		PurgeRequestQueued,
		requestedBy, time.Now().UTC(),
	)
	if err != nil {
		return false, err
	}

	added, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return added > 0, nil
}

func (r *purgeRequestsRepo) NextQueued(limit int) ([]*PurgeRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	const query = `
SELECT` + purgeRequestColumns + `
FROM purge_requests
WHERE status = ?
ORDER BY requested_at, id
LIMIT ?`

	return r.browse(ctx, query, PurgeRequestQueued, limit)
}

func (r *purgeRequestsRepo) BrowseBatch(batchID string) ([]*PurgeRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	const query = `
SELECT` + purgeRequestColumns + `
FROM purge_requests
WHERE batch_id = ?
ORDER BY id`

	return r.browse(ctx, query, batchID)
}

func (r *purgeRequestsRepo) browse(ctx context.Context, query string, args ...any) ([]*PurgeRequest, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []*PurgeRequest
	for rows.Next() {
		request := &PurgeRequest{}
		if err := rows.Scan(
			&request.ID,
			&request.BatchID,
			&request.ActorID,
			&request.ActorAlias,
			&request.ActorType,
			&request.Status,
			&request.Attempts,
			&request.Error,
			&request.RequestedBy,
		); err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}

	return requests, rows.Err()
}

func (r *purgeRequestsRepo) MarkDone(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	const query = `
UPDATE purge_requests
SET
  status = ?,
  error = '',
  finished_at = ?
WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, PurgeRequestDone, time.Now().UTC(), id)
	return err
}

func (r *purgeRequestsRepo) MarkAttemptFailed(request *PurgeRequest, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if len(reason) > purgeErrorMaxLength {
		reason = reason[:purgeErrorMaxLength]
	}

	request.Attempts++
	request.Error = reason

	// Decided here rather than in the statement, so the row and the struct the
	// caller holds cannot disagree about whether it is still queued.
	var finishedAt any
	if request.Attempts >= PurgeRequestMaxAttempts {
		request.Status = PurgeRequestFailed
		finishedAt = time.Now().UTC()
	}

	const query = `
UPDATE purge_requests
SET
  status = ?,
  attempts = ?,
  error = ?,
  finished_at = ?
WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query,
		request.Status, request.Attempts, request.Error, finishedAt, request.ID)
	return err
}
