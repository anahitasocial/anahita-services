package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	an_const "anahita-libs/constants"
)

// Preferences is what somebody has chosen about suggestions. Every choice is
// yes until they say otherwise.
type Preferences struct {
	// To be suggested people, and groups.
	SuggestPeople bool `json:"suggest_people"`
	SuggestGroups bool `json:"suggest_groups"`
	// That what they liked and wrote may be used to choose for them.
	UseActivity bool `json:"use_activity"`
	// That they may be suggested to other people.
	Discoverable bool `json:"discoverable"`
}

// DefaultPreferences is what somebody has who has chosen nothing.
func DefaultPreferences() *Preferences {
	return &Preferences{
		SuggestPeople: true,
		SuggestGroups: true,
		UseActivity:   true,
		Discoverable:  true,
	}
}

const preferencesRead = `
SELECT suggest_people, suggest_groups, use_activity, discoverable
FROM recommendation_preferences
WHERE person_id = ?`

const preferencesWrite = `
INSERT INTO recommendation_preferences
    (person_id, suggest_people, suggest_groups, use_activity, discoverable, updated_at)
VALUES
    (?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    suggest_people = VALUES(suggest_people),
    suggest_groups = VALUES(suggest_groups),
    use_activity = VALUES(use_activity),
    discoverable = VALUES(discoverable),
    updated_at = VALUES(updated_at)`

const preferencesForget = `
DELETE FROM recommendation_preferences
WHERE person_id = ?`

type PreferencesInterface interface {
	Read(personID int64) (*Preferences, error)
	Write(personID int64, prefs *Preferences) error
	// Forget removes what somebody chose, when they are gone.
	Forget(personID int64) error
}

type PreferencesRepo struct {
	db      *sql.DB
	timeout time.Duration
}

func NewPreferences(db *sql.DB) PreferencesInterface {
	return &PreferencesRepo{db: db, timeout: an_const.DBTimeout}
}

func (r *PreferencesRepo) Read(personID int64) (*Preferences, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	prefs := &Preferences{}
	err := r.db.QueryRowContext(ctx, preferencesRead, personID).
		Scan(&prefs.SuggestPeople, &prefs.SuggestGroups, &prefs.UseActivity, &prefs.Discoverable)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultPreferences(), nil
	}
	if err != nil {
		return nil, err
	}

	return prefs, nil
}

func (r *PreferencesRepo) Write(personID int64, prefs *Preferences) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, preferencesWrite,
		personID, prefs.SuggestPeople, prefs.SuggestGroups, prefs.UseActivity, prefs.Discoverable, time.Now().UTC())

	return err
}

func (r *PreferencesRepo) Forget(personID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, preferencesForget, personID)

	return err
}
