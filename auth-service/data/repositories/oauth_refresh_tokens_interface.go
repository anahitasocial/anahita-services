package repositories

import (
	"auth/data/entities"

	libs_entities "anahita-libs/data/entities"
)

type OAuthRefreshTokensInterface interface {
	// ReadByOAuthRefreshTokenID returns a single refresh token by its bigint id.
	ReadByID(id int64) (*entities.OAuthRefreshToken, error)

	// ReadByTokenHash returns a refresh token by its hash.
	ReadByTokenHash(tokenHash string) (*entities.OAuthRefreshToken, error)

	// BrowseByViewer returns all non-revoked refresh tokens belonging
	// to the currently authenticated person.
	BrowseByViewer(viewer *libs_entities.Viewer) ([]*entities.OAuthRefreshToken, error)

	// BrowseByClientID returns all non-revoked refresh tokens for a client.
	BrowseByClientID(clientID string) ([]*entities.OAuthRefreshToken, error)

	// Add inserts a new refresh token into the database.
	Add(token *entities.OAuthRefreshToken) (*entities.OAuthRefreshToken, error)

	// Revoke revokes a single refresh token by its bigint id.
	Revoke(id int64) error

	// RevokeIfActive revokes a single refresh token only if it is not
	// already revoked. Reports whether this caller was the one that
	// revoked it.
	RevokeIfActive(id int64) (bool, error)

	// RevokeByViewer revokes all refresh tokens belonging to the viewer.
	RevokeByViewer(viewer *libs_entities.Viewer) error

	// RevokeByClientID revokes all refresh tokens issued to a client.
	RevokeByClientID(clientID string) error

	// RevokeByUserAndClient revokes all refresh tokens issued to a client
	// for a single user. This is the reuse-detection blast radius.
	RevokeByUserAndClient(userID int64, clientID string) error

	// DeleteExpired hard-deletes all refresh tokens past expiry.
	DeleteExpired() (int64, error)
}
