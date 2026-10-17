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

// Due is a Pulse whose feed is to be read now, as a worker took it.
type Due struct {
	NodeID       int64
	FeedURL      string
	ETag         string
	LastModified string
	Failures     int
	// How many it posted in the hour that began at HourBeganAt.
	HourPosts   int
	HourBeganAt *time.Time
	// The WebSub hub it asked, and until when that holds: the end of the
	// hub's lease, or, until the hub checks it, when to ask again.
	HubURL   string
	HubUntil *time.Time
}

// Read is how reading a feed went, kept for the next time.
type Read struct {
	ETag         string
	LastModified string
	HourPosts    int
	HourBeganAt  *time.Time
	NextReadAt   time.Time
}

// PulsesInterface is what is kept of each Pulse's feed.
type PulsesInterface interface {
	Add(ctx context.Context, p *Pulse) error
	FeedTaken(ctx context.Context, feedURL string) (bool, error)

	// Claim takes the Pulse whose feed is most overdue, for one worker
	// alone; nil when none is due.
	Claim(ctx context.Context) (*Due, error)
	// Read keeps how reading went and lets the Pulse go.
	Read(ctx context.Context, nodeID int64, read Read) error
	// Failed counts a read that failed and puts the next one off.
	Failed(ctx context.Context, nodeID int64, failures int, next time.Time, why string) error
	// Pause stops a Pulse's feed being read, saying why.
	Pause(ctx context.Context, nodeID int64, why string) error
	// PauseForProof pauses the Pulses resting on a domain whose proof is
	// gone; ResumeAfterProof starts again those it paused.
	PauseForProof(ctx context.Context, domainID int64) error
	ResumeAfterProof(ctx context.Context, domainID int64) error

	// Hub says a subscription to a hub was asked for, to be asked again
	// at askAgain unless the hub checks it first (HubVerified), which puts
	// that off to when the hub's lease ends.
	Hub(ctx context.Context, nodeID int64, hubURL string, askAgain time.Time) error
	HubVerified(ctx context.Context, nodeID int64, until time.Time) error
	// Feed is the feed a Pulse reads and the hub it asked, for a hub's
	// check.
	Feed(ctx context.Context, nodeID int64) (feedURL, hubURL string, err error)
	// PausedFor is the Pulses paused for this reason, a page at a time;
	// Resume starts one again, its feed read at once.
	PausedFor(ctx context.Context, why string, limit int) ([]int64, error)
	Resume(ctx context.Context, nodeID int64) error
	// ReadSoon brings the next read forward to now, as a hub's word that
	// something was published does: at most once a minute.
	ReadSoon(ctx context.Context, nodeID int64) error
}

// Why a Pulse was paused, in last_error.
const (
	// Its domain's proof has been gone a week.
	PausedProofGone = "proof_gone"
	// Its profile is gone or switched off.
	PausedProfileGone = "profile_gone"
)

// A claim is taken back from a worker that took it this long ago and never
// let it go: it stopped while reading.
const claimHeld = 10 * time.Minute

// The most overdue feed of an active Pulse that no worker holds.
const pulseNextDue = `
SELECT node_id
FROM pulses
WHERE state = 'active'
  AND next_read_at <= ?
  AND (claimed_at IS NULL OR claimed_at < ?)
ORDER BY next_read_at
LIMIT 1
FOR UPDATE SKIP LOCKED`

const pulseClaim = `
UPDATE pulses
SET claimed_at = ?
WHERE node_id = ?`

const pulseDue = `
SELECT
    node_id,
    feed_url,
    COALESCE(etag, ''),
    COALESCE(last_modified, ''),
    failures,
    hour_posts,
    hour_began_at,
    COALESCE(hub_url, ''),
    hub_until
FROM pulses
WHERE node_id = ?`

const pulseHub = `
UPDATE pulses
SET hub_url = ?,
    hub_until = ?,
    updated_at = ?
WHERE node_id = ?`

const pulseHubVerified = `
UPDATE pulses
SET hub_until = ?,
    updated_at = ?
WHERE node_id = ?
  AND hub_url IS NOT NULL`

const pulsesPausedFor = `
SELECT node_id
FROM pulses
WHERE state = 'paused'
  AND last_error = ?
ORDER BY updated_at
LIMIT ?`

const pulseResume = `
UPDATE pulses
SET state = 'active',
    last_error = NULL,
    next_read_at = ?,
    updated_at = ?
WHERE node_id = ?
  AND state = 'paused'`

const pulseFeed = `
SELECT
    feed_url,
    COALESCE(hub_url, '')
FROM pulses
WHERE node_id = ?`

// Not a Pulse that is paused, and not one read in the last minute: a hub
// that says so over and over brings nothing forward.
const pulseReadSoon = `
UPDATE pulses
SET next_read_at = ?,
    updated_at = ?
WHERE node_id = ?
  AND state = 'active'
  AND (read_at IS NULL OR read_at < ?)`

