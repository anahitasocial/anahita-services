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

// ErrFeedTaken is a feed that already has a channel.
var ErrFeedTaken = errors.New("feed already connected")

// The states of a channel.
const (
	StateActive = "active"
	StatePaused = "paused"
)

// Channel is how a channel's feed is read: a row of channels.
type Channel struct {
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

const channelAdd = `
INSERT INTO channel_feeds (
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

const channelFeedTaken = `
SELECT COUNT(*)
FROM channel_feeds
WHERE feed_hash = ?`

// Due is a channel whose feed is to be read now, as a worker took it.
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

// ChannelsInterface is what is kept of each channel's feed.
type ChannelsInterface interface {
	Add(ctx context.Context, p *Channel) error
	FeedTaken(ctx context.Context, feedURL string) (bool, error)

	// Claim takes the channel whose feed is most overdue, for one worker
	// alone; nil when none is due.
	Claim(ctx context.Context) (*Due, error)
	// Read keeps how reading went and lets the channel go.
	Read(ctx context.Context, nodeID int64, read Read) error
	// Failed counts a read that failed and puts the next one off.
	Failed(ctx context.Context, nodeID int64, failures int, next time.Time, why string) error
	// Pause stops a channel's feed being read, saying why.
	Pause(ctx context.Context, nodeID int64, why string) error
	// PauseForProof pauses the channels resting on a domain whose proof is
	// gone; ResumeAfterProof starts again those it paused.
	PauseForProof(ctx context.Context, domainID int64) error
	ResumeAfterProof(ctx context.Context, domainID int64) error

	// Hub says a subscription to a hub was asked for, to be asked again
	// at askAgain unless the hub checks it first (HubVerified), which puts
	// that off to when the hub's lease ends.
	Hub(ctx context.Context, nodeID int64, hubURL string, askAgain time.Time) error
	HubVerified(ctx context.Context, nodeID int64, until time.Time) error
	// Feed is the feed a channel reads and the hub it asked, for a hub's
	// check.
	Feed(ctx context.Context, nodeID int64) (feedURL, hubURL string, err error)
	// PausedFor is the channels paused for this reason, a page at a time;
	// Resume starts one again, its feed read at once.
	PausedFor(ctx context.Context, why string, limit int) ([]int64, error)
	Resume(ctx context.Context, nodeID int64) error
	// ReadSoon brings the next read forward to now, as a hub's word that
	// something was published does: at most once a minute.
	ReadSoon(ctx context.Context, nodeID int64) error
}

// Why a channel was paused, in last_error.
const (
	// Its domain's proof has been gone a week.
	PausedProofGone = "proof_gone"
	// Its profile is gone or switched off.
	PausedProfileGone = "profile_gone"
)

// A claim is taken back from a worker that took it this long ago and never
// let it go: it stopped while reading.
const claimHeld = 10 * time.Minute

// The most overdue feed of an active channel that no worker holds.
const channelNextDue = `
SELECT node_id
FROM channel_feeds
WHERE state = 'active'
  AND next_read_at <= ?
  AND (claimed_at IS NULL OR claimed_at < ?)
ORDER BY next_read_at
LIMIT 1
FOR UPDATE SKIP LOCKED`

const channelClaim = `
UPDATE channel_feeds
SET claimed_at = ?
WHERE node_id = ?`

const channelDue = `
SELECT
    node_id,
    feed_url,
    etag,
    last_modified,
    failures,
    hour_posts,
    hour_began_at,
    hub_url,
    hub_until
FROM channel_feeds
WHERE node_id = ?`

const channelHub = `
UPDATE channel_feeds
SET hub_url = ?,
    hub_until = ?,
    updated_at = ?
WHERE node_id = ?`

const channelHubVerified = `
UPDATE channel_feeds
SET hub_until = ?,
    updated_at = ?
WHERE node_id = ?
  AND hub_url <> ''`

const channelsPausedFor = `
SELECT node_id
FROM channel_feeds
WHERE state = 'paused'
  AND last_error = ?
ORDER BY updated_at
LIMIT ?`

const channelResume = `
UPDATE channel_feeds
SET state = 'active',
    last_error = '',
    next_read_at = ?,
    updated_at = ?
WHERE node_id = ?
  AND state = 'paused'`

const channelFeed = `
SELECT
    feed_url,
    hub_url
FROM channel_feeds
WHERE node_id = ?`

// Not a channel that is paused, and not one read in the last minute: a hub
// that says so over and over brings nothing forward.
const channelReadSoon = `
UPDATE channel_feeds
SET next_read_at = ?,
    updated_at = ?
WHERE node_id = ?
  AND state = 'active'
  AND (read_at IS NULL OR read_at < ?)`

const channelRead = `
UPDATE channel_feeds
SET etag = ?,
    last_modified = ?,
    hour_posts = ?,
    hour_began_at = ?,
    next_read_at = ?,
    read_at = ?,
    failures = 0,
    last_error = '',
    claimed_at = NULL,
    updated_at = ?
WHERE node_id = ?`

const channelFailed = `
UPDATE channel_feeds
SET failures = ?,
    next_read_at = ?,
    last_error = ?,
    claimed_at = NULL,
    updated_at = ?
WHERE node_id = ?`

const channelPause = `
UPDATE channel_feeds
SET state = 'paused',
    last_error = ?,
    claimed_at = NULL,
    updated_at = ?
WHERE node_id = ?`

const channelPauseForProof = `
UPDATE channel_feeds
SET state = 'paused',
    last_error = 'proof_gone',
    updated_at = ?
WHERE domain_id = ?
  AND state = 'active'`

// Only what the proof paused: a channel paused for another reason stays so.
const channelResumeAfterProof = `
UPDATE channel_feeds
SET state = 'active',
    last_error = '',
    next_read_at = ?,
    updated_at = ?
WHERE domain_id = ?
  AND state = 'paused'
  AND last_error = 'proof_gone'`

type channelsRepo struct {
	db  *sql.DB
	now func() time.Time
}

func NewChannels(db *sql.DB) ChannelsInterface {
	return &channelsRepo{db: db, now: time.Now}
}

// Add keeps a new channel's feed, to be read from the next hour: what it
// holds now has been marked seen.
func (r *channelsRepo) Add(ctx context.Context, p *Channel) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, channelAdd,
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

func (r *channelsRepo) FeedTaken(ctx context.Context, feedURL string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, channelFeedTaken, FeedHash(feedURL)).Scan(&count)
	return count > 0, err
}

// A new channel's feed is read at once, to post its first items.
const firstRead = 0

func isDuplicate(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Error 1062")
}

func (r *channelsRepo) Claim(ctx context.Context) (*Due, error) {
	now := r.now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var nodeID int64
	err = tx.QueryRowContext(ctx, channelNextDue, now, now.Add(-claimHeld)).Scan(&nodeID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, channelClaim, now, nodeID); err != nil {
		return nil, err
	}

	var (
		due      Due
		began    sql.NullTime
		hubUntil sql.NullTime
	)
	err = tx.QueryRowContext(ctx, channelDue, nodeID).Scan(
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

func (r *channelsRepo) Read(ctx context.Context, nodeID int64, read Read) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, channelRead,
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

func (r *channelsRepo) Failed(ctx context.Context, nodeID int64, failures int, next time.Time, why string) error {
	if len(why) > 500 {
		why = why[:500]
	}
	_, err := r.db.ExecContext(ctx, channelFailed, failures, next.UTC(), why, r.now().UTC(), nodeID)
	return err
}

func (r *channelsRepo) Pause(ctx context.Context, nodeID int64, why string) error {
	_, err := r.db.ExecContext(ctx, channelPause, why, r.now().UTC(), nodeID)
	return err
}

func (r *channelsRepo) PauseForProof(ctx context.Context, domainID int64) error {
	_, err := r.db.ExecContext(ctx, channelPauseForProof, r.now().UTC(), domainID)
	return err
}

func (r *channelsRepo) ResumeAfterProof(ctx context.Context, domainID int64) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, channelResumeAfterProof, now, now, domainID)
	return err
}

func (r *channelsRepo) Hub(ctx context.Context, nodeID int64, hubURL string, askAgain time.Time) error {
	_, err := r.db.ExecContext(ctx, channelHub, hubURL, askAgain.UTC(), r.now().UTC(), nodeID)
	return err
}

func (r *channelsRepo) HubVerified(ctx context.Context, nodeID int64, until time.Time) error {
	_, err := r.db.ExecContext(ctx, channelHubVerified, until.UTC(), r.now().UTC(), nodeID)
	return err
}

func (r *channelsRepo) Feed(ctx context.Context, nodeID int64) (string, string, error) {
	var feedURL, hubURL string
	err := r.db.QueryRowContext(ctx, channelFeed, nodeID).Scan(&feedURL, &hubURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return feedURL, hubURL, err
}

func (r *channelsRepo) ReadSoon(ctx context.Context, nodeID int64) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, channelReadSoon, now, now, nodeID, now.Add(-time.Minute))
	return err
}

func (r *channelsRepo) PausedFor(ctx context.Context, why string, limit int) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, channelsPausedFor, why, limit)
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

func (r *channelsRepo) Resume(ctx context.Context, nodeID int64) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, channelResume, now, now, nodeID)
	return err
}
