package repositories

import (
	"context"
	"database/sql"
	"time"

	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// NewOAuthAuthorizations returns a new instance of the oauth authorization repository.
func NewOAuthAuthorizations(db *sql.DB) OAuthAuthorizationsInterface {
	return &oauthAuthorizations{
		db: db,
		sb: stmtBuilder,
	}
}

type oauthAuthorizations struct {
	db *sql.DB
	sb sq.StatementBuilderType
}

// Add inserts a new authorization code into the database.
func (repo *oauthAuthorizations) Add(auth *entities.OAuthAuthorization) (*entities.OAuthAuthorization, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Insert("oauth_authorizations").Columns(
		"code_hash",
		"client_id",
		"user_id",
		"redirect_uri",
		"scopes",
		"code_challenge",
		"code_challenge_method",
		"nonce",
		"expires_at",
	).Values(
		auth.CodeHash,
		auth.ClientID,
		auth.UserID,
		auth.RedirectURI,
		nullStringIfEmpty(auth.Scopes),
		nullStringIfEmpty(auth.CodeChallenge),
		nullStringIfEmpty(auth.CodeChallengeMethod),
		nullStringIfEmpty(auth.Nonce),
		auth.ExpiresAt,
	)

	id, err := insertReturningID(ctx, repo.db, query)
	if err != nil {
		return nil, err
	}

	auth.ID = id

	return repo.ReadByID(auth.ID)
}

// Delete removes an authorization code by its bigint id. Called after
// successful code exchange to prevent reuse (authorization codes are
// single-use per OAuth spec).
func (repo *oauthAuthorizations) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Delete("oauth_authorizations").
		Where(sq.Eq{"id": id})

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

// DeleteByClientID removes all authorization codes for a client. Used
// when deactivating a client to clean up any in-flight codes.
func (repo *oauthAuthorizations) DeleteByClientID(clientID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Delete("oauth_authorizations").
		Where(sq.Eq{"client_id": clientID})

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

// DeleteExpired removes all expired authorization codes. Intended to be
// called periodically by a cleanup job. Returns the number of rows deleted.
func (repo *oauthAuthorizations) DeleteExpired() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Delete("oauth_authorizations").
		Where(sq.Lt{"expires_at": time.Now()})

	q, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	result, err := repo.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// nullStringIfEmpty converts an empty string to a SQL NULL for nullable
// text columns.
func nullStringIfEmpty(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
