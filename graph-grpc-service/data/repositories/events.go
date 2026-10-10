package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"anahita-libs/constants"
)

// EventDetails is what only an event has, beside the node every actor is,
// and how many have answered.
type EventDetails struct {
	NodeID       int64
	TimezoneName string
	Capacity     uint32
	OnlineURL    string
	CancelledAt  *time.Time
	// How many are going, and how many said maybe.
	GoingCount uint64
	MaybeCount uint64
	// What the person asked about has answered: going, maybe, or "".
	ViewerRsvp string
}

// EventsRepoInterface is what only an event has: its time zone, how many
// may go, where to join online, whether it was called off, and who is going.
//
// Going to an event is following it, with the answer on the follow edge.
// Whether somebody MAY answer (they can see the event, or were invited) is
// asked before this is called. What it guards is the number of places.
type EventsRepoInterface interface {
	// Save writes an event's own details, making the row when there is none.
	// It does not call an event off, or back on: see Cancel.
	Save(details *EventDetails) error

	// Read returns the details of each of these events, with what this
	// person answered. An id that is not an event has no entry.
	Read(ids []int64, personID int64) (map[int64]*EventDetails, error)

	// Rsvp records a person's answer, or takes it back when rsvp is "". It
	// returns what became of it: saved, full, closed or missing.
	Rsvp(eventID int64, personID int64, rsvp string) (string, error)

	// Cancel calls an event off. It answers false when it already was, or
	// there is no such event.
	Cancel(eventID int64) (bool, error)
}

func NewEventsRepo(db *sql.DB) EventsRepoInterface {
	return &eventsRepo{db: db, dbTimeout: constants.DBTimeout}
}

type eventsRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
}

const saveEvent = `
INSERT INTO events (
  node_id,
  timezone_name,
  capacity,
  online_url
) VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  timezone_name = VALUES(timezone_name),
  capacity = VALUES(capacity),
  online_url = VALUES(online_url)`

const selectEvents = `
SELECT
  event.node_id,
  event.timezone_name,
  event.capacity,
  event.online_url,
  event.cancelled_at
FROM events AS event
WHERE event.node_id IN (%s)`

const countEventAnswers = `
SELECT
  edge.node_b_id,
  SUM(edge.meta = ?),
  SUM(edge.meta IS NULL OR edge.meta <> ?)
FROM edges AS edge
WHERE edge.type = ?
  AND edge.node_b_id IN (%s)
GROUP BY edge.node_b_id`

const selectViewerAnswers = `
SELECT
  edge.node_b_id,
  COALESCE(edge.meta, '')
FROM edges AS edge
WHERE edge.type = ?
  AND edge.node_a_id = ?
  AND edge.node_b_id IN (%s)`

// The row is locked for as long as an answer is being recorded, so that two
// people cannot both be given the last place.
const lockEvent = `
SELECT
  event.capacity,
  event.cancelled_at IS NOT NULL,
  COALESCE(node.ends_at, NOW()) <= NOW(),
  node.type
FROM events AS event
JOIN nodes AS node ON node.id = event.node_id
WHERE event.node_id = ?
FOR UPDATE`

const countGoingBesides = `
SELECT COUNT(*)
FROM edges
WHERE type = ?
  AND node_b_id = ?
  AND meta = ?
  AND node_a_id <> ?`

const upsertAnswer = `
INSERT INTO edges (
  type,
  node_a_id,
  node_a_type,
  node_b_id,
  node_b_type,
  meta,
  created_at,
  created_by,
  updated_at,
  modified_by
) VALUES (?, ?, ?, ?, ?, ?, NOW(), ?, NOW(), ?)
ON DUPLICATE KEY UPDATE
  meta = VALUES(meta),
  updated_at = NOW(),
  modified_by = VALUES(modified_by)`

const deleteAnswer = `
DELETE FROM edges
WHERE type = ?
  AND node_a_id = ?
  AND node_b_id = ?`

const cancelEvent = `
UPDATE events
SET cancelled_at = NOW()
WHERE node_id = ?
  AND cancelled_at IS NULL`

