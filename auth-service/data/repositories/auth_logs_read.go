package repositories

import (
	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// ReadByID returns a single authLog by its ID
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

// ReadLatestByAgent returns the most recent auth log for the given
// person, OAuth client and agent fingerprint, or sql.ErrNoRows if
// none exists.
//
// This is the key the silent re-auth path matches on. When the SPA's
// access token expires it bounces through /oauth/authorize, the
// person_session cookie mints a fresh code, and the token endpoint
// lands on a brand new refresh token with no lineage back to the
// previous one — so there is nothing token-shaped to join on. The
// agent fingerprint is the closest available stand-in for "the same
// browser as before".
//
// It is deliberately narrow: matching only (created_by, client_id)
// would fold every browser on the account into one row, which is
// exactly the distinction the sessions list exists to show. Adding
// agent_name is what keeps Chrome and Safari apart; ip keeps the
// same browser on a different network reading as a new session,
// which is the safer error to make for something users check to
// spot intruders.
//
// Two profiles of the same browser on one machine are genuinely
// indistinguishable at this level and will share a row. Telling them
// apart needs an IdP session id carried through the authorization
// code, which the code record does not currently hold.
func (repo *authLogs) ReadLatestByAgent(
	createdBy int64,
	clientID string,
	device string,
	os string,
	name string,
	ip string,
) (*entities.AuthLog, error) {
	query := repo.getBaseSelectQuery().
		Where(sq.Eq{"created_by": createdBy}).
		Where(sq.Eq{"client_id": clientID}).
		Where(sq.Eq{"agent_device": device}).
		Where(sq.Eq{"agent_os": os}).
		Where(sq.Eq{"agent_name": name}).
		Where(sq.Eq{"ip": ip}).
		OrderBy("created_on DESC").
		Limit(1)
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
