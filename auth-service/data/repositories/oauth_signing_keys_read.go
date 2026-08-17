package repositories

import (
	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// ReadByOAuthSigningKeyID returns a single signing key by its bigint id.
func (repo *oauthSigningKeys) ReadByID(id int64) (*entities.OAuthSigningKey, error) {
	query := repo.getBaseSelectQueryWithPrivate().Where(sq.Eq{"id": id})
	return repo.getOneWithPrivate(query)
}

// ReadByKeyID returns a signing key by its JWT kid header value. Includes
// the private key — only called internally by the token-signing service.
func (repo *oauthSigningKeys) ReadByKeyID(keyID string) (*entities.OAuthSigningKey, error) {
	query := repo.getBaseSelectQueryWithPrivate().Where(sq.Eq{"key_id": keyID})
	return repo.getOneWithPrivate(query)
}

// ReadActiveSigningKey returns the current active, non-expired signing key
// used to sign new tokens. If multiple active keys exist, the most recently
// created is returned.
func (repo *oauthSigningKeys) ReadActiveSigningKey() (*entities.OAuthSigningKey, error) {
	query := repo.getBaseSelectQueryWithPrivate().
		Where(sq.Eq{"active": true}).
		Where(sq.Expr("expires_at > NOW()")).
		OrderBy("created_at DESC").
		Limit(1)
	return repo.getOneWithPrivate(query)
}
