package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const dbTimeout = 3 * time.Second

// Account is a person's choice to be on the fediverse, or not.
type Account struct {
	PersonID int64
	Enabled  bool
	// When they last chose to be.
	EnabledAt time.Time
	// Whether other servers may list them, and find them in search. On
	// until they say otherwise.
	Discoverable bool
	Indexable    bool
}

// AccountsInterface is who has chosen to be on the fediverse.
type AccountsInterface interface {
	// Read is a person's choice. Somebody who has never chosen is not
	// enabled, and that is no error.
	Read(personID int64) (*Account, error)
	// Set records a person's choice.
	Set(personID int64, enabled bool) error
	// SetListing records whether other servers may list them and find them.
	SetListing(personID int64, discoverable, indexable bool) error
	// Told records what of a person their followers' servers now have:
	// the person's own updated_at when they were sent their description.
	Told(personID int64, updatedAt time.Time) error
}

type Accounts struct {
	db *sql.DB
}

func NewAccounts(db *sql.DB) *Accounts {
	return &Accounts{db: db}
}

const readAccount = `
SELECT enabled, enabled_at, discoverable, indexable
FROM activitypub_accounts
WHERE person_id = ?`

func (repo *Accounts) Read(personID int64) (*Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	account := &Account{PersonID: personID, Discoverable: true, Indexable: true}
	var enabledAt sql.NullTime
	err := repo.db.QueryRowContext(ctx, readAccount, personID).Scan(
		&account.Enabled, &enabledAt, &account.Discoverable, &account.Indexable)
	if errors.Is(err, sql.ErrNoRows) {
		return account, nil
	}
	if err != nil {
		return nil, err
	}
	account.EnabledAt = enabledAt.Time
	return account, nil
}

const toldAccount = `
UPDATE activitypub_accounts
SET told_updated_at = ?
WHERE person_id = ?`

func (repo *Accounts) Told(personID int64, updatedAt time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, toldAccount, updatedAt.UTC(), personID)
	return err
}

// Somebody who has never gone on the fediverse may choose these first.
const setListing = `
INSERT INTO activitypub_accounts (person_id, discoverable, indexable, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    discoverable = VALUES(discoverable),
    indexable    = VALUES(indexable),
    updated_at   = VALUES(updated_at)`

func (repo *Accounts) SetListing(personID int64, discoverable, indexable bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	now := time.Now().UTC()
	_, err := repo.db.ExecContext(ctx, setListing, personID, discoverable, indexable, now, now)
	return err
}

// One row a person. Choosing again changes it; when they first chose is
// kept, and when they last turned it on is brought up to date.
const setAccount = `
INSERT INTO activitypub_accounts (person_id, enabled, enabled_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    enabled_at = IF(VALUES(enabled) AND NOT enabled, VALUES(enabled_at), enabled_at),
    enabled    = VALUES(enabled),
    updated_at = VALUES(updated_at)`

func (repo *Accounts) Set(personID int64, enabled bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	now := time.Now().UTC()
	var enabledAt any
	if enabled {
		enabledAt = now
	}
	_, err := repo.db.ExecContext(ctx, setAccount, personID, enabled, enabledAt, now, now)
	return err
}
