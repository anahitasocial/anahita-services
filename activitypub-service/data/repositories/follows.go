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
	// Begin keeps a follow that is not kept yet, and says whether it did.
	// Two copies of the service hear of the same follow; the one that is
	// told yes here is the one that sends it.
	Begin(follow *Follow) (bool, error)
	// Read is the follow between two people one way round, or nil.
	Read(localID, remoteID int64, direction string) (*Follow, error)
	// ReadByActivity is the follow that was called this when it was sent.
	ReadByActivity(activityID string) (*Follow, error)
	// FollowerInboxes is where to send what a person here posts: one
	// address for each server that has a follower of theirs, the one that
	// takes for everybody on that server where there is one.
	FollowerInboxes(localID int64) ([]string, error)
	// IsFollowedFromHere says whether anybody here follows somebody
	// elsewhere, or has asked to.
	IsFollowedFromHere(remoteID int64) (bool, error)
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

const beginFollow = `
INSERT IGNORE INTO activitypub_follows (local_id, remote_id, direction, activity_id, state, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`

func (repo *Follows) Begin(follow *Follow) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	now := time.Now().UTC()
	result, err := repo.db.ExecContext(ctx, beginFollow,
		follow.LocalID, follow.RemoteID, follow.Direction, follow.ActivityID, follow.State, now, now)
	if err != nil {
		return false, err
	}
	added, err := result.RowsAffected()
	return added == 1, err
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

const readFollowByActivity = `
SELECT local_id, remote_id, direction, activity_id, state
FROM activitypub_follows
WHERE activity_id = ?
LIMIT 1`

func (repo *Follows) ReadByActivity(activityID string) (*Follow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	follow := &Follow{}
	err := repo.db.QueryRowContext(ctx, readFollowByActivity, activityID).Scan(
		&follow.LocalID, &follow.RemoteID, &follow.Direction, &follow.ActivityID, &follow.State)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return follow, nil
}

const followerInboxes = `
SELECT DISTINCT IF(actor.shared_inbox <> '', actor.shared_inbox, actor.inbox)
FROM activitypub_follows AS follow
JOIN activitypub_actors AS actor ON actor.node_id = follow.remote_id
WHERE follow.local_id = ?
  AND follow.direction = 'in'
  AND follow.state = 'accepted'`

func (repo *Follows) FollowerInboxes(localID int64) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	rows, err := repo.db.QueryContext(ctx, followerInboxes, localID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var inboxes []string
	for rows.Next() {
		var inbox string
		if err := rows.Scan(&inbox); err != nil {
			return nil, err
		}
		inboxes = append(inboxes, inbox)
	}
	return inboxes, rows.Err()
}

const isFollowedFromHere = `
SELECT EXISTS (
    SELECT 1
    FROM activitypub_follows
    WHERE remote_id = ?
      AND direction = 'out'
)`

func (repo *Follows) IsFollowedFromHere(remoteID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var followed bool
	err := repo.db.QueryRowContext(ctx, isFollowedFromHere, remoteID).Scan(&followed)
	return followed, err
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
