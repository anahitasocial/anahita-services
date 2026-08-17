package repositories

import (
	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// ReadByID returns a single authlog by its ID
func (repo *authLogs) ReadByID(ID int64) (*entities.AuthLog, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"id": ID})
	return repo.getOne(query)
}

// ReadByOAuthRefreshTokenID looks up an auth log by its current
// refresh-token pointer.
func (repo *authLogs) ReadByOAuthRefreshTokenID(refreshTokenID int64) (*entities.AuthLog, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"oauth_refresh_token_id": refreshTokenID})
	return repo.getOne(query)
}

// ReadLatestByCreatedByAndClient returns the most recent auth log
// row for the given person (filtered via created_by) and OAuth
// client, or sql.ErrNoRows if none exists.
//
// Used by the login-notification gate in recordAuthLogAfterIssuance
// to suppress emails on silent re-auth: when the SPA's access token
// expires and it bounces through /oauth/authorize, the IdP's
// person_session cookie produces a new auth code → new token pair
// → new auth_log row, all without any credential event. Suppressing
// the email in that case requires knowing whether a prior auth_log
// row already exists in a recent window — that's what this lookup
// provides.
//
// Requires a composite index on (created_by, client_id, created_on
// DESC) for this to be cheap on the hot path.
func (repo *authLogs) ReadLatestByCreatedByAndClient(createdBy int64, clientID string) (*entities.AuthLog, error) {
	query := repo.getBaseSelectQuery().
		Where(sq.Eq{"created_by": createdBy}).
		Where(sq.Eq{"client_id": clientID}).
		OrderBy("created_on DESC").
		Limit(1)
	return repo.getOne(query)
}
