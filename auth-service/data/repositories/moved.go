package repositories

import (
	"context"
	"database/sql"
	"errors"
)

// Whether somebody moved their account away (anahita-web roadmap row 124,
// part 3): activitypub-service keeps it, in activitypub_accounts.moved_to,
// and this reads it to withhold from their tokens what posts, likes and
// follows (constants.OAuthScopesWithheldWhenMoved). Read only.

// MovedInterface says whether a person's account moved away.
type MovedInterface interface {
	HasMoved(personID int64) (bool, error)
}

type Moved struct {
	db *sql.DB
}

func NewMoved(db *sql.DB) *Moved {
	return &Moved{db: db}
}

const hasMoved = `
SELECT moved_to IS NOT NULL
FROM activitypub_accounts
WHERE person_id = ?`

func (repo *Moved) HasMoved(personID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var moved bool
	err := repo.db.QueryRowContext(ctx, hasMoved, personID).Scan(&moved)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return moved, err
}
