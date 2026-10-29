package federation

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"
)

// The accounts on other networks an installation refuses one by one
// (docs/plans/done/refused-accounts.md): kept by what does not change, the
// actor's address on the fediverse and the DID on Bluesky, and read by both
// services that listen to other networks. Refused by administrators; what
// is here of an account is removed when it is.

// The networks an account may be refused on.
const (
	NetworkActivityPub = "activitypub"
	NetworkATProto     = "atproto"
)

// RefusedAccount is one account on the list.
type RefusedAccount struct {
	ID int64 `json:"id"`
	// `activitypub` or `atproto`.
	Network string `json:"network"`
	// The actor's address, or the DID.
	Identifier string `json:"identifier"`
	// How it was shown when it was refused.
	Handle string `json:"handle"`
	Name   string `json:"name"`
	// Why it is refused. For administrators only.
	Reason string `json:"reason"`
	// The administrator who refused it.
	CreatedBy int64 `json:"created_by"`
	// When.
	CreatedAt time.Time `json:"created_at"`
}

// ErrAlreadyRefused is an account that is on the list already.
var ErrAlreadyRefused = errors.New("already refused")

// RefusedAccountsInterface is the accounts that are refused.
type RefusedAccountsInterface interface {
	// IsRefused says whether this account is refused.
	IsRefused(network, identifier string) bool
	// Refused is every account refused on one network, by identifier.
	Refused(network string) []string
	// Browse is a page of the list, newest first, and how many there are.
	// `search` narrows it to the names, handles and reasons it is in.
	Browse(search string, limit, offset int) ([]*RefusedAccount, int, error)
	// Add refuses an account, answering ErrAlreadyRefused when it is.
	Add(account *RefusedAccount) (*RefusedAccount, error)
	// Read is one account on the list, or nil.
	Read(id int64) (*RefusedAccount, error)
	Delete(id int64) error
}

type RefusedAccounts struct {
	db *sql.DB

	mu     sync.Mutex
	known  map[string]map[string]bool
	readAt time.Time
}

func NewRefusedAccounts(db *sql.DB) *RefusedAccounts {
	return &RefusedAccounts{db: db}
}

// CleanIdentifier is an identifier as it is kept and compared: a DID in
// lower case, an address as it is, both with nothing around them.
func CleanIdentifier(network, identifier string) string {
	identifier = strings.TrimSpace(identifier)
	if network == NetworkATProto {
		return strings.ToLower(identifier)
	}
	return identifier
}

func (repo *RefusedAccounts) IsRefused(network, identifier string) bool {
	identifier = CleanIdentifier(network, identifier)
	if identifier == "" {
		return false
	}
	return repo.remembered()[network][identifier]
}

func (repo *RefusedAccounts) Refused(network string) []string {
	list := []string{}
	for identifier := range repo.remembered()[network] {
		list = append(list, identifier)
	}
	return list
}

const readRefusedAccounts = `
SELECT network, identifier
FROM federation_account_blocks`

// remembered is the list, read again when it is stale, as the servers' list
// is: asked of every message that arrives, it is not asked of the database
// each time. When it cannot be read the last one known is kept, so that a
// database away for a moment lets nobody in who was refused.
func (repo *RefusedAccounts) remembered() map[string]map[string]bool {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if repo.known != nil && time.Since(repo.readAt) < blocksRemembered {
		return repo.known
	}

	ctx, cancel := context.WithTimeout(context.Background(), blocksTimeout)
	defer cancel()

	rows, err := repo.db.QueryContext(ctx, readRefusedAccounts)
	if err != nil {
		if repo.known == nil {
			repo.known = map[string]map[string]bool{}
		}
		return repo.known
	}
	defer rows.Close()

	known := map[string]map[string]bool{}
	for rows.Next() {
		var network, identifier string
		if rows.Scan(&network, &identifier) == nil {
			if known[network] == nil {
				known[network] = map[string]bool{}
			}
			known[network][identifier] = true
		}
	}
	repo.known, repo.readAt = known, time.Now()
	return known
}

func (repo *RefusedAccounts) forget() {
	repo.mu.Lock()
	repo.readAt = time.Time{}
	repo.mu.Unlock()
}

const countRefusedAccounts = `
SELECT COUNT(*)
FROM federation_account_blocks
WHERE name LIKE ? ESCAPE '!'
   OR handle LIKE ? ESCAPE '!'
   OR reason LIKE ? ESCAPE '!'`

const browseRefusedAccounts = `
SELECT id, network, identifier, handle, name, reason, created_by, created_at
FROM federation_account_blocks
WHERE name LIKE ? ESCAPE '!'
   OR handle LIKE ? ESCAPE '!'
   OR reason LIKE ? ESCAPE '!'
ORDER BY created_at DESC, id DESC
LIMIT ? OFFSET ?`

func (repo *RefusedAccounts) Browse(search string, limit, offset int) ([]*RefusedAccount, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), blocksTimeout)
	defer cancel()

	pattern := "%" + likeEscaper.Replace(strings.TrimSpace(search)) + "%"

	var total int
	if err := repo.db.QueryRowContext(ctx, countRefusedAccounts, pattern, pattern, pattern).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := repo.db.QueryContext(ctx, browseRefusedAccounts, pattern, pattern, pattern, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	accounts := []*RefusedAccount{}
	for rows.Next() {
		account, err := scanRefusedAccount(rows)
		if err != nil {
			return nil, 0, err
		}
		accounts = append(accounts, account)
	}
	return accounts, total, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRefusedAccount(row scanner) (*RefusedAccount, error) {
	account := &RefusedAccount{}
	err := row.Scan(&account.ID, &account.Network, &account.Identifier, &account.Handle,
		&account.Name, &account.Reason, &account.CreatedBy, &account.CreatedAt)
	return account, err
}

const readRefusedAccount = `
SELECT id, network, identifier, handle, name, reason, created_by, created_at
FROM federation_account_blocks
WHERE id = ?`

func (repo *RefusedAccounts) Read(id int64) (*RefusedAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), blocksTimeout)
	defer cancel()

	account, err := scanRefusedAccount(repo.db.QueryRowContext(ctx, readRefusedAccount, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return account, err
}

const addRefusedAccount = `
INSERT IGNORE INTO federation_account_blocks
    (network, identifier, handle, name, reason, created_by, created_at)
VALUES
    (?, ?, ?, ?, ?, ?, ?)`

func (repo *RefusedAccounts) Add(account *RefusedAccount) (*RefusedAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), blocksTimeout)
	defer cancel()

	account.Identifier = CleanIdentifier(account.Network, account.Identifier)
	account.CreatedAt = time.Now().UTC().Truncate(time.Second)

	result, err := repo.db.ExecContext(ctx, addRefusedAccount, account.Network, account.Identifier,
		account.Handle, account.Name, account.Reason, account.CreatedBy, account.CreatedAt)
	repo.forget()
	if err != nil {
		return nil, err
	}
	if added, _ := result.RowsAffected(); added == 0 {
		return nil, ErrAlreadyRefused
	}
	account.ID, _ = result.LastInsertId()
	return account, nil
}

const deleteRefusedAccount = `
DELETE FROM federation_account_blocks
WHERE id = ?`

func (repo *RefusedAccounts) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), blocksTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, deleteRefusedAccount, id)
	repo.forget()
	return err
}
