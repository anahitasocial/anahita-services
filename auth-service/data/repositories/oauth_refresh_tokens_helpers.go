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
	"revoked_at",
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
	var revokedAt sql.NullTime
	err = row.Scan(
		&token.ID,
		&token.TokenHash,
		&token.UserID,
		&token.ClientID,
		&scopes,
		&token.Revoked,
		&token.ExpiresAt,
		&token.CreatedAt,
		&revokedAt,
	)

	if err != nil {
		return nil, err
	}

	token.Scopes = scopes.String
	if revokedAt.Valid {
		token.RevokedAt = &revokedAt.Time
	}
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
		// Declared inside the loop, not above it: a shared NullTime would
		// let a row with a NULL revoked_at inherit the previous row's
		// timestamp, since Scan leaves the value untouched when Valid is
		// false.
		var revokedAt sql.NullTime
		err := rows.Scan(
			&token.ID,
			&token.TokenHash,
			&token.UserID,
			&token.ClientID,
			&scopes,
			&token.Revoked,
			&token.ExpiresAt,
			&token.CreatedAt,
			&revokedAt,
		)

		if err != nil {
			return nil, err
		}

		token.Scopes = scopes.String
		if revokedAt.Valid {
			token.RevokedAt = &revokedAt.Time
		}
		results = append(results, token)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// revokeWhere marks all matching non-revoked tokens as revoked, stamping
// when it happened.
//
// revoked_at comes from the database's NOW() rather than the application
// clock so that every auth-service replica writes on one clock; reuse
// detection compares against it to tell a rotation race from a replay.
// The revoked = false guard means an already-revoked row keeps its
// original timestamp instead of having it pushed forward, which would
// otherwise let a replayed token look freshly rotated on every attempt.
func (repo *oauthRefreshTokens) revokeWhere(pred sq.Sqlizer) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Update("oauth_refresh_tokens").
		Set("revoked", true).
		Set("revoked_at", sq.Expr("NOW()")).
		Where(pred).
		Where(sq.Eq{"revoked": false})

	q, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = repo.db.ExecContext(ctx, q, args...)
	return err
}
