package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
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
	// The old accounts this one also is, as addresses: its alsoKnownAs
	// (roadmap row 124, part 3).
	AlsoKnownAs []string
	// The account this one moved to, and when; "" and zero until it moves.
	MovedTo string
	MovedAt time.Time
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
	// SetAliases records the old accounts a person also is.
	SetAliases(personID int64, addresses []string) error
	// SetMoved records that a person moved to an account, now; "" takes
	// it back, when they move back.
	SetMoved(personID int64, to string) error
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
SELECT enabled, enabled_at, discoverable, indexable, also_known_as, moved_to, moved_at
FROM activitypub_accounts
WHERE person_id = ?`

func (repo *Accounts) Read(personID int64) (*Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	account := &Account{PersonID: personID, Discoverable: true, Indexable: true}
	var (
		enabledAt, movedAt sql.NullTime
		aliases, movedTo   sql.NullString
	)
	err := repo.db.QueryRowContext(ctx, readAccount, personID).Scan(
		&account.Enabled, &enabledAt, &account.Discoverable, &account.Indexable,
		&aliases, &movedTo, &movedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return account, nil
	}
	if err != nil {
		return nil, err
	}
	account.EnabledAt = enabledAt.Time
	if aliases.Valid && aliases.String != "" {
		_ = json.Unmarshal([]byte(aliases.String), &account.AlsoKnownAs)
	}
	account.MovedTo, account.MovedAt = movedTo.String, movedAt.Time
	return account, nil
}

// Somebody who has never gone on the fediverse may name them first, as
// they may choose their listing.
const setAliases = `
INSERT INTO activitypub_accounts (person_id, also_known_as, created_at, updated_at)
VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    also_known_as = VALUES(also_known_as),
    updated_at    = VALUES(updated_at)`

func (repo *Accounts) SetAliases(personID int64, addresses []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var kept any
	if len(addresses) > 0 {
		raw, err := json.Marshal(addresses)
		if err != nil {
			return err
		}
		kept = string(raw)
	}
	now := time.Now().UTC()
	_, err := repo.db.ExecContext(ctx, setAliases, personID, kept, now, now)
	return err
}

const setMoved = `
UPDATE activitypub_accounts
SET moved_to   = ?,
    moved_at   = IF(? IS NULL, moved_at, ?),
    updated_at = ?
WHERE person_id = ?`

// SetMoved keeps when they last moved even once they move back: the thirty
// days before they may move again are counted from it.
func (repo *Accounts) SetMoved(personID int64, to string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	now := time.Now().UTC()
	var movedTo, at any
	if to != "" {
		movedTo, at = to, now
	}
	_, err := repo.db.ExecContext(ctx, setMoved, movedTo, at, at, now, personID)
	return err
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
