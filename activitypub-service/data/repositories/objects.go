package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Object is a post made here that other servers have been told of.
type Object struct {
	NodeID  int64
	UUID    string
	ActorID int64
	// For a reply: the inbox of whoever it answers, when they are on
	// another server. They are told of it, and told when it is gone.
	ReplyInbox string
	// KindNote or KindRepost. Empty is a note.
	Kind string
}

// What was sent: a post or a reply, or a repost of one.
const (
	KindNote   = "note"
	KindRepost = "repost"
)

// ObjectsInterface is the posts that went out.
type ObjectsInterface interface {
	// Begin records a post as sent, and says whether it was this call that
	// did: the one that is told yes is the one that sends it.
	Begin(object *Object) (bool, error)
	// Read is the record of a post, or nil when it never went out.
	Read(nodeID int64) (*Object, error)
	// End forgets a post, and says whether it was this call that did.
	End(nodeID int64) (bool, error)
	// EndAll forgets every post of a person's.
	EndAll(actorID int64) error
	// Told records what of a post other servers now have: the post's own
	// updated_at when they were sent it.
	Told(nodeID int64, updatedAt time.Time) error
	// Count is how many of a person's posts went out.
	Count(actorID int64) (int64, error)
}

type Objects struct {
	db *sql.DB
}

func NewObjects(db *sql.DB) *Objects {
	return &Objects{db: db}
}

const beginObject = `
INSERT IGNORE INTO activitypub_objects (node_id, uuid, actor_id, kind, reply_inbox, created_at)
VALUES (?, ?, ?, ?, ?, ?)`

func (repo *Objects) Begin(object *Object) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	result, err := repo.db.ExecContext(ctx, beginObject, object.NodeID, object.UUID, object.ActorID, kindOf(object), object.ReplyInbox, time.Now().UTC())
	if err != nil {
		return false, err
	}
	added, err := result.RowsAffected()
	return added == 1, err
}

func kindOf(object *Object) string {
	if object.Kind == KindRepost {
		return KindRepost
	}
	return KindNote
}

const toldObject = `
UPDATE activitypub_objects
SET told_updated_at = ?
WHERE node_id = ?`

func (repo *Objects) Told(nodeID int64, updatedAt time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, toldObject, updatedAt.UTC(), nodeID)
	return err
}

const readObject = `
SELECT node_id, uuid, actor_id, kind, reply_inbox
FROM activitypub_objects
WHERE node_id = ?`

func (repo *Objects) Read(nodeID int64) (*Object, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	object := &Object{}
	err := repo.db.QueryRowContext(ctx, readObject, nodeID).Scan(&object.NodeID, &object.UUID, &object.ActorID, &object.Kind, &object.ReplyInbox)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return object, nil
}

const endObject = `
DELETE FROM activitypub_objects
WHERE node_id = ?`

func (repo *Objects) End(nodeID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	result, err := repo.db.ExecContext(ctx, endObject, nodeID)
	if err != nil {
		return false, err
	}
	gone, err := result.RowsAffected()
	return gone == 1, err
}

const endAllObjects = `
DELETE FROM activitypub_objects
WHERE actor_id = ?`

func (repo *Objects) EndAll(actorID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, endAllObjects, actorID)
	return err
}

const countObjects = `
SELECT COUNT(*)
FROM activitypub_objects
WHERE actor_id = ?`

func (repo *Objects) Count(actorID int64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var count int64
	err := repo.db.QueryRowContext(ctx, countObjects, actorID).Scan(&count)
	return count, err
}
