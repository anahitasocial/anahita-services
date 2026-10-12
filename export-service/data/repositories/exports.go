// Package repositories keeps the exports: each request for a person's copy
// of their data, from when it is asked for until its archive is deleted.
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	an_const "anahita-libs/constants"

	"github.com/go-sql-driver/mysql"
)

// The states an export goes through.
const (
	StatusPending   = "pending"
	StatusBuilding  = "building"
	StatusReady     = "ready"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
	StatusExpired   = "expired"
)

// How many times a worker tries an export before giving up on it.
const MaxAttempts = 3

// Export is one row of data_exports.
type Export struct {
	ID         string
	PersonID   int64
	Status     string
	PackageKey string
	SizeBytes  int64
	Error      string
	Attempts   int
	IP         string
	UserAgent  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ReadyAt    *time.Time
	ExpiresAt  *time.Time
}

var (
	// ErrNotInState is a change refused because the export has moved on.
	ErrNotInState = errors.New("the export is no longer in that state")
	// ErrActive is a second export asked for while one is being made.
	ErrActive = errors.New("an export is already being made")
)

const exportColumns = `
    id,
    person_id,
    status,
    package_key,
    size_bytes,
    error,
    attempts,
    ip,
    user_agent,
    created_at,
    updated_at,
    ready_at,
    expires_at`

const exportAdd = `
INSERT INTO data_exports (
    id,
    person_id,
    status,
    ip,
    user_agent,
    created_at,
    updated_at
)
VALUES (?, ?, 'pending', ?, ?, ?, ?)`

const exportRead = `
SELECT` + exportColumns + `
FROM data_exports
WHERE id = ?`

const exportsOfPerson = `
SELECT` + exportColumns + `
FROM data_exports
WHERE person_id = ?
ORDER BY created_at DESC
LIMIT 10`

// When somebody last had an export made that came to something: asked for
// and not given up or failed. A failure does not cost them the wait.
const exportLastMade = `
SELECT MAX(created_at)
FROM data_exports
WHERE person_id = ?
  AND status IN ('pending', 'building', 'ready', 'expired')`

const exportNextWaiting = `
SELECT id
FROM data_exports
WHERE attempts < ?
  AND (
      status = 'pending'
      OR (status = 'building' AND claimed_at < ?)
  )
ORDER BY created_at
LIMIT 1
FOR UPDATE SKIP LOCKED`

const exportClaim = `
UPDATE data_exports
SET status = 'building',
    claimed_at = ?,
    attempts = attempts + 1,
    updated_at = ?
WHERE id = ?`

const exportToReady = `
UPDATE data_exports
SET status = 'ready',
    package_key = ?,
    size_bytes = ?,
    ready_at = ?,
    expires_at = ?,
    claimed_at = NULL,
    updated_at = ?
WHERE id = ?
  AND status = 'building'`

const exportToFailed = `
UPDATE data_exports
SET status = 'failed',
    error = ?,
    claimed_at = NULL,
    updated_at = ?
WHERE id = ?
  AND status IN ('pending', 'building')`

// Let go of by a worker that could not finish, for another try.
const exportRelease = `
UPDATE data_exports
SET status = 'pending',
    claimed_at = NULL,
    updated_at = ?
WHERE id = ?
  AND status = 'building'`

const exportToCancelled = `
UPDATE data_exports
SET status = 'cancelled',
    updated_at = ?
WHERE id = ?
  AND status IN ('pending', 'building', 'ready', 'failed')`

const exportToExpired = `
UPDATE data_exports
SET status = 'expired',
    updated_at = ?
WHERE id = ?
  AND status = 'ready'`

const exportsPastKeeping = `
SELECT` + exportColumns + `
FROM data_exports
WHERE status = 'ready'
  AND expires_at < ?
LIMIT 50`

const exportsWornOut = `
SELECT id
FROM data_exports
WHERE status IN ('pending', 'building')
  AND attempts >= ?
  AND (claimed_at IS NULL OR claimed_at < ?)
LIMIT 20`

type ExportsInterface interface {
	Add(export *Export) error
	Read(id string) (*Export, error)
	OfPerson(personID int64) ([]*Export, error)
	// LastMade is when somebody last asked for an export that was not
	// given up or failed; the zero time when never.
	LastMade(personID int64) (time.Time, error)
	// Claim takes the next export waiting, or returns nil when none is.
	Claim() (*Export, error)
	ToReady(id, key string, size int64, keepFor time.Duration) error
	ToFailed(id, reason string) error
	Release(id string) error
	ToCancelled(id string) error
	ToExpired(id string) error
	PastKeeping() ([]*Export, error)
	WornOut() ([]string, error)
}

