package repositories

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"
)

// DomainBlock is a server this installation will have nothing to do with.
type DomainBlock struct {
	Domain    string    `json:"domain"`
	Reason    string    `json:"reason"`
	CreatedBy int64     `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// BlocksInterface is the servers that are refused.
type BlocksInterface interface {
	// IsBlocked says whether a server is refused: itself, or any name it
	// is under.
	IsBlocked(domain string) bool
	// Browse is a page of the list, by name, and how many there are in
	// all. `search` narrows it to the names and reasons it is found in.
	Browse(search string, limit, offset int) ([]*DomainBlock, int, error)
	Add(block *DomainBlock) error
	Delete(domain string) error
}

// How long the list is remembered before it is read again. It is asked of
// every message that arrives; a refusal that takes half a minute to take
// hold is the price of not asking the database each time.
const blocksRemembered = 30 * time.Second

type Blocks struct {
	db *sql.DB

	mu      sync.Mutex
	domains map[string]bool
	readAt  time.Time
}

func NewBlocks(db *sql.DB) *Blocks {
	return &Blocks{db: db}
}

// CleanDomain is a server's name as it is kept and compared: lower case,
// with nothing around it, and no address or handle left on it.
func CleanDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://")
	if at := strings.LastIndex(domain, "@"); at >= 0 {
		domain = domain[at+1:]
	}
	if slash := strings.IndexAny(domain, "/:"); slash >= 0 {
		domain = domain[:slash]
	}
	return strings.Trim(domain, ".")
}

func (repo *Blocks) IsBlocked(domain string) bool {
	domain = CleanDomain(domain)
	if domain == "" {
		return false
	}

	blocked := repo.remembered()
	for {
		if blocked[domain] {
			return true
		}
		dot := strings.Index(domain, ".")
		if dot < 0 {
			return false
		}
		domain = domain[dot+1:]
	}
}

const readBlockedDomains = `
SELECT domain
FROM federation_domain_blocks`

// remembered is the list, read again when it is stale. When it cannot be
// read the last one known is kept: a database that is away for a moment
// does not open the door to everybody who was refused.
func (repo *Blocks) remembered() map[string]bool {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if repo.domains != nil && time.Since(repo.readAt) < blocksRemembered {
		return repo.domains
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	rows, err := repo.db.QueryContext(ctx, readBlockedDomains)
	if err != nil {
		if repo.domains == nil {
			repo.domains = map[string]bool{}
		}
		return repo.domains
	}
	defer rows.Close()

	domains := map[string]bool{}
	for rows.Next() {
		var domain string
		if rows.Scan(&domain) == nil {
			domains[domain] = true
		}
	}
	repo.domains, repo.readAt = domains, time.Now()
	return domains
}

func (repo *Blocks) forget() {
	repo.mu.Lock()
	repo.readAt = time.Time{}
	repo.mu.Unlock()
}

const countBlocks = `
SELECT COUNT(*)
FROM federation_domain_blocks
WHERE domain LIKE ? ESCAPE '!'
   OR reason LIKE ? ESCAPE '!'`

const browseBlocks = `
SELECT domain, reason, created_by, created_at
FROM federation_domain_blocks
WHERE domain LIKE ? ESCAPE '!'
   OR reason LIKE ? ESCAPE '!'
ORDER BY domain
LIMIT ? OFFSET ?`

// What is searched for is looked for as it was typed, not as a pattern.
var likeEscaper = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")

// Browse reads a page and not the list: an installation that takes a list
// of servers to refuse from somebody else has thousands.
func (repo *Blocks) Browse(search string, limit, offset int) ([]*DomainBlock, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	pattern := "%" + likeEscaper.Replace(strings.TrimSpace(search)) + "%"

	var total int
	if err := repo.db.QueryRowContext(ctx, countBlocks, pattern, pattern).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := repo.db.QueryContext(ctx, browseBlocks, pattern, pattern, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	blocks := []*DomainBlock{}
	for rows.Next() {
		block := &DomainBlock{}
		if err := rows.Scan(&block.Domain, &block.Reason, &block.CreatedBy, &block.CreatedAt); err != nil {
			return nil, 0, err
		}
		blocks = append(blocks, block)
	}
	return blocks, total, rows.Err()
}

const addBlock = `
INSERT INTO federation_domain_blocks (domain, reason, created_by, created_at)
VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    reason = VALUES(reason)`

func (repo *Blocks) Add(block *DomainBlock) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, addBlock, block.Domain, block.Reason, block.CreatedBy, time.Now().UTC())
	repo.forget()
	return err
}

const deleteBlock = `
DELETE FROM federation_domain_blocks
WHERE domain = ?`

func (repo *Blocks) Delete(domain string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, deleteBlock, domain)
	repo.forget()
	return err
}
