package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Which post each repost heard from Bluesky was of.
//
// A repost that is taken back is heard as its name alone, with nothing
// said of what it was a repost of. So what it was of is kept beside its
// name when it is made.

type RepostsInterface interface {
	// Save keeps what a repost was of.
	Save(actorID int64, rkey string, parentID int64) error
	// Take answers with the post a repost was of, and forgets it. 0 when
	// nothing is known of it.
	Take(actorID int64, rkey string) (int64, error)
}

type Reposts struct {
	db *sql.DB
}

func NewReposts(db *sql.DB) *Reposts {
	return &Reposts{db: db}
}

const saveRepost = `
INSERT INTO atproto_reposts (actor_id, rkey, parent_id, created_at)
VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    parent_id = VALUES(parent_id)`

func (repo *Reposts) Save(actorID int64, rkey string, parentID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, saveRepost, actorID, rkey, parentID, time.Now().UTC())
	return err
}

const readRepost = `
SELECT parent_id
FROM atproto_reposts
WHERE actor_id = ?
  AND rkey = ?`

const deleteRepost = `
DELETE FROM atproto_reposts
WHERE actor_id = ?
  AND rkey = ?`

func (repo *Reposts) Take(actorID int64, rkey string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var parentID int64
	err := repo.db.QueryRowContext(ctx, readRepost, actorID, rkey).Scan(&parentID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	_, err = repo.db.ExecContext(ctx, deleteRepost, actorID, rkey)
	return parentID, err
}
