package repositories

import (
	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// ReadByID returns a single refresh token by its ID.
func (repo *oauthRefreshTokens) ReadByID(id int64) (*entities.OAuthRefreshToken, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"id": id})
	return repo.getOne(query)
}

// ReadByTokenHash returns a refresh token by its hash. Does not filter by
// revoked/expired state — callers should check IsValid().
func (repo *oauthRefreshTokens) ReadByTokenHash(tokenHash string) (*entities.OAuthRefreshToken, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"token_hash": tokenHash})
	return repo.getOne(query)
}
