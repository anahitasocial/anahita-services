package repositories

import (
	"context"
	"database/sql"
	"time"

	"anahita-libs/constants"
	"anahita-libs/videos"
)

// What the bus may have lost.
//
// This service is told of what happens here on a bus that can drop a
// message: a follow, a post, a post taken away. What it was not told of,
// other servers were not told of either. So now and then it looks for
// itself: at what is in the graph and at what it has on record, and where
// the two differ, at which of them is behind.
//
// It only looks. Each thing found is handed to what would have handled the
// event, which decides again from the thing as it is now, and does once
// what it would have done then.

// Pair is a follow: who here, and who elsewhere.
type Pair struct {
	LocalID  int64
	RemoteID int64
}

// MissedInterface finds where the graph and this service's records differ.
type MissedInterface interface {
	// Follows of somebody elsewhere that are in the graph and that the
	// other server was never asked about.
	FollowsNotAsked(olderThan time.Time, limit int) ([]Pair, error)
	// Follows the other server was asked about that are no longer in the
	// graph.
	FollowsGone(limit int) ([]Pair, error)
	// Notes of people on the fediverse that are open to everybody, or are
	// replies in a thread that is, written since they joined it and since
	// `since`, that other servers were never sent. Whether each is one
	// that goes out is for whoever is handed it to say.
	NotesNotSent(since, olderThan time.Time, limit int) ([]int64, error)
	// Notes other servers were sent that are gone, or are no longer open
	// to everybody.
	NotesGone(limit int) ([]int64, error)
	// Notes other servers have that were changed here since they were last
	// sent.
	NotesEdited(olderThan time.Time, limit int) ([]int64, error)
	// People on the fediverse whose profile was changed since their
	// followers' servers were last sent it.
	PeopleEdited(olderThan time.Time, limit int) ([]int64, error)
	// Everybody who is on the fediverse.
	People(limit int) ([]int64, error)
}

type Missed struct {
	db *sql.DB
}

func NewMissed(db *sql.DB) *Missed {
	return &Missed{db: db}
}

const followsNotAsked = `
SELECT follow.node_a_id, follow.node_b_id
FROM edges AS follow
LEFT JOIN activitypub_follows AS asked
       ON asked.local_id = follow.node_a_id
      AND asked.remote_id = follow.node_b_id
      AND asked.direction = 'out'
WHERE follow.type = ?
  AND follow.node_b_type = ?
  AND follow.node_a_type <> ?
  AND follow.created_at <= ?
  AND asked.id IS NULL
ORDER BY follow.id
LIMIT ?`

func (repo *Missed) FollowsNotAsked(olderThan time.Time, limit int) ([]Pair, error) {
	return repo.pairs(followsNotAsked,
		constants.EdgeFollowEntityType,
		constants.NodeActorActivityPubPersonEntityType,
		constants.NodeActorActivityPubPersonEntityType,
		olderThan, limit)
}

const followsGone = `
SELECT asked.local_id, asked.remote_id
FROM activitypub_follows AS asked
LEFT JOIN edges AS follow
       ON follow.type = ?
      AND follow.node_a_id = asked.local_id
      AND follow.node_b_id = asked.remote_id
WHERE asked.direction = 'out'
  AND follow.id IS NULL
ORDER BY asked.id
LIMIT ?`

func (repo *Missed) FollowsGone(limit int) ([]Pair, error) {
	return repo.pairs(followsGone, constants.EdgeFollowEntityType, limit)
}

const notesNotSent = `
SELECT note.id
FROM nodes AS note
JOIN activitypub_accounts AS account
  ON account.person_id = note.created_by
 AND account.enabled = 1
LEFT JOIN nodes AS root
       ON root.id = note.root_id
LEFT JOIN activitypub_objects AS sent
       ON sent.node_id = note.id
WHERE note.type IN (?, ?, ?)
  AND note.enabled = 1
  AND note.deleted_at IS NULL
  AND COALESCE(root.access, note.access) = ?
  AND note.owner_id = note.created_by
  AND note.uri IS NULL
  AND note.created_at >= account.enabled_at
  AND note.created_at >= ?
  AND note.created_at <= ?
  AND sent.node_id IS NULL
ORDER BY note.id
LIMIT ?`

func (repo *Missed) NotesNotSent(since, olderThan time.Time, limit int) ([]int64, error) {
	return repo.ids(notesNotSent,
		constants.NodeMediumNoteEntityType, constants.NodeMediumPhotoEntityType, videos.NodeType, constants.AnahitaAccessPublic, since, olderThan, limit)
}

const notesGone = `
SELECT sent.node_id
FROM activitypub_objects AS sent
LEFT JOIN nodes AS note
       ON note.id = sent.node_id
LEFT JOIN nodes AS root
       ON root.id = note.root_id
WHERE note.id IS NULL
   OR note.enabled = 0
   OR note.deleted_at IS NOT NULL
   OR (note.root_id IS NOT NULL AND root.id IS NULL)
   OR COALESCE(root.access, note.access) <> ?
ORDER BY sent.node_id
LIMIT ?`

func (repo *Missed) NotesGone(limit int) ([]int64, error) {
	return repo.ids(notesGone, constants.AnahitaAccessPublic, limit)
}

// A second's grace: a post is written and then touched again within the
// same moment by what gives it its hashtags, and that is not a change.
const notesEdited = `
SELECT sent.node_id
FROM activitypub_objects AS sent
JOIN nodes AS note
  ON note.id = sent.node_id
LEFT JOIN nodes AS root
       ON root.id = note.root_id
WHERE sent.kind = 'note'
  AND note.enabled = 1
  AND note.deleted_at IS NULL
  AND COALESCE(root.access, note.access) = ?
  AND note.updated_at > COALESCE(sent.told_updated_at, note.created_at) + INTERVAL 1 SECOND
  AND note.updated_at <= ?
ORDER BY sent.node_id
LIMIT ?`

func (repo *Missed) NotesEdited(olderThan time.Time, limit int) ([]int64, error) {
	return repo.ids(notesEdited, constants.AnahitaAccessPublic, olderThan, limit)
}

const peopleEdited = `
SELECT account.person_id
FROM activitypub_accounts AS account
JOIN nodes AS person
  ON person.id = account.person_id
WHERE account.enabled = 1
  AND person.updated_at > COALESCE(account.told_updated_at, account.enabled_at) + INTERVAL 1 SECOND
  AND person.updated_at <= ?
ORDER BY account.person_id
LIMIT ?`

func (repo *Missed) PeopleEdited(olderThan time.Time, limit int) ([]int64, error) {
	return repo.ids(peopleEdited, olderThan, limit)
}

const peopleOnTheFediverse = `
SELECT person_id
FROM activitypub_accounts
WHERE enabled = 1
ORDER BY person_id
LIMIT ?`

func (repo *Missed) People(limit int) ([]int64, error) {
	return repo.ids(peopleOnTheFediverse, limit)
}

func (repo *Missed) pairs(query string, args ...any) ([]Pair, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	rows, err := repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var found []Pair
	for rows.Next() {
		var pair Pair
		if err := rows.Scan(&pair.LocalID, &pair.RemoteID); err != nil {
			return nil, err
		}
		found = append(found, pair)
	}
	return found, rows.Err()
}

func (repo *Missed) ids(query string, args ...any) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	rows, err := repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var found []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		found = append(found, id)
	}
	return found, rows.Err()
}
