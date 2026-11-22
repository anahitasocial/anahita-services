package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"anahita-libs/constants"
)

// What the bus lost: posts that should have been written to Bluesky and
// were not, because the word that they were made never came (banu.io,
// 2026-11-22: a channel's post, made while the graph could not reach the
// bus). The fediverse has the same sweep (activitypub-service,
// data/repositories/missed.go).

type MissedInterface interface {
	// PostsNotWritten is posts made, or changed, between `since` and
	// `olderThan` by somebody who had connected a working Bluesky account
	// by then, at the top of their own profile and open to everybody, that
	// were never written there. Whether each is one that goes is for
	// whoever is handed it to say.
	PostsNotWritten(kinds []string, since, olderThan time.Time, limit int) ([]int64, error)
}

type Missed struct {
	db *sql.DB
}

func NewMissed(db *sql.DB) *Missed {
	return &Missed{db: db}
}

// A post that came from its author's own account on Bluesky is there
// already (its meta says so, under "bluesky"), and is not looked at.
const postsNotWritten = `
SELECT note.id
FROM nodes AS note
JOIN atproto_accounts AS account
  ON account.person_id = note.created_by
 AND account.broken_at IS NULL
LEFT JOIN atproto_sent AS sent
       ON sent.kind = ?
      AND sent.local_id = note.id
WHERE note.type IN (%s)
  AND note.owner_id = note.created_by
  AND note.parent_id IS NULL
  AND note.root_id IS NULL
  AND note.access = ?
  AND note.enabled = 1
  AND note.deleted_at IS NULL
  AND note.library_only = 0
  AND note.draft = 0
  AND note.uri IS NULL
  AND NOT (JSON_VALID(note.meta) AND JSON_CONTAINS_PATH(note.meta, 'one', '$.bluesky'))
  AND note.updated_at >= account.created_at
  AND note.updated_at >= ?
  AND note.updated_at <= ?
  AND sent.id IS NULL
ORDER BY note.id
LIMIT ?`

func (repo *Missed) PostsNotWritten(kinds []string, since, olderThan time.Time, limit int) ([]int64, error) {
	if len(kinds) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	args := []any{SentPost}
	for _, kind := range kinds {
		args = append(args, kind)
	}
	args = append(args, constants.AnahitaAccessPublic, since.UTC(), olderThan.UTC(), limit)

	rows, err := repo.db.QueryContext(ctx, fmt.Sprintf(postsNotWritten, strings.TrimSuffix(strings.Repeat("?, ", len(kinds)), ", ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
