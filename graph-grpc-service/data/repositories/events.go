package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
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
	// The group hosting it, or 0.
	HostID int64
	// Whether everybody who follows the host may see that it is on and
	// answer it.
	OpenToHostFollowers bool
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

	// ListFor returns the ids of the events that are one person's, in the
	// order to show them, and how many there are: upcoming, past, hosting or
	// invited (the constants.EventList* names).
	//
	// These are the person's own, so nothing is gated here: they answered
	// it, look after it or were invited to it. Whoever sends them on decides
	// how much of an event the person was only invited to is shown.
	ListFor(personID int64, filter string, limit uint64, offset uint64) ([]int64, uint64, error)

	// ListHostedBy returns the ids of the events a group hosts, upcoming or
	// past, and how many. Unlike a person's own lists these are gated: only
	// the events this viewer may see.
	ListHostedBy(groupID int64, viewer *entities.Viewer, filter string, limit uint64, offset uint64) ([]int64, uint64, error)

	// AttendeeIDs returns who answered an event: everybody, or only the
	// ones going. For telling them something.
	AttendeeIDs(eventID int64, goingOnly bool) ([]int64, error)

	// ClaimReminders returns the events that start within the time given
	// and whose people have not been reminded, and marks each as reminded
	// as it is returned, so an event is handed out once however many
	// sweeps run.
	ClaimReminders(within time.Duration, limit int) ([]int64, error)

	// OverSince returns the events that ended before the time given and
	// are not archived yet.
	OverSince(before time.Time, limit int) ([]int64, error)
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
  online_url,
  open_to_host_followers
) VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  timezone_name = VALUES(timezone_name),
  capacity = VALUES(capacity),
  online_url = VALUES(online_url),
  open_to_host_followers = VALUES(open_to_host_followers)`

const selectEvents = `
SELECT
  event.node_id,
  event.timezone_name,
  event.capacity,
  event.online_url,
  event.cancelled_at,
  event.open_to_host_followers
FROM events AS event
WHERE event.node_id IN (%s)`

// <=> and not =, which is NULL beside a follow with nothing in its meta:
// an event all of whose followers had followed it the ordinary way had a
// count of NULL, and could not be read at all.
const countEventAnswers = `
SELECT
  edge.node_b_id,
  COALESCE(SUM(edge.meta <=> ?), 0),
  COALESCE(SUM(NOT (edge.meta <=> ?)), 0)
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

// One host an event, set once: nothing is inserted when it has one.
const insertEventHost = `
INSERT INTO edges (
  type,
  node_a_id,
  node_a_type,
  node_b_id,
  node_b_type,
  created_at,
  updated_at
)
SELECT ?, host.id, host.type, event.id, event.type, NOW(), NOW()
FROM nodes AS event
JOIN nodes AS host ON host.id = ?
WHERE event.id = ?
  AND NOT EXISTS (
    SELECT 1
    FROM edges AS had
    WHERE had.type = ?
      AND had.node_b_id = ?
  )`

const selectEventHosts = `
SELECT
  edge.node_b_id,
  edge.node_a_id
FROM edges AS edge
WHERE edge.type = ?
  AND edge.node_b_id IN (%s)`

const cancelEvent = `
UPDATE events
SET cancelled_at = NOW()
WHERE node_id = ?
  AND cancelled_at IS NULL`

