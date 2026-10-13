package federation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// A queue of things to send to other servers, kept in a table.
//
// What is sent to another server has to arrive, and the bus the services
// talk over cannot promise that: a message published while nothing is
// listening is gone, and one that is heard by a service which then stops is
// gone too. Another server may also simply be down for an afternoon. So what
// is to be sent is written to a table first, and stays there until it has
// been sent or has been tried for two days.
//
// Each service that speaks to other servers has a table of its own, of the
// one shape this file gives (TableSQL), and its own worker. Any number of
// workers may read one table: a row is handed to one of them at a time.

const (
	statusPending = "pending"
	statusFailed  = "failed"

	// How long a worker has to send what it took. After that it is taken to
	// have stopped, and what it held is handed out again.
	sendingTimeout = 10 * time.Minute

	// The first wait after a failure, and the longest between two tries.
	firstRetry = time.Minute
	maxRetry   = 6 * time.Hour

	// How long something is tried for before it is given up. Mastodon and
	// the like keep trying for about as long.
	DefaultGiveUpAfter = 48 * time.Hour

	lastErrorMax = 500
)

var tableName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// ErrPermanent wraps a failure that trying again will not mend: the other
// server said the address is gone, or refused what was sent as wrong. What
// failed so is given up at once.
var ErrPermanent = errors.New("not to be tried again")

// Permanent marks an error as one that trying again will not mend.
func Permanent(err error) error {
	return fmt.Errorf("%w: %v", ErrPermanent, err)
}

// Delivery is one thing to send to one place.
type Delivery struct {
	ID int64
	// ActorID is who here it is sent for: whose key signs it.
	ActorID int64
	// Target is where it goes: an inbox's address, or a PDS.
	Target string
	// Payload is what is sent, as the protocol's service made it.
	Payload []byte
	// Attempts is how many times it has been tried and failed.
	Attempts  int
	CreatedAt time.Time
}

// Queue is one service's table of things to send.
type Queue struct {
	db          *sql.DB
	table       string
	giveUpAfter time.Duration
	now         func() time.Time
}

// NewQueue is the queue kept in a table. The name is put into statements as
// it is, so it is held to what a table's name may be.
func NewQueue(db *sql.DB, table string) (*Queue, error) {
	if !tableName.MatchString(table) {
		return nil, fmt.Errorf("%q is no name for a table", table)
	}
	return &Queue{db: db, table: table, giveUpAfter: DefaultGiveUpAfter, now: func() time.Time { return time.Now().UTC() }}, nil
}

// TableSQL is the statement that makes a queue's table. A service's
// migration is this, with its own table's name.
func TableSQL(table string) string {
	return `CREATE TABLE IF NOT EXISTS ` + table + ` (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    -- Said by whoever adds a row, to add it once however often they try:
    -- the activity and the inbox it is for. NULL for no such promise.
    dedupe_key varchar(191) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL,
    -- Who here it is sent for.
    actor_id bigint unsigned NOT NULL,
    -- Where it goes.
    target varchar(512) NOT NULL,
    payload mediumblob NOT NULL,
    -- pending or failed. What was sent is removed. What a worker has taken
    -- is pending still, with next_attempt_at put off: see Claim.
    status varchar(16) NOT NULL DEFAULT 'pending',
    attempts int unsigned NOT NULL DEFAULT 0,
    next_attempt_at datetime NOT NULL,
    -- When a worker last took it.
    claimed_at datetime DEFAULT NULL,
    last_error varchar(500) DEFAULT NULL,
    created_at datetime NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_dedupe_key (dedupe_key),
    KEY status_next_attempt_at (status, next_attempt_at),
    KEY actor_id (actor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`
}

