package repositories

import (
	"context"

	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

var oauthAuthorizationFields = []string{
	"id",
	"code_hash",
	"client_id",
	"user_id",
	"redirect_uri",
	"scopes",
	"code_challenge",
	"code_challenge_method",
	"nonce",
	"expires_at",
	"created_at",
}

// getBaseSelectQuery returns the base select query for oauth authorizations.
func (repo *oauthAuthorizations) getBaseSelectQuery() sq.SelectBuilder {
	return repo.sb.Select(oauthAuthorizationFields...).From("oauth_authorizations")
}

// getOne scans a single authorization from the database.
func (repo *oauthAuthorizations) getOne(query sq.SelectBuilder) (*entities.OAuthAuthorization, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	row := repo.db.QueryRowContext(ctx, q, args...)
	auth := entities.NewOAuthAuthorization()

	err = row.Scan(
		&auth.ID,
		&auth.CodeHash,
		&auth.ClientID,
		&auth.UserID,
		&auth.RedirectURI,
		&auth.Scopes,
		&auth.CodeChallenge,
		&auth.CodeChallengeMethod,
		&auth.Nonce,
		&auth.ExpiresAt,
		&auth.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return auth, nil
}