func (r *eventsRepo) Save(details *EventDetails) error {
	if details == nil || details.NodeID <= 0 {
		return errors.New("an event is required")
	}

	_, err := r.db.Exec(saveEvent,
		details.NodeID, details.TimezoneName, details.Capacity, details.OnlineURL)

	return err
}

func (r *eventsRepo) Read(ids []int64, personID int64) (map[int64]*EventDetails, error) {
	events := map[int64]*EventDetails{}
	if len(ids) == 0 {
		return events, nil
	}

	in, args := idPlaceholders(ids)

	rows, err := r.db.Query(strings.Replace(selectEvents, "%s", in, 1), args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		details := &EventDetails{}
		var cancelled sql.NullTime
		if err := rows.Scan(&details.NodeID, &details.TimezoneName, &details.Capacity, &details.OnlineURL, &cancelled); err != nil {
			rows.Close()
			return nil, err
		}
		if cancelled.Valid {
			details.CancelledAt = &cancelled.Time
		}
		events[details.NodeID] = details
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return events, nil
	}

	countArgs := append([]any{constants.EventRsvpGoingMeta, constants.EventRsvpGoingMeta, constants.EdgeFollowEntityType}, args...)
	rows, err = r.db.Query(strings.Replace(countEventAnswers, "%s", in, 1), countArgs...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			id           int64
			going, maybe uint64
		)
		if err := rows.Scan(&id, &going, &maybe); err != nil {
			rows.Close()
			return nil, err
		}
		if details, ok := events[id]; ok {
			details.GoingCount = going
			details.MaybeCount = maybe
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if personID <= 0 {
		return events, nil
	}

	answerArgs := append([]any{constants.EdgeFollowEntityType, personID}, args...)
	rows, err = r.db.Query(strings.Replace(selectViewerAnswers, "%s", in, 1), answerArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id   int64
			meta string
		)
		if err := rows.Scan(&id, &meta); err != nil {
			return nil, err
		}
		if details, ok := events[id]; ok {
			details.ViewerRsvp = constants.EventRsvpFromMeta(meta)
		}
	}

	return events, rows.Err()
}

// Rsvp records an answer under a lock on the event, so the number going
// never passes the number of places.
//
// Taking an answer back is always allowed, over or called off or not:
// somebody can always stop following. Answering is not, once the event is
// over or called off. Going is refused when every place is taken by somebody
// else; maybe is not counted against the places.
func (r *eventsRepo) Rsvp(eventID int64, personID int64, rsvp string) (string, error) {
	if rsvp == "" {
		if _, err := r.db.Exec(deleteAnswer, constants.EdgeFollowEntityType, personID, eventID); err != nil {
			return "", err
		}
		return constants.EventRsvpSaved, nil
	}

	meta := constants.EventRsvpMeta(rsvp)
	if meta == "" {
		return "", errors.New("no such answer")
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var (
		capacity     uint32
		isCancelled  bool
		isOver       bool
		eventType    string
		alreadyGoing uint64
	)
	err = tx.QueryRowContext(ctx, lockEvent, eventID).Scan(&capacity, &isCancelled, &isOver, &eventType)
	if errors.Is(err, sql.ErrNoRows) {
		return constants.EventRsvpMissing, nil
	}
	if err != nil {
		return "", err
	}

	if isCancelled || isOver {
		return constants.EventRsvpClosed, nil
	}

	if rsvp == constants.EventRsvpGoing && capacity > 0 {
		if err = tx.QueryRowContext(ctx, countGoingBesides,
			constants.EdgeFollowEntityType, eventID, constants.EventRsvpGoingMeta, personID).Scan(&alreadyGoing); err != nil {
			return "", err
		}
		if alreadyGoing >= uint64(capacity) {
			return constants.EventRsvpFull, nil
		}
	}

	if _, err = tx.ExecContext(ctx, upsertAnswer,
		constants.EdgeFollowEntityType,
		personID, constants.NodeActorPersonEntityType,
		eventID, eventType,
		meta,
		personID, personID,
	); err != nil {
		return "", err
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}

	return constants.EventRsvpSaved, nil
}

func (r *eventsRepo) Cancel(eventID int64) (bool, error) {
	result, err := r.db.Exec(cancelEvent, eventID)
	if err != nil {
		return false, err
	}

	changed, err := result.RowsAffected()

	return changed > 0, err
}