// Enqueue adds something to send. With a key, a second row of the same key
// is not added, and that is no error: whatever sweeps for what the bus lost
// can add again what is already waiting.
func (q *Queue) Enqueue(ctx context.Context, actorID int64, target string, payload []byte, dedupeKey string) error {
	if actorID <= 0 || target == "" || len(payload) == 0 {
		return errors.New("something to send needs who it is for, where it goes and what it is")
	}

	var key any
	if dedupeKey != "" {
		key = dedupeKey
	}
	now := q.now()

	_, err := q.db.ExecContext(ctx, `
INSERT IGNORE INTO `+q.table+` (dedupe_key, actor_id, target, payload, status, next_attempt_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		key, actorID, target, payload, statusPending, now, now)
	return err
}

// Claim takes up to `limit` things that are due, oldest first, for this
// caller alone. Rows another worker is taking at the same moment are passed
// over, not waited for, so two workers never hold one row and neither is
// held up by the other.
//
// What a worker took and never answered for, because it stopped, is due
// again once it has been held too long.
//
// Workers taking rows at the same moment can be told by the database that
// one of them must give way. That is nothing taken this time, not an error:
// the caller asks again.
func (q *Queue) Claim(ctx context.Context, limit int) ([]Delivery, error) {
	if limit <= 0 {
		return nil, nil
	}
	now := q.now()

	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
SELECT id, actor_id, target, payload, attempts, created_at
FROM `+q.table+`
WHERE status = ? AND next_attempt_at <= ?
ORDER BY next_attempt_at, id
LIMIT ?
FOR UPDATE SKIP LOCKED`,
		statusPending, now, limit)
	if err != nil {
		return nil, notNow(err)
	}

	var claimed []Delivery
	for rows.Next() {
		var d Delivery
		if err := rows.Scan(&d.ID, &d.ActorID, &d.Target, &d.Payload, &d.Attempts, &d.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		claimed = append(claimed, d)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	for _, d := range claimed {
		// Taken by putting off when it is next due, and nothing else. If
		// this worker stops, the row is simply due again when that time
		// comes, with nothing to notice and nothing to put right.
		if _, err := tx.ExecContext(ctx, `UPDATE `+q.table+` SET next_attempt_at = ?, claimed_at = ? WHERE id = ?`, now.Add(sendingTimeout), now, d.ID); err != nil {
			return nil, notNow(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, notNow(err)
	}
	return claimed, nil
}

// notNow turns the database telling one worker to give way to another into
// nothing having happened.
func notNow(err error) error {
	if err != nil && (strings.Contains(err.Error(), "Error 1213") || strings.Contains(err.Error(), "Error 1205")) {
		return nil
	}
	return err
}

// Done removes what was sent.
func (q *Queue) Done(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `DELETE FROM `+q.table+` WHERE id = ?`, id)
	return err
}

// Fail records that something could not be sent. It is tried again later,
// each wait longer than the last, until it has been waiting two days; then,
// or at once when the failure is one that trying again will not mend, it is
// given up and kept as failed, for whoever looks into why.
func (q *Queue) Fail(ctx context.Context, d Delivery, cause error) error {
	now := q.now()
	attempts := d.Attempts + 1

	reason := ""
	if cause != nil {
		reason = cause.Error()
		if len(reason) > lastErrorMax {
			reason = reason[:lastErrorMax]
		}
	}

	next := now.Add(retryAfter(attempts))
	status := statusPending
	if errors.Is(cause, ErrPermanent) || next.Sub(d.CreatedAt) > q.giveUpAfter {
		status = statusFailed
	}

	_, err := q.db.ExecContext(ctx, `
UPDATE `+q.table+`
SET status = ?, attempts = ?, next_attempt_at = ?, claimed_at = NULL, last_error = ?
WHERE id = ?`,
		status, attempts, next, reason, d.ID)
	return err
}

// retryAfter is how long to wait after so many failures: a minute, four,
// sixteen, an hour and four minutes, four hours and sixteen, and then every
// six hours.
func retryAfter(attempts int) time.Duration {
	wait := firstRetry
	for i := 1; i < attempts; i++ {
		wait *= 4
		if wait >= maxRetry {
			return maxRetry
		}
	}
	return wait
}

// Run sends what is in the queue until the context ends: `send` is called
// for each thing due, a few at a time, and what it answers decides whether
// the row goes, waits, or is given up.
//
// One thing failing does not stop the next. A failure of the queue itself
// is logged and tried again after `every`.
func (q *Queue) Run(ctx context.Context, every time.Duration, batch int, send func(context.Context, Delivery) error) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		for {
			claimed, err := q.Claim(ctx, batch)
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("%s: taking what is due: %v", q.table, err)
				}
				break
			}
			if len(claimed) == 0 {
				break
			}

			for _, d := range claimed {
				var outcome error
				if err := send(ctx, d); err != nil {
					outcome = q.Fail(ctx, d, err)
				} else {
					outcome = q.Done(ctx, d.ID)
				}
				if outcome != nil {
					log.Printf("%s: recording delivery %d: %v", q.table, d.ID, outcome)
				}
			}

			if ctx.Err() != nil {
				return
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
