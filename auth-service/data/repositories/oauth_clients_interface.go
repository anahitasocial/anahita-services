package repositories

import (
	"auth/data/entities"
)

type OAuthClientsInterface interface {
	// Browse returns all oauth clients, newest first. Excludes secret hash.
	Browse() ([]*entities.OAuthClient, error)

	// ReadByID returns a single oauth client by its bigint id.
	ReadByID(id int64) (*entities.OAuthClient, error)

	// ReadByClientID returns a single oauth client by its slug client_id.
	// Excludes secret hash.
	ReadByClientID(clientID string) (*entities.OAuthClient, error)

	// ReadByClientIDWithSecret returns a single oauth client including
	// its secret hash, for /token client authentication.
	ReadByClientIDWithSecret(clientID string) (*entities.OAuthClient, error)

	// Add inserts a new oauth client into the database.
	Add(client *entities.OAuthClient) (*entities.OAuthClient, error)

	// Edit updates an oauth client.
	Edit(client *entities.OAuthClient) (*entities.OAuthClient, error)

	// Delete removes an oauth client.
	Delete(id int64) error
}
