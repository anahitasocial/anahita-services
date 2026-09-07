package repositories

import (
	"context"
	"database/sql"
	"errors"

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
func (repo *tokens) Count(tokenType string, req *requests.Browse) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.
		Select("COUNT(*)").
		From("tokens t")

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

// Browse returns a list of tokens
func (repo *tokens) Browse(tokenType string, req *requests.Browse) ([]*entities.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.getBaseSelectQuery()

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
			&token.Creator.GivenName,
			&token.Creator.FamilyName,
			&token.Creator.Email,
			&token.Editor.ID,
			&token.Editor.Type,
			&token.Editor.GivenName,
			&token.Editor.FamilyName,
			&token.Editor.Email,
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

	return repo.ReadByID(token.ID)
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
