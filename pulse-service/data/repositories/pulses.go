package repositories

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// ErrFeedTaken is a feed that already has a Pulse.
var ErrFeedTaken = errors.New("feed already connected")

// The states of a Pulse.
const (
	StateActive = "active"
	StatePaused = "paused"
)

// Pulse is how a Pulse's feed is read: a row of pulses.
type Pulse struct {
	NodeID   int64
	FeedURL  string
	SiteURL  string
	DomainID int64
	State    string
}

// FeedHash is what a feed is kept unique by: its address, which may be
// longer than an index takes, hashed.
func FeedHash(feedURL string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(feedURL)))
	return hex.EncodeToString(sum[:])
}

const pulseAdd = `
INSERT INTO pulses (
    node_id,
    feed_url,
    feed_hash,
    site_url,
    domain_id,
    state,
    next_read_at,
    created_at,
    updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

const pulseFeedTaken = `
SELECT COUNT(*)
FROM pulses
WHERE feed_hash = ?`

// PulsesInterface is what is kept of each Pulse's feed.
type PulsesInterface interface {
	Add(ctx context.Context, p *Pulse) error
	FeedTaken(ctx context.Context, feedURL string) (bool, error)
}

type pulsesRepo struct {
	db  *sql.DB
	now func() time.Time
}

func NewPulses(db *sql.DB) PulsesInterface {
	return &pulsesRepo{db: db, now: time.Now}
}

// Add keeps a new Pulse's feed, to be read from the next hour: what it
// holds now has been marked seen.
func (r *pulsesRepo) Add(ctx context.Context, p *Pulse) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, pulseAdd,
		p.NodeID,
		p.FeedURL,
		FeedHash(p.FeedURL),
		p.SiteURL,
		p.DomainID,
		StateActive,
		now.Add(firstRead),
		now,
		now,
	)
	if isDuplicate(err) {
		return ErrFeedTaken
	}
	return err
}

func (r *pulsesRepo) FeedTaken(ctx context.Context, feedURL string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, pulseFeedTaken, FeedHash(feedURL)).Scan(&count)
	return count > 0, err
}

// A new Pulse's feed is next read this long after it is made: whatever is
// published in the meantime is new.
const firstRead = 15 * time.Minute

func isDuplicate(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Error 1062")
}
