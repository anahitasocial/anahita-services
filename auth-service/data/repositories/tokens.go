package repositories

import (
	"context"
	"database/sql"
	"errors"

	"auth/constants"
	"auth/data/entities"

	"anahita-libs/requests"

	sq "github.com/Masterminds/squirrel"
)

// NewTokens returns a new instance of the tokens repository
func NewTokens(db *sql.DB) TokensInterface {
	return &tokens{
		db: db,
		sb: stmtBuilder,
	}
}

// tokens is the repository for tokens
type tokens struct {
	db *sql.DB
	sb sq.StatementBuilderType
}

// Count returns the number of tokens
// CountFrom totals the tokens of a type issued by one person.
//
// The pagination total beside BrowseFrom, and scoped the same way for
// the same reason — a count that answers a wider question than the
// list beneath it reports rows the caller will never be shown.
//
// This was Count(tokenType), which counted every token of the type
// whoever asked. Before that it took a `req` it never read, and the
// note explaining its removal said that a function accepting a filter
// and ignoring it is a trap. It was the right lesson from the wrong
// half of the problem: the parameter that needed adding was this one.
func (repo *tokens) CountFrom(tokenType string, personID int64) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.
		Select("COUNT(*)").
		From("tokens t").
		Where(sq.Eq{"t.created_by": personID})

	if tokenType != "" {
		query = query.Where(sq.Eq{"t.type": tokenType})
	}

	q, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	var numRows uint64
	if err := repo.db.QueryRowContext(ctx, q, args...).Scan(&numRows); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return numRows, nil
}

