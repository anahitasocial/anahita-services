package repositories

import (
	"context"
	"database/sql"

	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// NewOAuthClients returns a new instance of the oauth client repository.
func NewOAuthClients(db *sql.DB) OAuthClientsInterface {
	return &oauthClients{
		db: db,
		sb: stmtBuilder,
	}
}

type oauthClients struct {
	db *sql.DB
	sb sq.StatementBuilderType
}

// Browse returns all oauth clients, newest first. Does not include secret hash.
func (repo *oauthClients) Browse() ([]*entities.OAuthClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.getBaseSelectQuery().OrderBy("created_at DESC")

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*entities.OAuthClient
	for rows.Next() {
		client := entities.NewOAuthClient()
		err := rows.Scan(
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

		results = append(results, client)
	}

	return results, nil
}

// Add inserts a new oauth client into the database.
func (repo *oauthClients) Add(client *entities.OAuthClient) (*entities.OAuthClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	if err := client.Validate(); err != nil {
		return nil, err
	}

	query := repo.sb.Insert("oauth_clients").Columns(
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
	).Values(
		client.ClientID,
		client.SecretHash,
		client.Name,
		client.RedirectURIs,
		client.GrantTypes,
		client.Scopes,
		client.TokenExpiry,
		client.Confidential,
		client.SkipConsent,
		client.FirstParty,
		client.Active,
	)

	id, err := insertReturningID(ctx, repo.db, query)
	if err != nil {
		return nil, err
	}

	client.ID = id

	return repo.ReadByID(client.ID)
}

// Edit updates an oauth client. ClientID and SecretHash are immutable —
// for credential rotation, add a new client and deactivate the old one.
func (repo *oauthClients) Edit(client *entities.OAuthClient) (*entities.OAuthClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	if err := client.Validate(); err != nil {
		return nil, err
	}

	query := repo.sb.Update("oauth_clients").
		Set("name", client.Name).
		Set("redirect_uris", client.RedirectURIs).
		Set("grant_types", client.GrantTypes).
		Set("scopes", client.Scopes).
		Set("token_expiry", client.TokenExpiry).
		Set("confidential", client.Confidential).
		Set("skip_consent", client.SkipConsent).
		Set("first_party", client.FirstParty).
		Set("active", client.Active).
		Where(sq.Eq{"id": client.ID})

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	_, err = repo.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}

	return repo.ReadByID(client.ID)
}

// Delete removes an oauth client. ON DELETE CASCADE on child tables will
// drop all authorizations and refresh tokens for this client.
func (repo *oauthClients) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Delete("oauth_clients").Where(sq.Eq{"id": id})
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
