package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// What was written to Bluesky for somebody here: a reply, a like, a
// repost. Each is remembered by the name it was written under, so that it
// can be taken back, answered, and known for this installation's own when
// Bluesky tells of it.

// The kinds of thing written.
const (
	SentPost   = "post"
	SentLike   = "like"
	SentRepost = "repost"
	// A follow of somebody on Bluesky. LocalID is their node here.
	SentFollow = "follow"
)

// Sent is one thing written.
type Sent struct {
	ID       int64
	Kind     string
	PersonID int64
	// What it is here: the reply's node, the node that was liked, or the
	// repost's node.
	LocalID int64
	DID     string
	RKey    string
}

type SentInterface interface {
	// Begin remembers something about to be written. False when it was
	// remembered already: it is written once.
	Begin(sent *Sent) (bool, error)
	// Read finds what was written for something here. With `personID` 0,
	// whoever wrote it.
	Read(kind string, localID, personID int64) (*Sent, error)
	Delete(id int64) error
	DeleteAllOf(personID int64) error
	// ReadByName finds what was written under a name, or nil.
	ReadByName(did, kind, rkey string) (*Sent, error)
	// DeleteByName forgets one thing by the name it was to be written
	// under.
	DeleteByName(did, kind, rkey string) error
	// IsOurs says whether a record Bluesky tells of is one written from
	// here.
	IsOurs(did, kind, rkey string) (bool, error)
}

type SentRecords struct {
	db *sql.DB
}

func NewSent(db *sql.DB) *SentRecords {
	return &SentRecords{db: db}
}

const beginSent = `
INSERT IGNORE INTO atproto_sent (kind, person_id, local_id, did, rkey, created_at)
VALUES (?, ?, ?, ?, ?, ?)`

func (repo *SentRecords) Begin(sent *Sent) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	result, err := repo.db.ExecContext(ctx, beginSent,
		sent.Kind, sent.PersonID, sent.LocalID, sent.DID, sent.RKey, time.Now().UTC())
	if err != nil {
		return false, err
	}
	added, err := result.RowsAffected()
	return added == 1, err
}

const readSent = `
SELECT id, kind, person_id, local_id, did, rkey
FROM atproto_sent
WHERE kind = ?
  AND local_id = ?
  AND (? = 0 OR person_id = ?)
ORDER BY id
LIMIT 1`

func (repo *SentRecords) Read(kind string, localID, personID int64) (*Sent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	sent := &Sent{}
	err := repo.db.QueryRowContext(ctx, readSent, kind, localID, personID, personID).
		Scan(&sent.ID, &sent.Kind, &sent.PersonID, &sent.LocalID, &sent.DID, &sent.RKey)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return sent, err
}

const deleteSent = `
DELETE FROM atproto_sent
WHERE id = ?`

func (repo *SentRecords) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, deleteSent, id)
	return err
}

const deleteSentOf = `
DELETE FROM atproto_sent
WHERE person_id = ?`

func (repo *SentRecords) DeleteAllOf(personID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, deleteSentOf, personID)
	return err
}

const readSentByName = `
SELECT id, kind, person_id, local_id, did, rkey
FROM atproto_sent
WHERE did = ?
  AND rkey = ?
  AND kind = ?`

func (repo *SentRecords) ReadByName(did, kind, rkey string) (*Sent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	sent := &Sent{}
	err := repo.db.QueryRowContext(ctx, readSentByName, did, rkey, kind).
		Scan(&sent.ID, &sent.Kind, &sent.PersonID, &sent.LocalID, &sent.DID, &sent.RKey)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return sent, err
}

const deleteSentByName = `
DELETE FROM atproto_sent
WHERE did = ?
  AND rkey = ?
  AND kind = ?`

func (repo *SentRecords) DeleteByName(did, kind, rkey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, deleteSentByName, did, rkey, kind)
	return err
}

const isSent = `
SELECT EXISTS (
    SELECT 1
    FROM atproto_sent
    WHERE did = ?
      AND rkey = ?
      AND kind = ?
)`

func (repo *SentRecords) IsOurs(did, kind, rkey string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var ours bool
	err := repo.db.QueryRowContext(ctx, isSent, did, rkey, kind).Scan(&ours)
	return ours, err
}
