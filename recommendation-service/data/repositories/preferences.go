package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
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
	SuggestEvents bool `json:"suggest_events"`
	// That what they liked and wrote may be used to choose for them.
	UseActivity bool `json:"use_activity"`
	// That they may be suggested to other people.
	Discoverable bool `json:"discoverable"`
	// How much each reason counts for when choosing for them, by the
	// reason's name, where they have set it. Nil for the service's own.
	Weights map[string]float64 `json:"-"`
}

// DefaultPreferences is what somebody has who has chosen nothing.
func DefaultPreferences() *Preferences {
	return &Preferences{
		SuggestPeople: true,
		SuggestGroups: true,
		SuggestEvents: true,
		UseActivity:   true,
		Discoverable:  true,
	}
}

const preferencesRead = `
SELECT suggest_people, suggest_groups, suggest_events, use_activity, discoverable, weights
FROM recommendation_preferences
WHERE person_id = ?`

const preferencesWrite = `
INSERT INTO recommendation_preferences
    (person_id, suggest_people, suggest_groups, suggest_events, use_activity, discoverable, weights, updated_at)
VALUES
    (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    suggest_people = VALUES(suggest_people),
    suggest_groups = VALUES(suggest_groups),
    suggest_events = VALUES(suggest_events),
    use_activity = VALUES(use_activity),
    discoverable = VALUES(discoverable),
    weights = VALUES(weights),
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
	var weights string
	err := r.db.QueryRowContext(ctx, preferencesRead, personID).
		Scan(&prefs.SuggestPeople, &prefs.SuggestGroups, &prefs.SuggestEvents, &prefs.UseActivity, &prefs.Discoverable, &weights)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultPreferences(), nil
	}
	if err != nil {
		return nil, err
	}

	// Weights that cannot be read are no weights: the service's own.
	if weights != "" {
		_ = json.Unmarshal([]byte(weights), &prefs.Weights)
	}

	return prefs, nil
}

func (r *PreferencesRepo) Write(personID int64, prefs *Preferences) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	weights := ""
	if len(prefs.Weights) > 0 {
		raw, err := json.Marshal(prefs.Weights)
		if err != nil {
			return err
		}
		weights = string(raw)
	}

	_, err := r.db.ExecContext(ctx, preferencesWrite,
		personID, prefs.SuggestPeople, prefs.SuggestGroups, prefs.SuggestEvents, prefs.UseActivity, prefs.Discoverable, weights, time.Now().UTC())

	return err
}

func (r *PreferencesRepo) Forget(personID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, preferencesForget, personID)

	return err
}
