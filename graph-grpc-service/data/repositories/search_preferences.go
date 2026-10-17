package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"graph/ranking"
)

// Each person's own steps for search's Most relevant (ranking.Adjustable,
// ranking.Steps), as they have their own for recommendations (the user,
// 2026-10-16). Only what they have set is kept; the rest is as the
// installation has it.

const readSearchPreferences = `
SELECT weights
FROM search_preferences
WHERE person_id = ?`

const saveSearchPreferences = `
INSERT INTO search_preferences (person_id, weights, updated_at)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE
    weights = VALUES(weights),
    updated_at = VALUES(updated_at)`

// ReadPreferences is somebody's own steps: nothing for somebody who has set
// none, or who is not signed in.
func (repo *SearchRepo) ReadPreferences(personID int64) (map[string]float64, error) {
	steps := map[string]float64{}
	if personID <= 0 {
		return steps, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	var raw string
	err := repo.db.QueryRowContext(ctx, readSearchPreferences, personID).Scan(&raw)
	if err == sql.ErrNoRows || raw == "" {
		return steps, nil
	}
	if err != nil {
		return steps, err
	}
	_ = json.Unmarshal([]byte(raw), &steps)
	return kept(steps), nil
}

// SavePreferences keeps somebody's steps: only names that may be set, to
// values that are steps, and only those not as the installation has it.
func (repo *SearchRepo) SavePreferences(personID int64, steps map[string]float64) (map[string]float64, error) {
	steps = kept(steps)

	raw, err := json.Marshal(steps)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	if _, err := repo.db.ExecContext(ctx, saveSearchPreferences, personID, string(raw), time.Now()); err != nil {
		return nil, err
	}
	return steps, nil
}

// kept is what of somebody's steps is kept: what may be set, to a step,
// and not 1, which is as the installation has it.
func kept(steps map[string]float64) map[string]float64 {
	out := map[string]float64{}
	for _, name := range ranking.Adjustable {
		step, set := steps[name]
		if set && ranking.IsStep(step) && step != 1 {
			out[name] = step
		}
	}
	return out
}
