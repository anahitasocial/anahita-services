package repositories

import (
	"context"
	"database/sql"
	"time"

	an_const "anahita-libs/constants"
)

// The most dismissals listed back to somebody, newest first.
const DismissalsListed = 100

const dismissalAdd = `
INSERT INTO recommendation_dismissals
    (person_id, actor_id, created_at)
VALUES
    (?, ?, ?)
ON DUPLICATE KEY UPDATE
    created_at = created_at`

const dismissalRemove = `
DELETE FROM recommendation_dismissals
WHERE person_id = ?
  AND actor_id = ?`

const dismissalsOf = `
SELECT actor_id
FROM recommendation_dismissals
WHERE person_id = ?
ORDER BY created_at DESC, actor_id DESC
LIMIT ?`

// Everything about an actor that is gone: what they turned down, and their
// being turned down by others.
const dismissalsForget = `
DELETE FROM recommendation_dismissals
WHERE person_id = ?
   OR actor_id = ?`

type DismissalsInterface interface {
	Add(personID, actorID int64) error
	Remove(personID, actorID int64) error
	// List returns the profiles somebody turned down, most lately first.
	List(personID int64) ([]int64, error)
	Forget(actorID int64) error
}

type DismissalsRepo struct {
	db      *sql.DB
	timeout time.Duration
}

func NewDismissals(db *sql.DB) DismissalsInterface {
	return &DismissalsRepo{db: db, timeout: an_const.DBTimeout}
}

func (r *DismissalsRepo) Add(personID, actorID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, dismissalAdd, personID, actorID, time.Now().UTC())

	return err
}

func (r *DismissalsRepo) Remove(personID, actorID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, dismissalRemove, personID, actorID)

	return err
}

func (r *DismissalsRepo) List(personID int64) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, dismissalsOf, personID, DismissalsListed)
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

func (r *DismissalsRepo) Forget(actorID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, dismissalsForget, actorID, actorID)

	return err
}
