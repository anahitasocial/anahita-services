package repositories

import (
	"auth/data/entities"
)

type AuthLogsInterface interface {
	// Browse returns all authLogs for a given user
	Browse(user *entities.User) ([]*entities.AuthLog, error)

	// DeleteByPersonID removes every auth log belonging to a person and
	// returns how many rows went.
	//
	// For account purge only. These rows hold IP addresses, cities and device
	// fingerprints — a record of where somebody has been — and they survived
	// account deletion entirely until the sweeper started calling this.
	DeleteByPersonID(personID int64) (int64, error)

	// ReadByID returns a single authLog by its ID
	ReadByID(ID int64) (*entities.AuthLog, error)

	// ReadByOAuthRefreshTokenID returns a single authLog by its current
	// refresh-token pointer. Used by the refresh flow to find the row
	// to update on rotation.
	ReadByOAuthRefreshTokenID(refreshTokenID int64) (*entities.AuthLog, error)

	// ReadLatestByAgent returns the most recent auth log for the given
	// person, client and agent fingerprint, or sql.ErrNoRows if none
	// exists. Used by the silent re-auth path to find the row for this
	// browser instead of inserting a duplicate every 15 minutes.
	ReadLatestByAgent(createdBy int64, clientID, device, os, name, ip string) (*entities.AuthLog, error)

	// ReadLatestByCreatedByAndClient returns the most recent auth log
	// for the given (created_by, client_id) pair, or sql.ErrNoRows if
	// none exists. Used to suppress duplicate login-notification
	// emails on silent re-auth via the IdP's person_session cookie.
	ReadLatestByCreatedByAndClient(createdBy int64, clientID string) (*entities.AuthLog, error)

	// Add inserts a new authLog into the database
	Add(authLog *entities.AuthLog) (*entities.AuthLog, error)

	// UpdateOAuthRefreshTokenID updates the refresh-token pointer on
	// an auth log row. Called when a refresh token rotates.
	UpdateOAuthRefreshTokenID(authLogID, newRefreshTokenID int64) error

	// Trim deletes all but the last 10 authLogs for a given user
	Trim(user *entities.User) error

	// Delete removes an authLog from the database
	Delete(ID int64) error
}
