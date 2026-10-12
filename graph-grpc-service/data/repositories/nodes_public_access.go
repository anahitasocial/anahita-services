package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"anahita-libs/constants"
)

// Making everything public visible to signed-in people only, in one step.
//
// For an installation that has turned on MIN_CONTENT_ACCESS=registered. That
// setting stops anything NEW being public and leaves alone what already is;
// this is how a super administrator brings the rest into line, once.
//
// What it changes: people, groups and posts, with their revisions, whose
// access is exactly "public". Hashtags and places are left: they are not
// anybody's content, and reading one shows only the posts the reader may see
// anyway.
//
// It cannot be undone. Nothing records which rows were public before, and
// that is deliberate: a list of what used to be public is a list of what
// somebody chose to stop showing.

const (
	SiteActionMadePublicRegistered = "made-public-registered"

	publicAccessChunk = 5000
)

// PublicAccessCounts is how much is public, by kind.
type PublicAccessCounts struct {
	People int64 `json:"people"`
	Groups int64 `json:"groups"`
	Posts  int64 `json:"posts"`
}

// Total is every row MakePublicRegistered would change.
func (c *PublicAccessCounts) Total() int64 {
	return c.People + c.Groups + c.Posts
}

type PublicAccessRepoInterface interface {
	// Count says how much is public now.
	Count() (*PublicAccessCounts, error)

	// MakeRegistered changes everything Count counts to "registered", records
	// the action against performedBy, and returns what it changed. Safe to
	// run again: the second time there is nothing public left.
	MakeRegistered(performedBy int64) (*PublicAccessCounts, error)
}

func NewPublicAccessRepo(db *sql.DB) PublicAccessRepoInterface {
	return &publicAccessRepo{db: db}
}

type publicAccessRepo struct {
	db *sql.DB
}

// The three kinds, as the rows of one statement. A person is told from a
// group by its type; everything else that is not a tag is a post or part of
// one. Somebody kept from another server is an actor and not of the person
// type, and is no group: they have a uri.
const countPublicAccess = `
SELECT
  COALESCE(SUM(type = ?), 0)                                   AS people,
  COALESCE(SUM(type LIKE 'node.actor.%' AND type <> ? AND uri IS NULL), 0) AS ` + "`groups`" + `,
  COALESCE(SUM(type NOT LIKE 'node.actor.%'), 0)               AS posts
FROM nodes
WHERE access = ?
  AND type NOT LIKE 'node.tag.%'`

func (r *publicAccessRepo) Count() (*PublicAccessCounts, error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	counts := &PublicAccessCounts{}
	err := r.db.QueryRowContext(ctx, countPublicAccess,
		constants.NodeActorPersonEntityType,
		constants.NodeActorPersonEntityType,
		constants.AnahitaAccessPublic,
	).Scan(&counts.People, &counts.Groups, &counts.Posts)
	if err != nil {
		return nil, err
	}

	return counts, nil
}

func (r *publicAccessRepo) MakeRegistered(performedBy int64) (*PublicAccessCounts, error) {
	// Counted first, so what is recorded is what there was to change.
	counts, err := r.Count()
	if err != nil {
		return nil, err
	}

	// In chunks, each its own statement, so no one statement holds locks on
	// every public row at once. A chunk that fails leaves the rest public,
	// and running it again carries on: only rows still public match.
	//
	// updated_at is left alone. Nobody edited these posts.
	const makeRegistered = `
UPDATE nodes
SET access = ?
WHERE access = ?
  AND type NOT LIKE 'node.tag.%'
LIMIT ?`

	for {
		ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
		result, err := r.db.ExecContext(ctx, makeRegistered,
			constants.AnahitaAccessRegistered,
			constants.AnahitaAccessPublic,
			publicAccessChunk,
		)
		cancel()
		if err != nil {
			return nil, err
		}

		changed, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if changed < publicAccessChunk {
			break
		}
	}

	detail, err := json.Marshal(counts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	const recordAction = `
INSERT INTO site_actions (
  action,
  performed_by,
  detail,
  created_at
)
VALUES (?, ?, ?, ?)`

	if _, err := r.db.ExecContext(ctx, recordAction,
		SiteActionMadePublicRegistered, performedBy, string(detail), time.Now().UTC(),
	); err != nil {
		return nil, err
	}

	return counts, nil
}
