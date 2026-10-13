package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Where the listener is in what Bluesky has said: the time of the last
// thing it dealt with, and whose turn it is to listen.

type CursorInterface interface {
	// Read is the time of the last thing dealt with, in microseconds, or
	// 0 when nothing has been.
	Read() (int64, error)
	// Save keeps it, never moving it back.
	Save(timeUS int64) error
}

type Cursor struct {
	db *sql.DB
}

func NewCursor(db *sql.DB) *Cursor {
	return &Cursor{db: db}
}

const readCursor = `
SELECT time_us
FROM atproto_cursor
WHERE id = 1`

func (repo *Cursor) Read() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var timeUS int64
	err := repo.db.QueryRowContext(ctx, readCursor).Scan(&timeUS)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return timeUS, err
}

const saveCursor = `
INSERT INTO atproto_cursor (id, time_us, updated_at)
VALUES (1, ?, ?)
ON DUPLICATE KEY UPDATE
    time_us = GREATEST(time_us, VALUES(time_us)),
    updated_at = VALUES(updated_at)`

func (repo *Cursor) Save(timeUS int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, saveCursor, timeUS, time.Now().UTC())
	return err
}

// The name of the lock that says whose turn it is. One for the whole
// database server, so it carries the database's name.
const listenerLock = "anahita_atproto_listener"

// Turn is the right to be the one copy of this service that listens. It
// is a lock held on a connection of its own, and is lost with it: a copy
// that dies gives its turn up without being asked.
type Turn struct {
	conn *sql.Conn
	name string
}

const (
	takeTurn  = `SELECT GET_LOCK(CONCAT(DATABASE(), '.', ?), 0)`
	holdsTurn = `SELECT IS_USED_LOCK(CONCAT(DATABASE(), '.', ?)) = CONNECTION_ID()`
	giveTurn  = `SELECT RELEASE_LOCK(CONCAT(DATABASE(), '.', ?))`
)

// TakeTurn asks for it. Nil, and no error, when another copy has it.
func TakeTurn(ctx context.Context, db *sql.DB) (*Turn, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}

	var taken sql.NullBool
	if err := conn.QueryRowContext(ctx, takeTurn, listenerLock).Scan(&taken); err != nil || !taken.Bool {
		_ = conn.Close()
		return nil, err
	}
	return &Turn{conn: conn, name: listenerLock}, nil
}

// Held says whether the turn is still this copy's: its connection may have
// been lost while it was not looking.
func (turn *Turn) Held(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	var held sql.NullBool
	err := turn.conn.QueryRowContext(ctx, holdsTurn, turn.name).Scan(&held)
	return err == nil && held.Bool
}

// Give gives the turn up.
func (turn *Turn) Give() {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var released sql.NullBool
	_ = turn.conn.QueryRowContext(ctx, giveTurn, turn.name).Scan(&released)
	_ = turn.conn.Close()
}