// CountPendingFrom totals the invitations one person has outstanding.
//
// Pending means issued by them, not yet used, and not yet expired. All three
// are needed: a used or expired invitation costs its issuer nothing, which is
// what makes the cap self-healing — a slot frees when somebody accepts, when
// a link lapses, and when it is revoked.
//
// Expiry is computed rather than stored. `status` is derived in the entity,
// not a column, so the window has to be reconstructed here — start_time plus
// expired_after seconds, against now.
func (repo *tokens) CountPendingFrom(tokenType string, personID int64) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		SELECT COUNT(*)
		FROM tokens
		WHERE type = ?
		  AND created_by = ?
		  AND is_used = 0
		  AND DATE_ADD(start_time, INTERVAL expired_after SECOND) > NOW()`

	var count int
	if err := repo.db.QueryRowContext(ctx, query, tokenType, personID).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// BrowseFrom returns the tokens of a type issued by one person.
//
// Scoped by created_by, which is the whole point of the method: a
// tokens row holds a recipient's email address, so an unscoped browse
// hands the caller everybody else's. Pair it with CountFrom, never
// with an unscoped count.
func (repo *tokens) BrowseFrom(tokenType string, personID int64, req *requests.Browse) ([]*entities.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.getBaseSelectQuery().
		Where(sq.Eq{"t.created_by": personID})

	if tokenType != "" {
		query = query.Where(sq.Eq{"t.type": tokenType})
	}

	if req.Limit != 0 {
		query = query.Limit(uint64(req.Limit))
	} else if req.Offset != 0 {
		// MySQL, unlike Postgres, has no bare OFFSET — it is only valid
		// as part of a LIMIT clause, so "page 2 with no page size"
		// would be a syntax error rather than a skip-and-return-rest.
		// maxRowLimit is the documented MySQL idiom for "all remaining
		// rows".
		query = query.Limit(maxRowLimit)
	}

	if req.Offset != 0 {
		query = query.Offset(uint64(req.Offset))
	}

	query = query.OrderBy("t.created_at DESC")

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*entities.Token
	for rows.Next() {
		token := entities.NewToken()
		if err := rows.Scan(
			&token.ID,
			&token.RecipientEmail,
			&token.NewEmail,
			&token.Type,
			&token.Body,
			&token.StartTime,
			&token.ExpiredAfter,
			&token.IsUsed,
			&token.CreatedAt,
			&token.CreatedBy,
			&token.UpdatedAt,
			&token.UpdatedBy,
			&token.Creator.ID,
			&token.Creator.Type,
			&token.Creator.Name,
			&token.Editor.ID,
			&token.Editor.Type,
			&token.Editor.Name,
		); err != nil {
			return nil, err
		}

		token.SetStatus()

		results = append(results, token)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// Add creates a new token
func (repo *tokens) Add(token *entities.Token) (*entities.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	if err := token.Validate(); err != nil {
		return nil, err
	}

	query := repo.sb.Insert("tokens").Columns(
		"type",
		"body",
		"recipient_email",
		// Empty for every type but emailchange, which is fine: the column
		// is nullable and an empty string is as good as NULL for a value
		// nothing reads on those types. Omitting it here is what silently
		// dropped the destination address of every email change.
		"new_email",
		// NULL unless this is a one-time-code token. The code itself is
		// never a column: only its keyed hash is.
		"code_hash",
		"channel",
		"expired_after",
		"is_used",
		"created_at",
		"created_by",
		"updated_at",
		"updated_by",
	).Values(
		token.Type,
		token.Body,
		token.RecipientEmail,
		token.NewEmail,
		nullIfEmpty(token.CodeHash),
		nullIfEmpty(token.Channel),
		token.ExpiredAfter,
		token.IsUsed,
		token.CreatedAt,
		token.CreatedBy,
		token.UpdatedAt,
		token.UpdatedBy,
	)

	// The Postgres original returned token_id here while every other
	// method in this repository filters on id (see ReadByID, Edit,
	// Delete). id is the primary key; token_id was a leftover from the
	// upstream schema this service was ported from, and would have made
	// Add return a value the other methods could not look up.
	id, err := insertReturningID(ctx, repo.db, query)
	if err != nil {
		return nil, err
	}

	token.ID = id

	stored, err := repo.ReadByID(token.ID)
	if err != nil {
		return nil, err
	}

	// The code is never stored, so the row read back cannot carry it. The
	// caller still has to send it: hand it over on the returned token.
	stored.Code = token.Code

	return stored, nil
}

// nullIfEmpty stores NULL for a value that does not apply to this token type.
func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// SpendCodeAttempt counts one submission against a one-time-code token, and
// reports whether there was one left to spend.
//
// One statement, so the check and the increment cannot be separated. Read the
// count, compare, then write it back, and five guesses arriving together each
// read "four so far" and all go through.
func (repo *tokens) SpendCodeAttempt(id int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	result, err := repo.db.ExecContext(ctx,
		`UPDATE tokens SET code_attempts = code_attempts + 1
		  WHERE id = ? AND is_used = 0 AND code_attempts < ?`,
		id, constants.OTPMaxAttempts)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	return affected == 1, err
}

// ConsumeCode marks a one-time-code token used, and reports whether THIS call
// is the one that did it.
//
// False means it was already used: of two correct submissions arriving
// together, exactly one signs in.
func (repo *tokens) ConsumeCode(id, personID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	result, err := repo.db.ExecContext(ctx,
		`UPDATE tokens SET is_used = 1, updated_at = NOW(), updated_by = ?
		  WHERE id = ? AND is_used = 0`,
		personID, id)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	return affected == 1, err
}

// Edit updates a token
func (repo *tokens) Edit(token *entities.Token) (*entities.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	if err := token.Validate(); err != nil {
		return nil, err
	}

	query := repo.sb.Update("tokens").SetMap(map[string]interface{}{
		"expired_after": token.ExpiredAfter,
		"is_used":       token.IsUsed,
		"updated_at":    token.UpdatedAt,
		"updated_by":    token.UpdatedBy,
	}).Where(sq.Eq{"id": token.ID})

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	_, err = repo.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}

	token, err = repo.ReadByID(token.ID)
	if err != nil {
		return nil, err
	}

	token.SetStatus()

	return token, nil
}

// Delete deletes a token
// InvalidateOutstanding retires every unused token of one type for one address.
//
// A single UPDATE rather than a read-then-write loop: two forgot-password
// requests arriving together would otherwise each read the other's token as
// still pending and leave both live.
func (repo *tokens) InvalidateOutstanding(tokenType, email string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	result, err := repo.db.ExecContext(ctx,
		`UPDATE tokens SET is_used = 1, updated_at = NOW()
		  WHERE type = ? AND recipient_email = ? AND is_used = 0`,
		tokenType, email)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// DeleteByRecipientEmail removes every pending token for an address.
//
// Zero is a success: an account with no outstanding reset or invite has nothing
// to remove.
func (repo *tokens) DeleteByRecipientEmail(email string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	result, err := repo.db.ExecContext(ctx,
		`DELETE FROM tokens WHERE recipient_email = ?`, email)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// DeleteUnusedFrom removes the unused tokens of one type that a person sent.
func (repo *tokens) DeleteUnusedFrom(tokenType string, personID int64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	const query = `
DELETE FROM tokens
WHERE type = ?
  AND created_by = ?
  AND is_used = 0`

	result, err := repo.db.ExecContext(ctx, query, tokenType, personID)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

func (repo *tokens) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Delete("tokens").Where(sq.Eq{"id": id})

	q, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = repo.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}

	return nil
}
