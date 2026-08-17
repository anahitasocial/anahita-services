package repositories

import (
	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// ReadByID returns a single oauth client by its bigint id.
func (repo *oauthClients) ReadByID(id int64) (*entities.OAuthClient, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"id": id})
	return repo.getOne(query)
}

// ReadByClientID returns a single oauth client by its slug client_id.
// Does not include secret hash; does not filter by active state.
func (repo *oauthClients) ReadByClientID(clientID string) (*entities.OAuthClient, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"client_id": clientID})
	return repo.getOne(query)
}

// ReadByClientIDWithSecret returns an oauth client including its secret
// hash. Used for client authentication at /token. Callers MUST compare
// the secret using constant-time comparison.
func (repo *oauthClients) ReadByClientIDWithSecret(clientID string) (*entities.OAuthClient, error) {
	query := repo.getBaseSelectQueryWithSecret().Where(sq.Eq{"client_id": clientID})
	return repo.getOneWithSecret(query)
}
