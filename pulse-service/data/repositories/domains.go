package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrNotFound is a row that is not there.
var ErrNotFound = errors.New("not found")

// Domain is a domain being proved, or proved: a row of verified_domains.
type Domain struct {
	ID       int64
	Domain   string
	Host     string
	Token    string
	PersonID int64
	// The profile it is proved for; 0 until that profile is made.
	ProfileID    int64
	Method       string
	VerifiedAt   *time.Time
	CheckedAt    *time.Time
	FailingSince *time.Time
	CreatedAt    time.Time
}

// Proved says whether the token has been found.
func (d *Domain) Proved() bool {
	return d.VerifiedAt != nil
}

const domainColumns = `
    id,
    domain,
    host,
    token,
    person_id,
    profile_id,
    method,
    verified_at,
    checked_at,
    failing_since,
    created_at`

const domainAdd = `
INSERT INTO verified_domains (
    domain,
    host,
    token,
    person_id,
    created_at,
    updated_at
)
VALUES (?, ?, ?, ?, ?, ?)`

const domainRead = `
SELECT` + domainColumns + `
FROM verified_domains
WHERE id = ?`

// The token was found: it is proved, and holds again if it was failing.
const domainProved = `
UPDATE verified_domains
SET method = ?,
    verified_at = COALESCE(verified_at, ?),
    checked_at = ?,
    failing_since = NULL,
    updated_at = ?
WHERE id = ?`

// The profile it was proved for has been made. Only once.
const domainForProfile = `
UPDATE verified_domains
SET profile_id = ?,
    updated_at = ?
WHERE id = ?
  AND profile_id IS NULL`

// DomainsInterface is what is kept of the domains people prove.
type DomainsInterface interface {
	Add(ctx context.Context, d *Domain) (int64, error)
	Read(ctx context.Context, id int64) (*Domain, error)
	Proved(ctx context.Context, id int64, method string) error
	ForProfile(ctx context.Context, id, profileID int64) error
}

type domainsRepo struct {
	db  *sql.DB
	now func() time.Time
}

func NewDomains(db *sql.DB) DomainsInterface {
	return &domainsRepo{db: db, now: time.Now}
}

func (r *domainsRepo) Add(ctx context.Context, d *Domain) (int64, error) {
	now := r.now().UTC()
	res, err := r.db.ExecContext(ctx, domainAdd, d.Domain, d.Host, d.Token, d.PersonID, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *domainsRepo) Read(ctx context.Context, id int64) (*Domain, error) {
	return scanDomain(r.db.QueryRowContext(ctx, domainRead, id))
}

func (r *domainsRepo) Proved(ctx context.Context, id int64, method string) error {
	now := r.now().UTC()
	_, err := r.db.ExecContext(ctx, domainProved, method, now, now, now, id)
	return err
}

func (r *domainsRepo) ForProfile(ctx context.Context, id, profileID int64) error {
	_, err := r.db.ExecContext(ctx, domainForProfile, profileID, r.now().UTC(), id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDomain(row scanner) (*Domain, error) {
	var (
		d          Domain
		profileID  sql.NullInt64
		method     sql.NullString
		verifiedAt sql.NullTime
		checkedAt  sql.NullTime
		failing    sql.NullTime
	)
	err := row.Scan(
		&d.ID,
		&d.Domain,
		&d.Host,
		&d.Token,
		&d.PersonID,
		&profileID,
		&method,
		&verifiedAt,
		&checkedAt,
		&failing,
		&d.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d.ProfileID = profileID.Int64
	d.Method = method.String
	d.VerifiedAt = timeOrNil(verifiedAt)
	d.CheckedAt = timeOrNil(checkedAt)
	d.FailingSince = timeOrNil(failing)
	return &d, nil
}

func timeOrNil(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
