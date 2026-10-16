package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	an_const "anahita-libs/constants"

	"recommendation/constants"
)

// Preferences is what somebody has chosen about suggestions. Every choice is
// yes until they say otherwise.
type Preferences struct {
	// To be suggested people, and groups.
	SuggestPeople bool `json:"suggest_people"`
	// To be suggested groups.
	SuggestGroups bool `json:"suggest_groups"`
	// To be suggested events.
	SuggestEvents bool `json:"suggest_events"`
	// That what they liked and wrote may be used to choose for them.
	UseActivity bool `json:"use_activity"`
	// That they may be suggested to other people.
	Discoverable bool `json:"discoverable"`
	// How much each reason counts for when choosing for them, by the
	// reason's name, where they have set it. Nil for the service's own.
	Weights map[string]float64 `json:"-"`
	// How lately somebody must have been here to be suggested to them, in
	// days. Nothing is no limit.
	ActiveWithinDays int `json:"active_within_days"`
}

// Where ActiveWithinDays is kept: beside the weights, in the same column,
// as one more thing the ranking is tuned by. No reason has this name.
const activeWithinKey = "active_within_days"

// DefaultPreferences is what somebody has who has chosen nothing.
func DefaultPreferences() *Preferences {
	return &Preferences{
		SuggestPeople: true,
		SuggestGroups: true,
		SuggestEvents: true,
		UseActivity:   true,
		Discoverable:  true,

		ActiveWithinDays: constants.ActiveWithinDefault,
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
	prefs.ActiveWithinDays = constants.ActiveWithinDefault
	if weights != "" {
		_ = json.Unmarshal([]byte(weights), &prefs.Weights)
	}
	if days, set := prefs.Weights[activeWithinKey]; set {
		delete(prefs.Weights, activeWithinKey)
		if constants.IsActiveWithinChoice(int(days)) {
			prefs.ActiveWithinDays = int(days)
		}
	}
	if len(prefs.Weights) == 0 {
		prefs.Weights = nil
	}

	return prefs, nil
}

func (r *PreferencesRepo) Write(personID int64, prefs *Preferences) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	// Only what was chosen is kept: somebody on the service's own has
	// nothing here, and follows it when it changes.
	tuned := map[string]float64{}
	for reason, each := range prefs.Weights {
		tuned[reason] = each
	}
	if prefs.ActiveWithinDays != constants.ActiveWithinDefault {
		tuned[activeWithinKey] = float64(prefs.ActiveWithinDays)
	}

	weights := ""
	if len(tuned) > 0 {
		raw, err := json.Marshal(tuned)
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
