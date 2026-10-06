package repositories

import (
	"context"
	"database/sql"

	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// QuotesRepoInterface is what quote posts need beyond what any post does.
//
// A quote is a note with the id of the post it quotes. Nothing else about
// it is different from a note, so there is no listing here and no gate: it
// is read, listed and gated as the note it is.
type QuotesRepoInterface interface {
	// Recount puts right the number of notes quoting a post.
	Recount(quotedID int64) error

	// Detach takes the quoted post out of one note that quotes it. The
	// note stays. It reports whether anything changed: false when the note
	// quotes nothing, or was detached already.
	Detach(quoteNoteID int64, by int64) (bool, error)

	// List returns the notes that quote a post and that this viewer may
	// see, newest first: not one the post was taken out of.
	List(quotedID int64, viewer *entities.Viewer, limit uint64) ([]*entities.AnahitaNode, error)
}

// QuotesLimit is how many of a post's quotes are read at once.
const QuotesLimit = 100

func NewQuotesRepo(db *sql.DB) QuotesRepoInterface {
	return &quotesRepo{db: db}
}

type quotesRepo struct {
	db *sql.DB
}

// A post's count is the notes that still quote it: not one that has been
// removed or switched off, and not one its author took it out of.
//
// Counted over the rows, not kept by adding and subtracting, so it is right
// whatever happened before it.
const recountQuotes = `
UPDATE nodes quoted
SET quoted.quote_count = (
  SELECT COUNT(*)
  FROM (
    SELECT quoting.id
    FROM nodes quoting
    WHERE quoting.quote_id = ?
      AND quoting.quote_detached_at IS NULL
      AND quoting.enabled = 1
      AND quoting.deleted_at IS NULL
  ) quotes
)
WHERE quoted.id = ?`

func (r *quotesRepo) Recount(quotedID int64) error {
	if quotedID <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, recountQuotes, quotedID, quotedID)
	return err
}

// quote_id is kept, so the note still says what it quoted and cannot be made
// to quote it again: the same note is detached, and a new one would be asked
// of the post's policy like any other.
const detachQuote = `
UPDATE nodes
SET quote_detached_at = UTC_TIMESTAMP(),
    modified_by = ?,
    updated_at = UTC_TIMESTAMP()
WHERE id = ?
  AND quote_id IS NOT NULL
  AND quote_detached_at IS NULL`

func (r *quotesRepo) Detach(quoteNoteID int64, by int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeStatementTimeout)
	defer cancel()

	result, err := r.db.ExecContext(ctx, detachQuote, by, quoteNoteID)
	if err != nil {
		return false, err
	}

	changed, err := result.RowsAffected()
	return changed > 0, err
}

// List returns the notes that quote a post.
//
// Each is a note like any other and is gated as one: a note the viewer could
// not see on its author's profile is not listed as a quote either, so the
// list is shorter than the count on the post for anybody who cannot see them
// all.
func (r *quotesRepo) List(quotedID int64, viewer *entities.Viewer, limit uint64) ([]*entities.AnahitaNode, error) {
	if viewer == nil {
		viewer = entities.NewViewer()
	}
	if limit == 0 || limit > QuotesLimit {
		limit = QuotesLimit
	}

	query := GetBaseSelectQuery().
		Where(sq.Eq{"node.quote_id": quotedID}).
		Where("node.quote_detached_at IS NULL").
		Where("node.deleted_at IS NULL").
		Where(sq.Eq{"node.enabled": true})

	query = SetMediumAccessQuery(query, viewer)

	query = query.OrderBy("node.created_at DESC", "node.id DESC").Limit(limit)

	return FetchNodes(r.db, query)
}
