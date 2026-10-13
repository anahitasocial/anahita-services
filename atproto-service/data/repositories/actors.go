package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const dbTimeout = 3 * time.Second

// RemoteActor is what this service alone keeps of somebody on Bluesky.
// Who they are to everybody else (their name, their handle, what they say
// of themselves) is their node in the graph.
type RemoteActor struct {
	NodeID int64
	// Their lasting name: what does not change when their handle does.
	DID    string
	Handle string
	// Where their pictures were when last read, to tell when one changed.
	AvatarURL string
	BannerURL string
	// When what they posted before they were followed was read. Nil when
	// it has not been.
	BackfilledAt *time.Time
}

type ActorsInterface interface {
	Save(actor *RemoteActor) error
	ReadByDID(did string) (*RemoteActor, error)
	ReadByNode(nodeID int64) (*RemoteActor, error)
	Delete(nodeID int64) error
	// IsFollowedFromHere says whether anybody here follows them.
	IsFollowedFromHere(nodeID int64) (bool, error)
	// Followed is the DIDs of everybody on Bluesky that somebody here
	// follows, at most `limit` of them.
	Followed(limit int) ([]string, error)
	// NotBackfilled is who is followed from here and has never had their
	// earlier posts read.
	NotBackfilled(limit int) ([]*RemoteActor, error)
	// BeginBackfill claims the reading of somebody's earlier posts. False
	// when it was claimed already.
	BeginBackfill(nodeID int64) (bool, error)
	// UndoBackfill gives the claim back, for a reading that failed.
	UndoBackfill(nodeID int64) error
}

type Actors struct {
	db *sql.DB
}

func NewActors(db *sql.DB) *Actors {
	return &Actors{db: db}
}

const saveActor = `
INSERT INTO atproto_actors (node_id, did, handle, avatar_url, banner_url, fetched_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    handle = VALUES(handle),
    avatar_url = VALUES(avatar_url),
    banner_url = VALUES(banner_url),
    fetched_at = VALUES(fetched_at)`

func (repo *Actors) Save(actor *RemoteActor) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	now := time.Now().UTC()
	_, err := repo.db.ExecContext(ctx, saveActor,
		actor.NodeID, actor.DID, actor.Handle, actor.AvatarURL, actor.BannerURL, now, now)
	return err
}

const selectActor = `
SELECT node_id, did, handle, avatar_url, banner_url, backfilled_at
FROM atproto_actors`

const readActorByDID = selectActor + `
WHERE did = ?`

const readActorByNode = selectActor + `
WHERE node_id = ?`

func (repo *Actors) ReadByDID(did string) (*RemoteActor, error) {
	return repo.read(readActorByDID, did)
}

func (repo *Actors) ReadByNode(nodeID int64) (*RemoteActor, error) {
	return repo.read(readActorByNode, nodeID)
}

func (repo *Actors) read(query string, arg any) (*RemoteActor, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	actor, err := scanActor(repo.db.QueryRowContext(ctx, query, arg))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return actor, err
}

func scanActor(row interface{ Scan(...any) error }) (*RemoteActor, error) {
	actor := &RemoteActor{}
	var backfilled sql.NullTime
	err := row.Scan(&actor.NodeID, &actor.DID, &actor.Handle, &actor.AvatarURL, &actor.BannerURL, &backfilled)
	if err != nil {
		return nil, err
	}
	if backfilled.Valid {
		actor.BackfilledAt = &backfilled.Time
	}
	return actor, nil
}

const deleteActor = `
DELETE FROM atproto_actors
WHERE node_id = ?`

func (repo *Actors) Delete(nodeID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, deleteActor, nodeID)
	return err
}

// A follow of somebody on Bluesky is kept where every follow is: an edge
// in the graph, from somebody here. Somebody here is a node that came
// from nowhere else.
const followedFromHere = `
    SELECT 1
    FROM edges AS follow
    JOIN nodes AS follower ON follower.id = follow.node_a_id
    WHERE follow.node_b_id = actor.node_id
      AND follow.type = 'edge.actor.follow.v1'
      AND follower.uri IS NULL`

const isFollowedFromHere = `
SELECT EXISTS (
    SELECT 1
    FROM atproto_actors AS actor
    WHERE actor.node_id = ?
      AND EXISTS (` + followedFromHere + `
      )
)`

func (repo *Actors) IsFollowedFromHere(nodeID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var followed bool
	err := repo.db.QueryRowContext(ctx, isFollowedFromHere, nodeID).Scan(&followed)
	return followed, err
}

const followedDIDs = `
SELECT actor.did
FROM atproto_actors AS actor
WHERE EXISTS (` + followedFromHere + `
)
ORDER BY actor.node_id
LIMIT ?`

func (repo *Actors) Followed(limit int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*dbTimeout)
	defer cancel()

	rows, err := repo.db.QueryContext(ctx, followedDIDs, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dids := []string{}
	for rows.Next() {
		var did string
		if err := rows.Scan(&did); err != nil {
			return nil, err
		}
		dids = append(dids, did)
	}
	return dids, rows.Err()
}

const notBackfilled = `
SELECT actor.node_id, actor.did, actor.handle, actor.avatar_url, actor.banner_url, actor.backfilled_at
FROM atproto_actors AS actor
WHERE actor.backfilled_at IS NULL
  AND EXISTS (` + followedFromHere + `
  )
ORDER BY actor.node_id
LIMIT ?`

func (repo *Actors) NotBackfilled(limit int) ([]*RemoteActor, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*dbTimeout)
	defer cancel()

	rows, err := repo.db.QueryContext(ctx, notBackfilled, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	actors := []*RemoteActor{}
	for rows.Next() {
		actor, err := scanActor(rows)
		if err != nil {
			return nil, err
		}
		actors = append(actors, actor)
	}
	return actors, rows.Err()
}

const beginBackfill = `
UPDATE atproto_actors
SET backfilled_at = ?
WHERE node_id = ?
  AND backfilled_at IS NULL`

func (repo *Actors) BeginBackfill(nodeID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	result, err := repo.db.ExecContext(ctx, beginBackfill, time.Now().UTC(), nodeID)
	if err != nil {
		return false, err
	}
	claimed, err := result.RowsAffected()
	return claimed == 1, err
}

const undoBackfill = `
UPDATE atproto_actors
SET backfilled_at = NULL
WHERE node_id = ?`

func (repo *Actors) UndoBackfill(nodeID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, undoBackfill, nodeID)
	return err
}
