package repositories

import (
	"auth/data/entities"
	"context"
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

// getBaseSelectQuery returns the base select query for authlogs
func (repo *authLogs) getBaseSelectQuery() sq.SelectBuilder {
	var fields = []string{
		"id",
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
	}

	return repo.sql.Select(fields...).From("auth_logs")
}

// getOne scans a single authlog from the database
func (repo *authLogs) getOne(query sq.SelectBuilder) (*entities.AuthLog, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	row := repo.db.QueryRowContext(ctx, q, args...)
	authLog := entities.NewAuthLog()
	var refreshTokenID sql.NullInt64
	err = row.Scan(
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

	return authLog, nil
}
