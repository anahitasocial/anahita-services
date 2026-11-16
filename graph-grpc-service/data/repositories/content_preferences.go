package repositories

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Each reader's choice of Show, Warn or Hide for each kind of content
// warning (roadmap row 122, phase 2). Only what is not Warn is kept; read,
// every kind is there (entities.WarningChoices).

type ContentPreferencesRepoInterface interface {
	// Read is a person's choice for every kind.
	Read(personID int64) (map[string]string, error)
	// Save replaces their choices, and is what they are now.
	Save(personID int64, choices map[string]string) (map[string]string, error)
}

func NewContentPreferencesRepo(db *sql.DB) ContentPreferencesRepoInterface {
	return &contentPreferencesRepo{db: db, dbTimeout: constants.DBTimeout}
}

type contentPreferencesRepo struct {
	db        *sql.DB
	dbTimeout time.Duration
}

const readContentPreferences = `
SELECT choices
FROM content_preferences
WHERE person_id = ?`

const saveContentPreferences = `
INSERT INTO content_preferences (person_id, choices, updated_at)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE
    choices = VALUES(choices),
    updated_at = VALUES(updated_at)`

func (repo *contentPreferencesRepo) Read(personID int64) (map[string]string, error) {
	choices := map[string]string{}
	if personID <= 0 {
		return entities.WarningChoices(choices), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	var raw string
	err := repo.db.QueryRowContext(ctx, readContentPreferences, personID).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &choices)
	}
	return entities.WarningChoices(choices), nil
}

func (repo *contentPreferencesRepo) Save(personID int64, choices map[string]string) (map[string]string, error) {
	raw, err := json.Marshal(entities.WarningChoicesKept(choices))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	if _, err := repo.db.ExecContext(ctx, saveContentPreferences, personID, string(raw), time.Now().UTC()); err != nil {
		return nil, err
	}
	return entities.WarningChoices(choices), nil
}
