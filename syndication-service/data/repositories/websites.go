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

// ErrFeedTaken is a feed that already has a website.
var ErrFeedTaken = errors.New("feed already connected")

// The states of a website.
const (
	StateActive = "active"
	StatePaused = "paused"
)

// Website is how a website's feed is read: a row of websites.
type Website struct {
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

const websiteAdd = `
INSERT INTO websites (
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

const websiteFeedTaken = `
SELECT COUNT(*)
FROM websites
WHERE feed_hash = ?`

// Due is a website whose feed is to be read now, as a worker took it.
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

// WebsitesInterface is what is kept of each website's feed.
type WebsitesInterface interface {
	Add(ctx context.Context, p *Website) error
	FeedTaken(ctx context.Context, feedURL string) (bool, error)

	// Claim takes the website whose feed is most overdue, for one worker
	// alone; nil when none is due.
	Claim(ctx context.Context) (*Due, error)
	// Read keeps how reading went and lets the website go.
	Read(ctx context.Context, nodeID int64, read Read) error
	// Failed counts a read that failed and puts the next one off.
	Failed(ctx context.Context, nodeID int64, failures int, next time.Time, why string) error
	// Pause stops a website's feed being read, saying why.
	Pause(ctx context.Context, nodeID int64, why string) error
	// PauseForProof pauses the websites resting on a domain whose proof is
	// gone; ResumeAfterProof starts again those it paused.
	PauseForProof(ctx context.Context, domainID int64) error
	ResumeAfterProof(ctx context.Context, domainID int64) error

	// Hub says a subscription to a hub was asked for, to be asked again
	// at askAgain unless the hub checks it first (HubVerified), which puts
	// that off to when the hub's lease ends.
	Hub(ctx context.Context, nodeID int64, hubURL string, askAgain time.Time) error
	HubVerified(ctx context.Context, nodeID int64, until time.Time) error
	// Feed is the feed a website reads and the hub it asked, for a hub's
	// check.
	Feed(ctx context.Context, nodeID int64) (feedURL, hubURL string, err error)
	// PausedFor is the websites paused for this reason, a page at a time;
	// Resume starts one again, its feed read at once.
	PausedFor(ctx context.Context, why string, limit int) ([]int64, error)
	Resume(ctx context.Context, nodeID int64) error
	// ReadSoon brings the next read forward to now, as a hub's word that
	// something was published does: at most once a minute.
	ReadSoon(ctx context.Context, nodeID int64) error
}

// Why a website was paused, in last_error.
const (
	// Its domain's proof has been gone a week.
	PausedProofGone = "proof_gone"
	// Its profile is gone or switched off.
	PausedProfileGone = "profile_gone"
)

// A claim is taken back from a worker that took it this long ago and never
// let it go: it stopped while reading.
const claimHeld = 10 * time.Minute

// The most overdue feed of an active website that no worker holds.
const websiteNextDue = `
SELECT node_id
FROM websites
WHERE state = 'active'
  AND next_read_at <= ?
  AND (claimed_at IS NULL OR claimed_at < ?)
ORDER BY next_read_at
LIMIT 1
FOR UPDATE SKIP LOCKED`

const websiteClaim = `
UPDATE websites
SET claimed_at = ?
WHERE node_id = ?`

const websiteDue = `
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
FROM websites
WHERE node_id = ?`

const websiteHub = `
UPDATE websites
SET hub_url = ?,
    hub_until = ?,
    updated_at = ?
WHERE node_id = ?`

const websiteHubVerified = `
UPDATE websites
SET hub_until = ?,
    updated_at = ?
WHERE node_id = ?
  AND hub_url <> ''`

const websitesPausedFor = `
SELECT node_id
FROM websites
WHERE state = 'paused'
  AND last_error = ?
ORDER BY updated_at
LIMIT ?`

const websiteResume = `
UPDATE websites
SET state = 'active',
    last_error = '',
    next_read_at = ?,
    updated_at = ?
WHERE node_id = ?
  AND state = 'paused'`

const websiteFeed = `
SELECT
    feed_url,
    hub_url
FROM websites
WHERE node_id = ?`

// Not a website that is paused, and not one read in the last minute: a hub
// that says so over and over brings nothing forward.
const websiteReadSoon = `
UPDATE websites
SET next_read_at = ?,
    updated_at = ?
WHERE node_id = ?
  AND state = 'active'
  AND (read_at IS NULL OR read_at < ?)`

const websiteRead = `
UPDATE websites
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

const websiteFailed = `
UPDATE websites
SET failures = ?,
    next_read_at = ?,
    last_error = ?,
    claimed_at = NULL,
    updated_at = ?
WHERE node_id = ?`

const websitePause = `
UPDATE websites
SET state = 'paused',
    last_error = ?,
    claimed_at = NULL,
    updated_at = ?
WHERE node_id = ?`

const websitePauseForProof = `
UPDATE websites
SET state = 'paused',
    last_error = 'proof_gone',
    updated_at = ?
WHERE domain_id = ?
  AND state = 'active'`

// Only what the proof paused: a website paused for another reason stays so.
const websiteResumeAfterProof = `
UPDATE websites
SET state = 'active',
    last_error = '',
    next_read_at = ?,
    updated_at = ?
WHERE domain_id = ?
  AND state = 'paused'
  AND last_error = 'proof_gone'`

type websitesRepo struct {
	db  *sql.DB
	now func() time.Time
}

func NewWebsites(db *sql.DB) WebsitesInterface {
	return &websitesRepo{db: db, now: time.Now}
}

// Add keeps a new website's feed, to be read from the next hour: what it
// holds now has been marked seen.
func (r *websitesRepo) Add(ctx context.Context, p *Website) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, websiteAdd,
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

func (r *websitesRepo) FeedTaken(ctx context.Context, feedURL string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, websiteFeedTaken, FeedHash(feedURL)).Scan(&count)
	return count > 0, err
}

