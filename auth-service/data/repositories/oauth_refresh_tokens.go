package repositories

import (
	"context"
	"database/sql"
	"time"

	"auth/data/entities"

	libs_entities "anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// NewOAuthRefreshTokens returns a new instance of the oauth refresh token repository.
func NewOAuthRefreshTokens(db *sql.DB) OAuthRefreshTokensInterface {
	return &oauthRefreshTokens{
		db: db,
		sb: stmtBuilder,
	}
}

type oauthRefreshTokens struct {
	db *sql.DB
	sb sq.StatementBuilderType
}

// BrowseByViewer returns all non-revoked refresh tokens belonging to the
// currently authenticated person. Used for the "manage my sessions" UI.
func (repo *oauthRefreshTokens) BrowseByViewer(viewer *libs_entities.Viewer) ([]*entities.OAuthRefreshToken, error) {
	return repo.browseWhere(sq.Eq{
		"user_id": viewer.ID,
	})
}

// BrowseByClientID returns all non-revoked refresh tokens for a client.
func (repo *oauthRefreshTokens) BrowseByClientID(clientID string) ([]*entities.OAuthRefreshToken, error) {
	return repo.browseWhere(sq.Eq{"client_id": clientID})
}

// Add inserts a new refresh token into the database.
func (repo *oauthRefreshTokens) Add(token *entities.OAuthRefreshToken) (*entities.OAuthRefreshToken, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Insert("oauth_refresh_tokens").Columns(
		"token_hash",
		"user_id",
		"client_id",
		"scopes",
		"revoked",
		"expires_at",
	).Values(
		token.TokenHash,
		token.UserID,
		token.ClientID,
		sql.NullString{String: token.Scopes, Valid: token.Scopes != ""},
		false,
		token.ExpiresAt,
	)

	id, err := insertReturningID(ctx, repo.db, query)
	if err != nil {
		return nil, err
	}

	token.ID = id

	return repo.ReadByID(token.ID)
}

// Revoke revokes a single refresh token by its bigint id.
func (repo *oauthRefreshTokens) Revoke(id int64) error {
	return repo.revokeWhere(sq.Eq{"id": id})
}

// RevokeByViewer revokes all refresh tokens belonging to the currently
// authenticated person. Used for "sign out of all devices."
func (repo *oauthRefreshTokens) RevokeByViewer(viewer *libs_entities.Viewer) error {
	return repo.revokeWhere(sq.Eq{
		"user_id": viewer.ID,
	})
}

// RevokeByClientID revokes all refresh tokens issued to a client.
// Used when deactivating a client.
func (repo *oauthRefreshTokens) RevokeByClientID(clientID string) error {
	return repo.revokeWhere(sq.Eq{"client_id": clientID})
}

// DeleteExpired hard-deletes all refresh tokens past expiry, regardless of
// revoked state. Intended for periodic cleanup. Returns rows deleted.
func (repo *oauthRefreshTokens) DeleteExpired() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Delete("oauth_refresh_tokens").
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
