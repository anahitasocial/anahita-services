package repositories

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"
)

// The states of an item.
const (
	// Seen, and never to be posted: what the feed held when its website was
	// made.
	ItemSeen = "seen"
	// To be posted, held back by the hourly cap.
	ItemWaiting = "waiting"
	// Posted: node_id is the article it became.
	ItemPosted = "posted"
)

// ItemSeenAs is an item of a feed as it is kept.
type ItemSeenAs struct {
	Key         string
	ContentHash string
}

// ContentHash is what says an item has changed: its title and its excerpt.
func ContentHash(title, excerpt string) string {
	sum := sha256.Sum256([]byte(title + "\n" + excerpt))
	return hex.EncodeToString(sum[:])
}

// Rows are added a page at a time, so that a feed of a thousand items is
// not one statement.
const itemsPage = 100

// Items kept for the first time: seen, or waiting. An item already kept is
// left as it is.
const itemsAddHead = `
INSERT IGNORE INTO website_items (
    website_id,
    item_key,
    content_hash,
    state,
    created_at,
    updated_at
)
VALUES `

// Known is an item already kept.
type Known struct {
	State       string
	NodeID      int64
	ContentHash string
}

// ItemsInterface is what is kept of the items of each website's feed.
type ItemsInterface interface {
	MarkSeen(ctx context.Context, websiteID int64, items []ItemSeenAs) error
	// Known is which of these items are kept already, by key.
	Known(ctx context.Context, websiteID int64, keys []string) (map[string]Known, error)
	// AddWaiting keeps new items, to be posted as the hourly cap allows.
	AddWaiting(ctx context.Context, websiteID int64, items []ItemSeenAs) error
	// Posted says what an item became.
	Posted(ctx context.Context, websiteID int64, key string, nodeID int64) error
	// Changed keeps what a changed item says now.
	Changed(ctx context.Context, websiteID int64, key, contentHash string) error
	// Waiting is how many of its items wait to be posted.
	Waiting(ctx context.Context, websiteID int64) (int, error)
}

const itemsWaiting = `
SELECT COUNT(*)
FROM website_items
WHERE website_id = ?
  AND state = 'waiting'`

const itemsKnownHead = `
SELECT
    item_key,
    state,
    COALESCE(node_id, 0),
    content_hash
FROM website_items
WHERE website_id = ?
  AND item_key IN (`

const itemPosted = `
UPDATE website_items
SET state = 'posted',
    node_id = ?,
    updated_at = ?
WHERE website_id = ?
  AND item_key = ?`

const itemChanged = `
UPDATE website_items
SET content_hash = ?,
    updated_at = ?
WHERE website_id = ?
  AND item_key = ?`

type itemsRepo struct {
	db  *sql.DB
	now func() time.Time
}

func NewItems(db *sql.DB) ItemsInterface {
	return &itemsRepo{db: db, now: time.Now}
}

// MarkSeen keeps items as seen and not to be posted. An item already kept
// is left as it is.
func (r *itemsRepo) MarkSeen(ctx context.Context, websiteID int64, items []ItemSeenAs) error {
	return r.add(ctx, itemsAddHead, ItemSeen, websiteID, items)
}

func (r *itemsRepo) AddWaiting(ctx context.Context, websiteID int64, items []ItemSeenAs) error {
	return r.add(ctx, itemsAddHead, ItemWaiting, websiteID, items)
}

func (r *itemsRepo) Known(ctx context.Context, websiteID int64, keys []string) (map[string]Known, error) {
	known := map[string]Known{}
	for start := 0; start < len(keys); start += itemsPage {
		page := keys[start:min(start+itemsPage, len(keys))]
		marks := strings.TrimSuffix(strings.Repeat("?, ", len(page)), ", ")
		args := make([]any, 0, len(page)+1)
		args = append(args, websiteID)
		for _, key := range page {
			args = append(args, key)
		}

		rows, err := r.db.QueryContext(ctx, itemsKnownHead+marks+")", args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var (
				key  string
				item Known
			)
			if err := rows.Scan(&key, &item.State, &item.NodeID, &item.ContentHash); err != nil {
				rows.Close()
				return nil, err
			}
			known[key] = item
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return known, nil
}

func (r *itemsRepo) Posted(ctx context.Context, websiteID int64, key string, nodeID int64) error {
	_, err := r.db.ExecContext(ctx, itemPosted, nodeID, r.now().UTC(), websiteID, key)
	return err
}

func (r *itemsRepo) Changed(ctx context.Context, websiteID int64, key, contentHash string) error {
	_, err := r.db.ExecContext(ctx, itemChanged, contentHash, r.now().UTC(), websiteID, key)
	return err
}

func (r *itemsRepo) add(ctx context.Context, head, state string, websiteID int64, items []ItemSeenAs) error {
	now := r.now().UTC()
	for start := 0; start < len(items); start += itemsPage {
		end := min(start+itemsPage, len(items))
		page := items[start:end]

		rows := make([]string, len(page))
		args := make([]any, 0, len(page)*6)
		for i, item := range page {
			rows[i] = "(?, ?, ?, ?, ?, ?)"
			args = append(args, websiteID, item.Key, item.ContentHash, state, now, now)
		}
		if _, err := r.db.ExecContext(ctx, head+strings.Join(rows, ",\n       "), args...); err != nil {
			return err
		}
	}
	return nil
}

func (r *itemsRepo) Waiting(ctx context.Context, websiteID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, itemsWaiting, websiteID).Scan(&count)
	return count, err
}