// A new website's feed is read at once, to post its first items.
const firstRead = 0

func isDuplicate(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Error 1062")
}

func (r *websitesRepo) Claim(ctx context.Context) (*Due, error) {
	now := r.now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var nodeID int64
	err = tx.QueryRowContext(ctx, websiteNextDue, now, now.Add(-claimHeld)).Scan(&nodeID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, websiteClaim, now, nodeID); err != nil {
		return nil, err
	}

	var (
		due      Due
		began    sql.NullTime
		hubUntil sql.NullTime
	)
	err = tx.QueryRowContext(ctx, websiteDue, nodeID).Scan(
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

func (r *websitesRepo) Read(ctx context.Context, nodeID int64, read Read) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, websiteRead,
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

func (r *websitesRepo) Failed(ctx context.Context, nodeID int64, failures int, next time.Time, why string) error {
	if len(why) > 500 {
		why = why[:500]
	}
	_, err := r.db.ExecContext(ctx, websiteFailed, failures, next.UTC(), why, r.now().UTC(), nodeID)
	return err
}

func (r *websitesRepo) Pause(ctx context.Context, nodeID int64, why string) error {
	_, err := r.db.ExecContext(ctx, websitePause, why, r.now().UTC(), nodeID)
	return err
}

func (r *websitesRepo) PauseForProof(ctx context.Context, domainID int64) error {
	_, err := r.db.ExecContext(ctx, websitePauseForProof, r.now().UTC(), domainID)
	return err
}

func (r *websitesRepo) ResumeAfterProof(ctx context.Context, domainID int64) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, websiteResumeAfterProof, now, now, domainID)
	return err
}

func (r *websitesRepo) Hub(ctx context.Context, nodeID int64, hubURL string, askAgain time.Time) error {
	_, err := r.db.ExecContext(ctx, websiteHub, hubURL, askAgain.UTC(), r.now().UTC(), nodeID)
	return err
}

func (r *websitesRepo) HubVerified(ctx context.Context, nodeID int64, until time.Time) error {
	_, err := r.db.ExecContext(ctx, websiteHubVerified, until.UTC(), r.now().UTC(), nodeID)
	return err
}

func (r *websitesRepo) Feed(ctx context.Context, nodeID int64) (string, string, error) {
	var feedURL, hubURL string
	err := r.db.QueryRowContext(ctx, websiteFeed, nodeID).Scan(&feedURL, &hubURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return feedURL, hubURL, err
}

func (r *websitesRepo) ReadSoon(ctx context.Context, nodeID int64) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, websiteReadSoon, now, now, nodeID, now.Add(-time.Minute))
	return err
}

func (r *websitesRepo) PausedFor(ctx context.Context, why string, limit int) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, websitesPausedFor, why, limit)
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

func (r *websitesRepo) Resume(ctx context.Context, nodeID int64) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, websiteResume, now, now, nodeID)
	return err
}
