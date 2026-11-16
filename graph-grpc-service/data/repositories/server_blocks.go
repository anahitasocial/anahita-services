package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"anahita-libs/constants"

	sq "github.com/Masterminds/squirrel"
)

// The servers a person blocked for themselves (roadmap row 152), in
// person_domain_blocks. A server is its domain, lowercase, without a scheme
// or a port; blocking it blocks every server under it too, as the site's
// own refused servers do (anahita-libs/federation, IsBlocked).

// ServerBlocksInterface keeps them.
type ServerBlocksInterface interface {
	Add(personID int64, domain string) error
	Delete(personID int64, domain string) error
	// Browse is the person's blocked servers, the latest first, and how
	// many there are.
	Browse(personID int64, start, limit uint64) ([]*ServerBlockRow, int64, error)
	// Domains is all of them, for what is filtered outside SQL.
	Domains(personID int64) ([]string, error)
	// Follows is the follows, either way, between the person and anybody
	// on the server or one under it, as source and target.
	Follows(personID int64, domain string) ([][2]int64, error)
}

// ServerBlockRow is one blocked server.
type ServerBlockRow struct {
	Domain    string
	CreatedAt time.Time
}

type serverBlocksRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
}

func NewServerBlocksRepo(db *sql.DB) ServerBlocksInterface {
	return &serverBlocksRepo{db: db, dbTimeout: 10 * time.Second}
}

// onBlockedServer is the SQL of a column holding a node's server, as
// nodes.origin_domain does, being the blocked server `b.domain` or one
// under it. A port, which a server's address may carry, is not part of
// the server.
func onBlockedServer(column string) string {
	host := fmt.Sprintf("SUBSTRING_INDEX(%s, ':', 1)", column)
	return fmt.Sprintf("(%s = b.domain OR %s LIKE CONCAT('%%.', b.domain))", host, host)
}

// onServer is the same against a domain given as a parameter.
func onServer(column string) string {
	host := fmt.Sprintf("SUBSTRING_INDEX(%s, ':', 1)", column)
	return fmt.Sprintf("(%s = ? OR %s LIKE CONCAT('%%.', ?))", host, host)
}

const (
	addServerBlock = `
INSERT IGNORE INTO person_domain_blocks (person_id, domain, created_at)
VALUES (?, ?, UTC_TIMESTAMP())`

	deleteServerBlock = `
DELETE FROM person_domain_blocks
WHERE person_id = ?
  AND domain = ?`

	countServerBlocks = `
SELECT COUNT(*)
FROM person_domain_blocks
WHERE person_id = ?`

	browseServerBlocks = `
SELECT domain, created_at
FROM person_domain_blocks
WHERE person_id = ?
ORDER BY created_at DESC, domain
LIMIT ? OFFSET ?`

	allServerBlocks = `
SELECT domain
FROM person_domain_blocks
WHERE person_id = ?`
)

// followsWithServer is the follows between the person and anybody whose
// server is the one given, or one under it: the other end of the edge is
// read for its origin_domain.
var followsWithServer = `
SELECT e.node_a_id, e.node_b_id
FROM edges e
JOIN nodes other ON other.id = IF(e.node_a_id = ?, e.node_b_id, e.node_a_id)
WHERE e.type = ?
  AND (e.node_a_id = ? OR e.node_b_id = ?)
  AND ` + onServer("other.origin_domain")

func (r *serverBlocksRepo) Add(personID int64, domain string) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()
	_, err := r.db.ExecContext(ctx, addServerBlock, personID, domain)
	return err
}

func (r *serverBlocksRepo) Delete(personID int64, domain string) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()
	_, err := r.db.ExecContext(ctx, deleteServerBlock, personID, domain)
	return err
}

func (r *serverBlocksRepo) Browse(personID int64, start, limit uint64) ([]*ServerBlockRow, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	var total int64
	if err := r.db.QueryRowContext(ctx, countServerBlocks, personID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, browseServerBlocks, personID, limit, start)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	blocks := []*ServerBlockRow{}
	for rows.Next() {
		row := &ServerBlockRow{}
		if err := rows.Scan(&row.Domain, &row.CreatedAt); err != nil {
			return nil, 0, err
		}
		blocks = append(blocks, row)
	}
	return blocks, total, rows.Err()
}

func (r *serverBlocksRepo) Domains(personID int64) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, allServerBlocks, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	domains := []string{}
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}
	return domains, rows.Err()
}

func (r *serverBlocksRepo) Follows(personID int64, domain string) ([][2]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, followsWithServer,
		personID, constants.EdgeFollowEntityType, personID, personID, domain, domain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	follows := [][2]int64{}
	for rows.Next() {
		var source, target int64
		if err := rows.Scan(&source, &target); err != nil {
			return nil, err
		}
		follows = append(follows, [2]int64{source, target})
	}
	return follows, rows.Err()
}

// fromBlockedServer keeps the rows of a list that are not from a server the
// viewer blocked: a post or a reply whose own server, or whose profile's,
// is blocked, and a repost of one. The row is aliased `node`.
func fromBlockedServer(viewerID int64) sq.Sqlizer {
	return sq.Expr(`NOT EXISTS (
          SELECT 1 FROM person_domain_blocks b
          WHERE b.person_id = ?
            AND (`+onBlockedServer("node.origin_domain")+`
              OR EXISTS (
                SELECT 1 FROM nodes owner_server
                WHERE owner_server.id IN (COALESCE(node.owner_id, 0), COALESCE(node.created_by, 0))
                  AND `+onBlockedServer("owner_server.origin_domain")+`
              )
              OR (node.type = ? AND EXISTS (
                SELECT 1 FROM nodes reposted_server
                WHERE reposted_server.id = node.parent_id
                  AND `+onBlockedServer("reposted_server.origin_domain")+`
              )))
        )`, viewerID, constants.NodeBaseRepostType)
}

// OnBlockedServer says whether a node's server (its origin_domain) is one of
// these blocked servers or under one, as onBlockedServer says it in SQL:
// for what is filtered outside SQL, a thread's replies.
func OnBlockedServer(origin string, blocked []string) bool {
	host := strings.ToLower(strings.TrimSpace(origin))
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return false
	}
	for _, domain := range blocked {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
