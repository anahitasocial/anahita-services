package repositories

import (
	"context"

	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// oauthClientPublicFields is the column list excluding secret_hash.
var oauthClientPublicFields = []string{
	"id",
	"client_id",
	"name",
	"redirect_uris",
	"grant_types",
	"scopes",
	"token_expiry",
	"confidential",
	"skip_consent",
	"first_party",
	"active",
	"created_at",
}

// oauthClientFullFields is the column list including secret_hash. Only
// used for client authentication at /token.
var oauthClientFullFields = []string{
	"id",
	"client_id",
	"secret_hash",
	"name",
	"redirect_uris",
	"grant_types",
	"scopes",
	"token_expiry",
	"confidential",
	"skip_consent",
	"first_party",
	"active",
	"created_at",
}

// getBaseSelectQuery returns the base select query for oauth clients
// without the secret hash.
func (repo *oauthClients) getBaseSelectQuery() sq.SelectBuilder {
	return repo.sb.Select(oauthClientPublicFields...).From("oauth_clients")
}

// getBaseSelectQueryWithSecret returns the base select query for oauth
// clients including the secret hash.
func (repo *oauthClients) getBaseSelectQueryWithSecret() sq.SelectBuilder {
	return repo.sb.Select(oauthClientFullFields...).From("oauth_clients")
}

// getOne scans a single oauth client (without secret hash) from the database.
func (repo *oauthClients) getOne(query sq.SelectBuilder) (*entities.OAuthClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	row := repo.db.QueryRowContext(ctx, q, args...)
	client := entities.NewOAuthClient()
	err = row.Scan(
		&client.ID,
		&client.ClientID,
		&client.Name,
		&client.RedirectURIs,
		&client.GrantTypes,
		&client.Scopes,
		&client.TokenExpiry,
		&client.Confidential,
		&client.SkipConsent,
		&client.FirstParty,
		&client.Active,
		&client.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return client, nil
}

// getOneWithSecret scans a single oauth client including the secret hash.
func (repo *oauthClients) getOneWithSecret(query sq.SelectBuilder) (*entities.OAuthClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	row := repo.db.QueryRowContext(ctx, q, args...)
	client := entities.NewOAuthClient()
	err = row.Scan(
		&client.ID,
		&client.ClientID,
		&client.SecretHash,
		&client.Name,
		&client.RedirectURIs,
		&client.GrantTypes,
		&client.Scopes,
		&client.TokenExpiry,
		&client.Confidential,
		&client.SkipConsent,
		&client.FirstParty,
		&client.Active,
		&client.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return client, nil
}
