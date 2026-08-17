package repositories

import (
	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// ReadByID returns a single authorization by its bigint id.
func (repo *oauthAuthorizations) ReadByID(id int64) (*entities.OAuthAuthorization, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"id": id})
	return repo.getOne(query)
}

// ReadByCodeHash returns an authorization by its hashed code. Does not
// filter by expiry — callers should check IsExpired() and delete stale rows.
func (repo *oauthAuthorizations) ReadByCodeHash(codeHash string) (*entities.OAuthAuthorization, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"code_hash": codeHash})
	return repo.getOne(query)
}
