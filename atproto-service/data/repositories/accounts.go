package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Account is the Bluesky account somebody here connected: what they do
// here to things from Bluesky is done there as it.
type Account struct {
	PersonID int64
	DID      string
	Handle   string
	// The server that holds the account.
	PDS string
	// The long token, encrypted. The password it was got with is not kept.
	Refresh string
	// When the account's server last refused the token: the account has
	// to be connected again. Nil while it works.
	BrokenAt *time.Time
	// The time of the last thing somebody on Bluesky did to what was
	// written from here that was dealt with.
	NotifiedAt time.Time
}

type AccountsInterface interface {
	Read(personID int64) (*Account, error)
	// WhoConnected is who here connected a Bluesky account, or 0.
	WhoConnected(did string) (int64, error)
	// Shadowed is the Bluesky accounts that somebody here connected and
	// that are also kept here as people of another server.
	Shadowed() ([]string, error)
	// Working is every account that is connected and can be read as.
	Working() ([]*Account, error)
	SetNotified(personID int64, at time.Time) error
	// Save connects an account, or connects it again. ErrAccountTaken
	// when it is somebody else's here.
	Save(account *Account) error
	SetRefresh(personID int64, refresh string) error
	SetBroken(personID int64) error
	Delete(personID int64) error
}

// ErrAccountTaken is a Bluesky account that somebody else here connected.
var ErrAccountTaken = errors.New("that account is connected by somebody else")

type Accounts struct {
	db *sql.DB
}

func NewAccounts(db *sql.DB) *Accounts {
	return &Accounts{db: db}
}

const selectAccount = `
SELECT person_id, did, handle, pds, refresh_token, broken_at, COALESCE(notified_at, created_at)
FROM atproto_accounts`

const readAccount = selectAccount + `
WHERE person_id = ?`

const workingAccounts = selectAccount + `
WHERE broken_at IS NULL
ORDER BY person_id`

func scanAccount(row interface{ Scan(...any) error }) (*Account, error) {
	account := &Account{}
	var broken sql.NullTime
	err := row.Scan(&account.PersonID, &account.DID, &account.Handle, &account.PDS, &account.Refresh, &broken, &account.NotifiedAt)
	if err != nil {
		return nil, err
	}
	if broken.Valid {
		account.BrokenAt = &broken.Time
	}
	return account, nil
}

func (repo *Accounts) Working() ([]*Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	rows, err := repo.db.QueryContext(ctx, workingAccounts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

const setAccountNotified = `
UPDATE atproto_accounts
SET notified_at = ?
WHERE person_id = ?`

func (repo *Accounts) SetNotified(personID int64, at time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, setAccountNotified, at.UTC(), personID)
	return err
}

func (repo *Accounts) Read(personID int64) (*Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	account, err := scanAccount(repo.db.QueryRowContext(ctx, readAccount, personID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return account, err
}

const whoseAccount = `
SELECT person_id
FROM atproto_accounts
WHERE did = ?`

func (repo *Accounts) WhoConnected(did string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var owner int64
	err := repo.db.QueryRowContext(ctx, whoseAccount, did).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return owner, err
}

const shadowedAccounts = `
SELECT account.did
FROM atproto_accounts AS account
JOIN atproto_actors AS actor ON actor.did = account.did`

func (repo *Accounts) Shadowed() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	rows, err := repo.db.QueryContext(ctx, shadowedAccounts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dids []string
	for rows.Next() {
		var did string
		if err := rows.Scan(&did); err != nil {
			return nil, err
		}
		dids = append(dids, did)
	}
	return dids, rows.Err()
}

const saveAccount = `
INSERT INTO atproto_accounts (person_id, did, handle, pds, refresh_token, broken_at, notified_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, NULL, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    did = VALUES(did),
    handle = VALUES(handle),
    pds = VALUES(pds),
    refresh_token = VALUES(refresh_token),
    broken_at = NULL,
    notified_at = VALUES(notified_at),
    updated_at = VALUES(updated_at)`

func (repo *Accounts) Save(account *Account) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var owner int64
	err := repo.db.QueryRowContext(ctx, whoseAccount, account.DID).Scan(&owner)
	if err == nil && owner != account.PersonID {
		return ErrAccountTaken
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	now := time.Now().UTC()
	_, err = repo.db.ExecContext(ctx, saveAccount,
		account.PersonID, account.DID, account.Handle, account.PDS, account.Refresh, now, now, now)
	return err
}

const setAccountRefresh = `
UPDATE atproto_accounts
SET refresh_token = ?, broken_at = NULL, updated_at = ?
WHERE person_id = ?`

func (repo *Accounts) SetRefresh(personID int64, refresh string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, setAccountRefresh, refresh, time.Now().UTC(), personID)
	return err
}

const setAccountBroken = `
UPDATE atproto_accounts
SET broken_at = ?
WHERE person_id = ?
  AND broken_at IS NULL`

func (repo *Accounts) SetBroken(personID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, setAccountBroken, time.Now().UTC(), personID)
	return err
}

const deleteAccount = `
DELETE FROM atproto_accounts
WHERE person_id = ?`

func (repo *Accounts) Delete(personID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, deleteAccount, personID)
	return err
}
