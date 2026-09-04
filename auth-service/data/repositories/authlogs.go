package repositories

import (
	"context"
	"database/sql"
	"time"

	"auth/constants"
	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// NewAuthLog returns a new instance of the authlog repository
func NewAuthLogs(db *sql.DB) AuthLogsInterface {
	return &authLogs{
		db:  db,
		sql: stmtBuilder,
	}
}

type authLogs struct {
	db  *sql.DB
	sql sq.StatementBuilderType
}

// DeleteByPersonID removes every auth log belonging to a person.
//
// Zero is a success: somebody who never signed in has nothing to remove.
func (repo *authLogs) DeleteByPersonID(personID int64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	result, err := repo.db.ExecContext(ctx,
		`DELETE FROM auth_logs WHERE created_by = ?`, personID)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// Browse returns all authlogs for a given user
func (repo *authLogs) Browse(user *entities.User) ([]*entities.AuthLog, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.getBaseSelectQuery().Where(
		sq.Eq{
			"created_by": user.ID,
		},
	).OrderBy("created_on DESC")

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*entities.AuthLog
	for rows.Next() {
		authLog := entities.NewAuthLog()
		var refreshTokenID sql.NullInt64
		err := rows.Scan(
			&authLog.ID,
			&authLog.CreatedOn,
			&authLog.CreatedBy,
			&authLog.Device,
			&authLog.OS,
			&authLog.Name,
			&authLog.IP,
			&authLog.City,
			&authLog.Country,
			&refreshTokenID,
			&authLog.ClientID,
		)

		if err != nil {
			return nil, err
		}

		if refreshTokenID.Valid {
			authLog.OAuthRefreshTokenID = refreshTokenID.Int64
		}

		results = append(results, authLog)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// Add inserts a new authlog into the database
func (repo *authLogs) Add(authLog *entities.AuthLog) (*entities.AuthLog, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	if err := authLog.Validate(); err != nil {
		return nil, err
	}

	query := repo.sql.Insert("auth_logs").Columns(
		"created_on",
		"created_by",
		"agent_device",
		"agent_os",
		"agent_name",
		"ip",
		"city",
		"country",
		"oauth_refresh_token_id",
		"client_id",
	).Values(
		time.Now().UTC(),
		authLog.CreatedBy,
		authLog.Device,
		authLog.OS,
		authLog.Name,
		authLog.IP,
		authLog.City,
		authLog.Country,
		authLog.OAuthRefreshTokenID,
		authLog.ClientID,
	)

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	res, err := repo.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	authLog.ID = id

	return repo.ReadByID(authLog.ID)
}

// UpdateOAuthRefreshTokenID updates the refresh-token pointer on an
// auth log row. Called from the refresh flow when a refresh token
// rotates, so the auth log continues to point at the live token.
//
// Note: client_id is NOT updated here — it stays the same across
// rotations since the OAuth client is the same. Only the refresh
// token row id changes.
func (repo *authLogs) UpdateOAuthRefreshTokenID(authLogID, newRefreshTokenID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sql.Update("auth_logs").
		Set("oauth_refresh_token_id", newRefreshTokenID).
		Where(sq.Eq{"id": authLogID})

	q, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = repo.db.ExecContext(ctx, q, args...)
	return err
}

// Trim deletes all but the last 10 authlogs for a given user
func (repo *authLogs) Trim(user *entities.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	q := `
		DELETE FROM auth_logs
		WHERE created_by = ?
		AND id NOT IN (
			SELECT id FROM (
				SELECT id FROM auth_logs WHERE created_by = ? ORDER BY created_on DESC LIMIT ?
			) AS t
		)
	`

	_, err := repo.db.ExecContext(ctx, q, user.ID, user.ID, constants.AuthLogMaxNumLogs)
	if err != nil {
		return err
	}

	return nil
}

// Delete removes an authlog from the database
func (repo *authLogs) Delete(ID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// For MySQL, use sq.Question for placeholder format
	query := repo.sql.Delete("auth_logs").Where(sq.Eq{"id": ID})
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
