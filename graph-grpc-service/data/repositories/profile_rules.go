package repositories

import (
	"anahita-libs/constants"
	"context"
	"database/sql"
	"errors"
	"time"
)

// A group's and an event's rules (roadmap row 102, part 5): written by its
// administrators, at most ten, ticked one by one by whoever follows it or
// answers it, and ticked again when they change. The version goes up with
// every change, and what each person agreed to is kept beside it.

// ProfileRulesMax is the most rules a profile may have: every line of a
// checklist is to be read (the user, 2026-10-19).
const ProfileRulesMax = 10

// The longest a rule's title and its line of explanation may be.
const (
	ProfileRuleTitleMax = 100
	ProfileRuleBodyMax  = 300
)

// ErrTooManyRules is a list longer than ProfileRulesMax.
var ErrTooManyRules = errors.New("a profile has at most ten rules")

// ProfileRule is one rule: a short title, and a line more if it needs one.
type ProfileRule struct {
	Title string
	Body  string
}

// ProfileRules is a profile's rules, in order, and the version they are at.
// Version 0 with none is a profile that never had any.
type ProfileRules struct {
	ProfileID int64
	Version   int64
	UpdatedAt *time.Time
	Items     []ProfileRule
}

type ProfileRulesRepoInterface interface {
	// Read is a profile's rules as they are now.
	Read(profileID int64) (*ProfileRules, error)
	// Set replaces them, in the order given, and raises their version.
	Set(profileID int64, items []ProfileRule) (*ProfileRules, error)
	// Agree records that a person agreed to a version.
	Agree(personID, profileID, version int64) error
	// Agreed is the version a person last agreed to, or 0.
	Agreed(personID, profileID int64) (int64, error)
}

func NewProfileRulesRepo(db *sql.DB) ProfileRulesRepoInterface {
	return &profileRulesRepo{db: db, dbTimeout: constants.DBTimeout}
}

type profileRulesRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
}

const readProfileRuleVersion = `
SELECT
  version,
  updated_at
FROM profile_rule_versions
WHERE profile_id = ?`

const readProfileRuleItems = `
SELECT
  title,
  body
FROM profile_rules
WHERE profile_id = ?
ORDER BY position`

func (r *profileRulesRepo) Read(profileID int64) (*ProfileRules, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	rules := &ProfileRules{ProfileID: profileID}
	var updated sql.NullTime
	err := r.db.QueryRowContext(ctx, readProfileRuleVersion, profileID).Scan(&rules.Version, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return rules, nil
	}
	if err != nil {
		return nil, err
	}
	if updated.Valid {
		rules.UpdatedAt = &updated.Time
	}

	rows, err := r.db.QueryContext(ctx, readProfileRuleItems, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item ProfileRule
		if err := rows.Scan(&item.Title, &item.Body); err != nil {
			return nil, err
		}
		rules.Items = append(rules.Items, item)
	}
	return rules, rows.Err()
}

const deleteProfileRuleItems = `
DELETE FROM profile_rules
WHERE profile_id = ?`

const addProfileRuleItem = `
INSERT INTO profile_rules (
  profile_id,
  position,
  title,
  body
) VALUES (?, ?, ?, ?)`

const raiseProfileRuleVersion = `
INSERT INTO profile_rule_versions (
  profile_id,
  version,
  updated_at
) VALUES (?, 1, NOW())
ON DUPLICATE KEY UPDATE
  version = version + 1,
  updated_at = NOW()`

// Set replaces a profile's rules in one transaction: an empty list takes
// them all away, which is itself a change, and raises the version too.
func (r *profileRulesRepo) Set(profileID int64, items []ProfileRule) (*ProfileRules, error) {
	if len(items) > ProfileRulesMax {
		return nil, ErrTooManyRules
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, deleteProfileRuleItems, profileID); err != nil {
		return nil, err
	}
	for position, item := range items {
		if _, err := tx.ExecContext(ctx, addProfileRuleItem, profileID, position, item.Title, item.Body); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, raiseProfileRuleVersion, profileID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.Read(profileID)
}

const agreeProfileRules = `
INSERT INTO profile_rule_agreements (
  person_id,
  profile_id,
  version,
  created_at
) VALUES (?, ?, ?, NOW())
ON DUPLICATE KEY UPDATE
  version = VALUES(version),
  created_at = NOW()`

func (r *profileRulesRepo) Agree(personID, profileID, version int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, agreeProfileRules, personID, profileID, version)
	return err
}

const readProfileRulesAgreed = `
SELECT
  version
FROM profile_rule_agreements
WHERE person_id = ?
  AND profile_id = ?`

func (r *profileRulesRepo) Agreed(personID, profileID int64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	var version int64
	err := r.db.QueryRowContext(ctx, readProfileRulesAgreed, personID, profileID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return version, err
}