const pulseRead = `
UPDATE pulses
SET etag = NULLIF(?, ''),
    last_modified = NULLIF(?, ''),
    hour_posts = ?,
    hour_began_at = ?,
    next_read_at = ?,
    read_at = ?,
    failures = 0,
    last_error = NULL,
    claimed_at = NULL,
    updated_at = ?
WHERE node_id = ?`

const pulseFailed = `
UPDATE pulses
SET failures = ?,
    next_read_at = ?,
    last_error = ?,
    claimed_at = NULL,
    updated_at = ?
WHERE node_id = ?`

const pulsePause = `
UPDATE pulses
SET state = 'paused',
    last_error = ?,
    claimed_at = NULL,
    updated_at = ?
WHERE node_id = ?`

const pulsePauseForProof = `
UPDATE pulses
SET state = 'paused',
    last_error = 'proof_gone',
    updated_at = ?
WHERE domain_id = ?
  AND state = 'active'`

// Only what the proof paused: a Pulse paused for another reason stays so.
const pulseResumeAfterProof = `
UPDATE pulses
SET state = 'active',
    last_error = NULL,
    next_read_at = ?,
    updated_at = ?
WHERE domain_id = ?
  AND state = 'paused'
  AND last_error = 'proof_gone'`

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

// A new Pulse's feed is read at once, to post its first items.
const firstRead = 0

func isDuplicate(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Error 1062")
}

func (r *pulsesRepo) Claim(ctx context.Context) (*Due, error) {
	now := r.now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var nodeID int64
	err = tx.QueryRowContext(ctx, pulseNextDue, now, now.Add(-claimHeld)).Scan(&nodeID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, pulseClaim, now, nodeID); err != nil {
		return nil, err
	}

	var (
		due      Due
		began    sql.NullTime
		hubUntil sql.NullTime
	)
	err = tx.QueryRowContext(ctx, pulseDue, nodeID).Scan(
		&due.NodeID,
		&due.FeedURL,
		&due.ETag,
		&due.LastModified,
		&due.Failures,
		&due.HourPosts,
		&began,
		&due.HubURL,
		&hubUntil,
	)
	if err != nil {
		return nil, err
	}
	due.HourBeganAt = timeOrNil(began)
	due.HubUntil = timeOrNil(hubUntil)

	return &due, tx.Commit()
}

func (r *pulsesRepo) Read(ctx context.Context, nodeID int64, read Read) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, pulseRead,
		read.ETag,
		read.LastModified,
		read.HourPosts,
		read.HourBeganAt,
		read.NextReadAt.UTC(),
		now,
		now,
		nodeID,
	)
	return err
}

func (r *pulsesRepo) Failed(ctx context.Context, nodeID int64, failures int, next time.Time, why string) error {
	if len(why) > 500 {
		why = why[:500]
	}
	_, err := r.db.ExecContext(ctx, pulseFailed, failures, next.UTC(), why, r.now().UTC(), nodeID)
	return err
}

func (r *pulsesRepo) Pause(ctx context.Context, nodeID int64, why string) error {
	_, err := r.db.ExecContext(ctx, pulsePause, why, r.now().UTC(), nodeID)
	return err
}

func (r *pulsesRepo) PauseForProof(ctx context.Context, domainID int64) error {
	_, err := r.db.ExecContext(ctx, pulsePauseForProof, r.now().UTC(), domainID)
	return err
}

func (r *pulsesRepo) ResumeAfterProof(ctx context.Context, domainID int64) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, pulseResumeAfterProof, now, now, domainID)
	return err
}

func (r *pulsesRepo) Hub(ctx context.Context, nodeID int64, hubURL string, askAgain time.Time) error {
	_, err := r.db.ExecContext(ctx, pulseHub, hubURL, askAgain.UTC(), r.now().UTC(), nodeID)
	return err
}

func (r *pulsesRepo) HubVerified(ctx context.Context, nodeID int64, until time.Time) error {
	_, err := r.db.ExecContext(ctx, pulseHubVerified, until.UTC(), r.now().UTC(), nodeID)
	return err
}

func (r *pulsesRepo) Feed(ctx context.Context, nodeID int64) (string, string, error) {
	var feedURL, hubURL string
	err := r.db.QueryRowContext(ctx, pulseFeed, nodeID).Scan(&feedURL, &hubURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return feedURL, hubURL, err
}

func (r *pulsesRepo) ReadSoon(ctx context.Context, nodeID int64) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, pulseReadSoon, now, now, nodeID, now.Add(-time.Minute))
	return err
}

func (r *pulsesRepo) PausedFor(ctx context.Context, why string, limit int) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, pulsesPausedFor, why, limit)
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

func (r *pulsesRepo) Resume(ctx context.Context, nodeID int64) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, pulseResume, now, now, nodeID)
	return err
}
