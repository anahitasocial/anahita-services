package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// RemoteActor is what is known of somebody on another server, beyond what
// their node holds: where to write to them, and the key what they send is
// checked against.
type RemoteActor struct {
	NodeID      int64
	URI         string
	Inbox       string
	SharedInbox string
	KeyID       string
	PublicKey   string
	IconURL     string
	Locked      bool
	FetchedAt   time.Time
}

// ActorsInterface is the people of other servers who are kept here.
type ActorsInterface interface {
	// Save keeps what was just read of somebody, in place of what was kept.
	Save(actor *RemoteActor) error
	// ReadByNode is what is kept of the person a node is, or nil.
	ReadByNode(nodeID int64) (*RemoteActor, error)
	// ReadByURI is what is kept of the person at an address, or nil.
	ReadByURI(uri string) (*RemoteActor, error)
}

type Actors struct {
	db *sql.DB
}

func NewActors(db *sql.DB) *Actors {
	return &Actors{db: db}
}

const saveActor = `
INSERT INTO activitypub_actors
    (node_id, uri, inbox, shared_inbox, key_id, public_key, icon_url, locked, fetched_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    inbox        = VALUES(inbox),
    shared_inbox = VALUES(shared_inbox),
    key_id       = VALUES(key_id),
    public_key   = VALUES(public_key),
    icon_url     = VALUES(icon_url),
    locked       = VALUES(locked),
    fetched_at   = VALUES(fetched_at)`

func (repo *Actors) Save(actor *RemoteActor) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	now := time.Now().UTC()
	_, err := repo.db.ExecContext(ctx, saveActor,
		actor.NodeID, actor.URI, actor.Inbox, actor.SharedInbox, actor.KeyID, actor.PublicKey,
		actor.IconURL, actor.Locked, now, now,
	)
	return err
}

const readActor = `
SELECT node_id, uri, inbox, shared_inbox, key_id, public_key, icon_url, locked, fetched_at
FROM activitypub_actors
WHERE `

func (repo *Actors) ReadByNode(nodeID int64) (*RemoteActor, error) {
	return repo.read(readActor+"node_id = ?", nodeID)
}

func (repo *Actors) ReadByURI(uri string) (*RemoteActor, error) {
	return repo.read(readActor+"uri = ?", uri)
}

func (repo *Actors) read(query string, arg any) (*RemoteActor, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	actor := &RemoteActor{}
	err := repo.db.QueryRowContext(ctx, query, arg).Scan(
		&actor.NodeID, &actor.URI, &actor.Inbox, &actor.SharedInbox, &actor.KeyID,
		&actor.PublicKey, &actor.IconURL, &actor.Locked, &actor.FetchedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return actor, nil
}