type ExportsRepo struct {
	db      *sql.DB
	timeout time.Duration
	now     func() time.Time
	// How long a worker may hold an export before another takes it over.
	ClaimFor time.Duration
}

func NewExports(db *sql.DB) *ExportsRepo {
	return &ExportsRepo{
		db:       db,
		timeout:  an_const.DBTimeout,
		now:      func() time.Time { return time.Now().UTC() },
		ClaimFor: 2 * time.Hour,
	}
}

func (r *ExportsRepo) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), r.timeout)
}

// Add records a request. ErrActive when the person has one being made: the
// table allows one at a time, whoever asks first.
func (r *ExportsRepo) Add(export *Export) error {
	ctx, cancel := r.ctx()
	defer cancel()

	now := r.now()
	export.Status = StatusPending
	export.CreatedAt, export.UpdatedAt = now, now

	_, err := r.db.ExecContext(ctx, exportAdd, export.ID, export.PersonID, export.IP, export.UserAgent, now, now)

	var duplicate *mysql.MySQLError
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return ErrActive
	}
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanExport(row scanner) (*Export, error) {
	e := &Export{}
	var ready, expires sql.NullTime
	err := row.Scan(&e.ID, &e.PersonID, &e.Status, &e.PackageKey, &e.SizeBytes, &e.Error,
		&e.Attempts, &e.IP, &e.UserAgent, &e.CreatedAt, &e.UpdatedAt, &ready, &expires)
	if err != nil {
		return nil, err
	}
	if ready.Valid {
		e.ReadyAt = &ready.Time
	}
	if expires.Valid {
		e.ExpiresAt = &expires.Time
	}
	return e, nil
}

func (r *ExportsRepo) Read(id string) (*Export, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	export, err := scanExport(r.db.QueryRowContext(ctx, exportRead, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return export, err
}

func (r *ExportsRepo) list(query string, args ...any) ([]*Export, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exports []*Export
	for rows.Next() {
		export, err := scanExport(rows)
		if err != nil {
			return nil, err
		}
		exports = append(exports, export)
	}
	return exports, rows.Err()
}

func (r *ExportsRepo) OfPerson(personID int64) ([]*Export, error) {
	return r.list(exportsOfPerson, personID)
}

func (r *ExportsRepo) PastKeeping() ([]*Export, error) {
	return r.list(exportsPastKeeping, r.now())
}

func (r *ExportsRepo) LastMade(personID int64) (time.Time, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	var last sql.NullTime
	if err := r.db.QueryRowContext(ctx, exportLastMade, personID).Scan(&last); err != nil {
		return time.Time{}, err
	}
	return last.Time, nil
}

func (r *ExportsRepo) change(query string, args ...any) error {
	ctx, cancel := r.ctx()
	defer cancel()

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return err
	} else if changed == 0 {
		return ErrNotInState
	}
	return nil
}

func (r *ExportsRepo) ToReady(id, key string, size int64, keepFor time.Duration) error {
	now := r.now()
	return r.change(exportToReady, key, size, now, now.Add(keepFor), now, id)
}

func (r *ExportsRepo) ToFailed(id, reason string) error {
	return r.change(exportToFailed, reason, r.now(), id)
}

func (r *ExportsRepo) Release(id string) error {
	return r.change(exportRelease, r.now(), id)
}

func (r *ExportsRepo) ToCancelled(id string) error {
	return r.change(exportToCancelled, r.now(), id)
}

func (r *ExportsRepo) ToExpired(id string) error {
	return r.change(exportToExpired, r.now(), id)
}

func (r *ExportsRepo) Claim() (*Export, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	now := r.now()

	var id string
	err = tx.QueryRowContext(ctx, exportNextWaiting, MaxAttempts, now.Add(-r.ClaimFor)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, exportClaim, now, now, id); err != nil {
		return nil, err
	}
	export, err := scanExport(tx.QueryRowContext(ctx, exportRead, id))
	if err != nil {
		return nil, err
	}
	return export, tx.Commit()
}

func (r *ExportsRepo) WornOut() ([]string, error) {
	ctx, cancel := r.ctx()
	defer cancel()

	rows, err := r.db.QueryContext(ctx, exportsWornOut, MaxAttempts, r.now().Add(-r.ClaimFor))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
