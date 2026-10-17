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
	// Seen, and never to be posted: what the feed held when its Pulse was
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

const itemsSeenHead = `
INSERT IGNORE INTO pulse_items (
    pulse_id,
    item_key,
    content_hash,
    state,
    created_at,
    updated_at
)
VALUES `

// ItemsInterface is what is kept of the items of each Pulse's feed.
type ItemsInterface interface {
	MarkSeen(ctx context.Context, pulseID int64, items []ItemSeenAs) error
}

type itemsRepo struct {
	db  *sql.DB
	now func() time.Time
}

func NewItems(db *sql.DB) ItemsInterface {
	return &itemsRepo{db: db, now: time.Now}
}

// MarkSeen keeps items as seen and not to be posted. An item already kept
// is left as it is.
func (r *itemsRepo) MarkSeen(ctx context.Context, pulseID int64, items []ItemSeenAs) error {
	now := r.now().UTC()
	for start := 0; start < len(items); start += itemsPage {
		end := min(start+itemsPage, len(items))
		page := items[start:end]

		rows := make([]string, len(page))
		args := make([]any, 0, len(page)*6)
		for i, item := range page {
			rows[i] = "(?, ?, ?, ?, ?, ?)"
			args = append(args, pulseID, item.Key, item.ContentHash, ItemSeen, now, now)
		}
		if _, err := r.db.ExecContext(ctx, itemsSeenHead+strings.Join(rows, ",\n       "), args...); err != nil {
			return err
		}
	}
	return nil
}