func (r *eventsRepo) Save(details *EventDetails) error {
	if details == nil || details.NodeID <= 0 {
		return errors.New("an event is required")
	}

	if _, err := r.db.Exec(saveEvent,
		details.NodeID, details.TimezoneName, details.Capacity, details.OnlineURL, details.OpenToHostFollowers); err != nil {
		return err
	}

	// Who hosts it is said once. An event that has a host keeps it.
	if details.HostID <= 0 {
		return nil
	}

	_, err := r.db.Exec(insertEventHost,
		constants.EdgeActorHostEntityType,
		details.HostID,
		details.NodeID,
		constants.EdgeActorHostEntityType, details.NodeID)

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
		if err := rows.Scan(&details.NodeID, &details.TimezoneName, &details.Capacity, &details.OnlineURL, &cancelled, &details.OpenToHostFollowers); err != nil {
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

	hostArgs := append([]any{constants.EdgeActorHostEntityType}, args...)
	rows, err = r.db.Query(strings.Replace(selectEventHosts, "%s", in, 1), hostArgs...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, hostID int64
		if err := rows.Scan(&id, &hostID); err != nil {
			rows.Close()
			return nil, err
		}
		if details, ok := events[id]; ok {
			details.HostID = hostID
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

// What makes a node an event that is still there to be listed.
const eventIsListed = `
  node.type = ?
  AND node.enabled = 1
  AND node.deleted_at IS NULL`

// Answered and looked after are the same join on a different kind of edge.
const eventsByEdge = `
FROM nodes AS node
JOIN edges AS edge ON edge.node_b_id = node.id
WHERE edge.type = ?
  AND edge.node_a_id = ?
  AND` + eventIsListed

const eventsInvitedTo = `
FROM nodes AS node
JOIN edges AS edge ON edge.node_b_id = node.id
WHERE edge.type = ?
  AND edge.node_a_id = ?
  AND edge.meta = ?
  AND edge.ends_at > NOW()
  AND` + eventIsListed + `
  AND node.ends_at > NOW()`

const eventStillToCome = `
  AND node.ends_at > NOW()`

const eventOver = `
  AND node.ends_at <= NOW()`

const eventsSoonestFirst = `
ORDER BY node.starts_at ASC, node.id ASC
LIMIT ? OFFSET ?`

const eventsLatestFirst = `
ORDER BY node.starts_at DESC, node.id DESC
LIMIT ? OFFSET ?`

func (r *eventsRepo) ListFor(personID int64, filter string, limit uint64, offset uint64) ([]int64, uint64, error) {
	var (
		from  string
		order string
		args  []any
	)

	switch filter {
	case constants.EventListUpcoming:
		from = eventsByEdge + eventStillToCome
		order = eventsSoonestFirst
		args = []any{constants.EdgeFollowEntityType, personID, constants.NodeActorEventEntityType}
	case constants.EventListPast:
		from = eventsByEdge + eventOver
		order = eventsLatestFirst
		args = []any{constants.EdgeFollowEntityType, personID, constants.NodeActorEventEntityType}
	case constants.EventListHosting:
		from = eventsByEdge
		order = eventsLatestFirst
		args = []any{constants.EdgeActorAdminEntityType, personID, constants.NodeActorEventEntityType}
	case constants.EventListInvited:
		from = eventsInvitedTo
		order = eventsSoonestFirst
		args = []any{constants.EdgeActorInviteEntityType, personID, constants.InviteMetaPending, constants.NodeActorEventEntityType}
	default:
		return nil, 0, errors.New("no such list of events")
	}

	var total uint64
	if err := r.db.QueryRow("SELECT COUNT(*)"+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query("SELECT node.id"+from+order, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}

	return ids, total, rows.Err()
}

// ListHostedBy lists a group's events for a viewer. Built with the same
// gate as every other list of actors, so an event for its followers alone
// is not shown to somebody who is not one, whoever hosts it.
func (r *eventsRepo) ListHostedBy(groupID int64, viewer *entities.Viewer, filter string, limit uint64, offset uint64) ([]int64, uint64, error) {
	if viewer == nil {
		viewer = entities.NewViewer()
	}

	hosted := func(columns string) sq.SelectBuilder {
		query := sq.Select(columns).
			From("nodes AS node").
			Join("edges AS host ON host.node_b_id = node.id").
			Where(sq.Eq{
				"host.type":      constants.EdgeActorHostEntityType,
				"host.node_a_id": groupID,
				"node.type":      constants.NodeActorEventEntityType,
				"node.enabled":   true,
			}).
			Where("node.deleted_at IS NULL")

		if filter == constants.EventListPast {
			query = query.Where("node.ends_at <= NOW()")
		} else {
			query = query.Where("node.ends_at > NOW()")
		}

		// What the viewer may see anyway, and, for somebody who follows
		// this group, the events that let their host's followers in.
		// Those come back to a caller that has to send them on as their
		// name and time alone: see the relation's ViaHost.
		if viewer.IsAdmin() {
			return query
		}

		gate := sq.Or{actorAccessCondition(viewer)}
		if viewer.ID > 0 {
			gate = append(gate, sq.And{
				relNotBlockedEitherWay(viewer.ID, "node.id"),
				sq.Expr(`
        EXISTS (
          SELECT 1
          FROM events AS open
          JOIN edges AS follows ON follows.node_b_id = host.node_a_id
          WHERE open.node_id = node.id
            AND open.open_to_host_followers = 1
            AND follows.type = ?
            AND follows.node_a_id = ?
        )`, constants.EdgeFollowEntityType, viewer.ID),
			})
		}

		return query.Where(gate)
	}

	if filter != constants.EventListPast && filter != constants.EventListUpcoming {
		return nil, 0, errors.New("no such list of events")
	}

	countSQL, countArgs, err := hosted("COUNT(*)").ToSql()
	if err != nil {
		return nil, 0, err
	}
	var total uint64
	if err := r.db.QueryRow(countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	list := hosted("node.id")
	if filter == constants.EventListPast {
		list = list.OrderBy("node.starts_at DESC", "node.id DESC")
	} else {
		list = list.OrderBy("node.starts_at ASC", "node.id ASC")
	}
	listSQL, listArgs, err := list.Limit(limit).Offset(offset).ToSql()
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(listSQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}

	return ids, total, rows.Err()
}

const selectAttendees = `
SELECT edge.node_a_id
FROM edges AS edge
WHERE edge.type = ?
  AND edge.node_b_id = ?`

const andGoing = `
  AND edge.meta = ?`

func (r *eventsRepo) AttendeeIDs(eventID int64, goingOnly bool) ([]int64, error) {
	query := selectAttendees
	args := []any{constants.EdgeFollowEntityType, eventID}
	if goingOnly {
		query += andGoing
		args = append(args, constants.EventRsvpGoingMeta)
	}

	return r.ids(query, args...)
}

const selectRemindersDue = `
SELECT event.node_id
FROM events AS event
JOIN nodes AS node ON node.id = event.node_id
WHERE event.reminded_at IS NULL
  AND event.cancelled_at IS NULL
  AND node.enabled = 1
  AND node.deleted_at IS NULL
  AND node.starts_at > NOW()
  AND node.starts_at <= ?
ORDER BY node.starts_at ASC
LIMIT ?`

const claimReminder = `
UPDATE events
SET reminded_at = NOW()
WHERE node_id = ?
  AND reminded_at IS NULL`

func (r *eventsRepo) ClaimReminders(within time.Duration, limit int) ([]int64, error) {
	due, err := r.ids(selectRemindersDue, time.Now().UTC().Add(within), limit)
	if err != nil {
		return nil, err
	}

	// Each is claimed by the update. A sweep that lost the race for one
	// changes no row and leaves it to whoever won.
	claimed := []int64{}
	for _, id := range due {
		result, err := r.db.Exec(claimReminder, id)
		if err != nil {
			return claimed, err
		}
		if changed, _ := result.RowsAffected(); changed > 0 {
			claimed = append(claimed, id)
		}
	}

	return claimed, nil
}

const selectEventsOver = `
SELECT node.id
FROM nodes AS node
WHERE node.type = ?
  AND node.archived_at IS NULL
  AND node.deleted_at IS NULL
  AND node.ends_at <= ?
ORDER BY node.ends_at ASC
LIMIT ?`

func (r *eventsRepo) OverSince(before time.Time, limit int) ([]int64, error) {
	return r.ids(selectEventsOver, constants.NodeActorEventEntityType, before.UTC(), limit)
}

// ids runs a query that returns one column of ids, in its order.
func (r *eventsRepo) ids(query string, args ...any) ([]int64, error) {
	rows, err := r.db.Query(query, args...)
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
