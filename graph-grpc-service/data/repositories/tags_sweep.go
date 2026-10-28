package repositories

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/feeds"
)

// TagsRepoInterface removes the hashtags and locations no post carries any
// more (docs/plans/tag-sweep.md). Nothing else removes them: a tag is made
// the first time a post carries it, and outlives the last post that did, in
// suggestions, search and counts, with nothing behind it.
type TagsRepoInterface interface {
	SweepUnused(madeBefore time.Time) (TagSweep, error)
}

// TagSweep is how many of each kind a sweep removed.
type TagSweep struct {
	Hashtags  int
	Locations int
}

const (
	// tagSweepBatch is how many tags one statement removes; tagSweepBatches
	// caps one run, so that a site just purged is cleared over a few runs
	// rather than in one long one.
	tagSweepBatch   = 500
	tagSweepBatches = 20
)

func NewTagsRepo(db *sql.DB) TagsRepoInterface {
	return &tagsRepo{
		db:        db,
		dbTimeout: constants.DBTimeout,
	}
}

type tagsRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
}

// A hashtag no post is tagged with, made before the cutoff: anything newer
// may belong to a post being written.
const selectUnusedHashtags = `
SELECT tag.id
FROM nodes AS tag
WHERE tag.type = ?
  AND tag.created_at < ?
  AND NOT EXISTS (
      SELECT 1
      FROM edges AS tagged
      WHERE tagged.node_a_id = tag.id
        AND tagged.type = ?
  )
ORDER BY tag.id
LIMIT ?`

// The same for a location, but for one a custom feed names: a feed names
// locations by id (hashtags by name), so removing it would take the feed's
// filter with it.
const selectUnusedLocations = `
SELECT tag.id
FROM nodes AS tag
WHERE tag.type = ?
  AND tag.created_at < ?
  AND NOT EXISTS (
      SELECT 1
      FROM edges AS tagged
      WHERE tagged.node_a_id = tag.id
        AND tagged.type = ?
  )
  AND tag.id NOT IN (
      SELECT picked.location
      FROM nodes AS feed,
           JSON_TABLE(feed.meta, '$.definition.filters.locations_any[*]' COLUMNS (location BIGINT PATH '$')) AS picked
      WHERE feed.type = ?
        AND picked.location IS NOT NULL
  )
ORDER BY tag.id
LIMIT ?`

// The test again in the DELETE itself: a post tagged between the read and
// this keeps its tag.
const deleteUnusedTags = `
DELETE FROM nodes
WHERE id IN ({{ids}})
  AND type = ?
  AND NOT EXISTS (
      SELECT 1
      FROM edges AS tagged
      WHERE tagged.node_a_id = nodes.id
        AND tagged.type = ?
  )`

func (r *tagsRepo) SweepUnused(madeBefore time.Time) (TagSweep, error) {
	var swept TagSweep
	var err error

	swept.Hashtags, err = r.sweep(constants.NodeBaseHashtagEntityType, constants.EdgeTagHashtagType,
		selectUnusedHashtags, constants.NodeBaseHashtagEntityType, madeBefore, constants.EdgeTagHashtagType)
	if err != nil {
		return swept, err
	}

	swept.Locations, err = r.sweep(constants.NodeBaseLocationEntityType, constants.EdgeTagLocationType,
		selectUnusedLocations, constants.NodeBaseLocationEntityType, madeBefore, constants.EdgeTagLocationType, feeds.NodeType)
	return swept, err
}

// sweep removes one kind in batches, until a batch comes back short or the
// run's cap is reached.
func (r *tagsRepo) sweep(nodeType, edgeType, selectQuery string, selectArgs ...any) (int, error) {
	removed := 0
	for batch := 0; batch < tagSweepBatches; batch++ {
		ids, err := r.unused(selectQuery, append(selectArgs, tagSweepBatch)...)
		if err != nil || len(ids) == 0 {
			return removed, err
		}

		n, err := r.remove(ids, nodeType, edgeType)
		removed += n
		if err != nil || len(ids) < tagSweepBatch {
			return removed, err
		}
	}
	return removed, nil
}

func (r *tagsRepo) unused(query string, args ...any) ([]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query, args...)
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

func (r *tagsRepo) remove(ids []int64, nodeType, edgeType string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	query := strings.Replace(deleteUnusedTags, "{{ids}}", placeholders(len(ids)), 1)
	args := append(int64Args(ids), nodeType, edgeType)

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	return int(n), err
}
