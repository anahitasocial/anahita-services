package repositories

import (
	"auth/data/entities"
)

type OAuthAuthorizationsInterface interface {
	// ReadByID returns a single authorization by its bigint id.
	ReadByID(id int64) (*entities.OAuthAuthorization, error)

	// ReadByCodeHash returns an authorization by its hashed code.
	ReadByCodeHash(codeHash string) (*entities.OAuthAuthorization, error)

	// Add inserts a new authorization code into the database.
	Add(auth *entities.OAuthAuthorization) (*entities.OAuthAuthorization, error)

	// Delete removes an authorization code by its bigint id.
	Delete(id int64) error

	// DeleteByClientID removes all authorization codes for a client.
	DeleteByClientID(clientID string) error

	// DeleteExpired removes all expired authorization codes.
	DeleteExpired() (int64, error)
}
