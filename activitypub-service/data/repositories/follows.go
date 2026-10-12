package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const (
	// The person elsewhere follows the one here.
	FollowIn = "in"
	// The person here follows the one elsewhere.
	FollowOut = "out"

	FollowPending  = "pending"
	FollowAccepted = "accepted"
)

// Follow is a follow between somebody here and somebody on another server,
// as the other server knows it.
type Follow struct {
	LocalID    int64
	RemoteID   int64
	Direction  string
	ActivityID string
	State      string
}

// FollowsInterface is the follows that cross servers.
type FollowsInterface interface {
	// Save keeps a follow, in place of what was kept for the same two
	// people the same way round.
	Save(follow *Follow) error
	// Read is the follow between two people one way round, or nil.
	Read(localID, remoteID int64, direction string) (*Follow, error)
	// Delete forgets it.
	Delete(localID, remoteID int64, direction string) error
}

type Follows struct {
	db *sql.DB
}

func NewFollows(db *sql.DB) *Follows {
	return &Follows{db: db}
}

const saveFollow = `
INSERT INTO activitypub_follows (local_id, remote_id, direction, activity_id, state, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    activity_id = VALUES(activity_id),
    state       = VALUES(state),
    updated_at  = VALUES(updated_at)`

func (repo *Follows) Save(follow *Follow) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	now := time.Now().UTC()
	_, err := repo.db.ExecContext(ctx, saveFollow,
		follow.LocalID, follow.RemoteID, follow.Direction, follow.ActivityID, follow.State, now, now)
	return err
}

const readFollow = `
SELECT local_id, remote_id, direction, activity_id, state
FROM activitypub_follows
WHERE local_id = ?
  AND remote_id = ?
  AND direction = ?`

func (repo *Follows) Read(localID, remoteID int64, direction string) (*Follow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	follow := &Follow{}
	err := repo.db.QueryRowContext(ctx, readFollow, localID, remoteID, direction).Scan(
		&follow.LocalID, &follow.RemoteID, &follow.Direction, &follow.ActivityID, &follow.State)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return follow, nil
}

const deleteFollow = `
DELETE FROM activitypub_follows
WHERE local_id = ?
  AND remote_id = ?
  AND direction = ?`

func (repo *Follows) Delete(localID, remoteID int64, direction string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, deleteFollow, localID, remoteID, direction)
	return err
}
