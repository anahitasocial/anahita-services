package repositories

import (
	"context"
	"database/sql"
	"errors"

	"anahita-libs/constants"
)

// ErrNotAPerson is a person type asked of something that is not a person
// here: a group, a profile on another server, or a person deleted.
var ErrNotAPerson = errors.New("not_a_person")

// ErrNotAPersonType is a type that is not one a person may be given.
var ErrNotAPersonType = errors.New("not_a_person_type")

// PersonTypeOf is a person's type: only a person here, not deleted. A profile
// from another server has no type here worth the name, and a deleted one is
// restored, not promoted.
func (r *nodesRepo) PersonTypeOf(id int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	const read = `
SELECT person_type
FROM nodes
WHERE id = ?
  AND type = ?
  AND deleted_at IS NULL;
`
	var personType string
	err := r.db.QueryRowContext(ctx, read, id, constants.NodeActorPersonEntityType).Scan(&personType)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotAPerson
	}
	return personType, err
}

// SetPersonType changes a person's type from one to another, answering
// whether it changed. The only thing that writes person_type after a person
// is made; Edit leaves it alone, it being an authority level and not a
// profile field.
//
// Only from the type that was read and judged: if it has changed since,
// nothing is written, so a decision is never applied to a type it was not
// made about. Who may ask is the caller's to say (constants.MayChangePersonType).
func (r *nodesRepo) SetPersonType(id int64, from, to string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if !constants.IsGivenPersonType(to) {
		return false, ErrNotAPersonType
	}

	const update = `
UPDATE nodes
SET person_type = ?
WHERE id = ?
  AND type = ?
  AND person_type = ?
  AND deleted_at IS NULL;
`
	result, err := r.db.ExecContext(ctx, update, to, id, constants.NodeActorPersonEntityType, from)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}
