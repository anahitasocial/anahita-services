package repositories

import (
	"context"
	"database/sql"

	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// oauthRefreshTokenFields is the canonical select list. It must stay in
// step with the scans in getOne and browseWhere.
//
// It previously also listed oauth_refresh_token_id and organization_id,
// both carried over from the upstream Postgres schema this service was
// ported from. Neither column exists in the MySQL oauth_refresh_tokens
// table, and neither was ever scanned — the select list was 10 columns
// against an 8-column scan, so every read would have failed regardless
// of engine.
var oauthRefreshTokenFields = []string{
	"id",
	"token_hash",
	"user_id",
	"client_id",
	"scopes",
	"revoked",
	"expires_at",
	"created_at",
}

// getBaseSelectQuery returns the base select query for oauth refresh tokens.
func (repo *oauthRefreshTokens) getBaseSelectQuery() sq.SelectBuilder {
	return repo.sb.Select(oauthRefreshTokenFields...).From("oauth_refresh_tokens")
}

// getOne scans a single refresh token from the database.
func (repo *oauthRefreshTokens) getOne(query sq.SelectBuilder) (*entities.OAuthRefreshToken, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	row := repo.db.QueryRowContext(ctx, q, args...)
	token := entities.NewOAuthRefreshToken()

	var scopes sql.NullString
	err = row.Scan(
		&token.ID,
		&token.TokenHash,
		&token.UserID,
		&token.ClientID,
		&scopes,
		&token.Revoked,
		&token.ExpiresAt,
		&token.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	token.Scopes = scopes.String
	return token, nil
}

// browseWhere returns all non-revoked refresh tokens matching the given
// predicate, newest first.
func (repo *oauthRefreshTokens) browseWhere(pred sq.Sqlizer) ([]*entities.OAuthRefreshToken, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.getBaseSelectQuery().
		Where(pred).
		Where(sq.Eq{"revoked": false}).
		OrderBy("created_at DESC")

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*entities.OAuthRefreshToken
	for rows.Next() {
		token := entities.NewOAuthRefreshToken()
		var scopes sql.NullString
		err := rows.Scan(
			&token.ID,
			&token.TokenHash,
			&token.UserID,
			&token.ClientID,
			&scopes,
			&token.Revoked,
			&token.ExpiresAt,
			&token.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		token.Scopes = scopes.String
		results = append(results, token)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// revokeWhere marks all matching non-revoked tokens as revoked.
func (repo *oauthRefreshTokens) revokeWhere(pred sq.Sqlizer) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Update("oauth_refresh_tokens").
		Set("revoked", true).
		Where(pred).
		Where(sq.Eq{"revoked": false})

	q, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = repo.db.ExecContext(ctx, q, args...)
	return err
}
