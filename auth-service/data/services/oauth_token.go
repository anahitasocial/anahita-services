package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"auth/data/entities"

	libs_constants "anahita-libs/constants"
)

// RefreshTokenLifetime is the validity window for refresh tokens.
const RefreshTokenLifetime = 30 * 24 * time.Hour

// IssueTokenPairForViewer issues an access + refresh token pair for the
// authenticated Viewer. Used by the /authorize consent handler after the
// user has approved the grant, and called via /token's
// authorization_code path after the code is exchanged.
//
// Because the Viewer already carries the person's fields fully
// hydrated, this path needs no further lookups.
//
// Order matters: the refresh token row is persisted first so its
// BIGINT id can be embedded as the refresh_token_id claim on the
// access token. This lets downstream code map an access token back to
// its session without an extra lookup.
//
// If the granted scopes include "openid", an ID token is included.
func (s *OAuthService) IssueTokenPairForViewer(
	ctx context.Context,
	user *entities.User,
	client *entities.OAuthClient,
	scopes string,
	nonce string,
) (*entities.OAuthTokenPair, error) {
	if user == nil {
		return nil, errors.New("user is required to issue token pair")
	}

	refreshPlaintext, persistedRefresh, err := s.persistRefreshToken(
		user.ID,
		client.ClientID,
		scopes,
	)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.MintAccessToken(ctx, accessTokenClaims{
		Subject:        strconv.FormatInt(user.ID, 10),
		PersonType:     user.UserType,
		ClientID:       client.ClientID,
		Scopes:         scopes,
		PersonPK:       user.ID,
		RefreshTokenID: persistedRefresh.ID,
	})
	if err != nil {
		return nil, err
	}

	pair := &entities.OAuthTokenPair{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(AccessTokenLifetime.Seconds()),
		RefreshToken: refreshPlaintext,
		Scope:        scopes,
	}

	if scopesInclude(scopes, libs_constants.OAuthScopeOpenID) {
		idToken, err := s.MintIDToken(ctx, idTokenInput{
			Subject:  strconv.FormatInt(user.ID, 10),
			ClientID: client.ClientID,
			Nonce:    nonce,
			User:     user,
			Scopes:   scopes,
		})
		if err != nil {
			return nil, err
		}
		pair.IDToken = idToken
	}

	return pair, nil
}

// IssueClientAccessToken mints an access token for the client_credentials
// grant. No refresh token, no user context, no person_type — the client
// itself is the subject. PersonPK, OrgPK, RefreshTokenID, and Apps are
// deliberately omitted; MintAccessToken skips the corresponding claims
// when they're zero/nil.
func (s *OAuthService) IssueClientAccessToken(
	ctx context.Context,
	client *entities.OAuthClient,
	scopes string,
) (*entities.OAuthTokenPair, error) {
	accessToken, err := s.MintAccessToken(ctx, accessTokenClaims{
		Subject:  client.ClientID,
		ClientID: client.ClientID,
		Scopes:   scopes,
	})
	if err != nil {
		return nil, err
	}

	return &entities.OAuthTokenPair{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(AccessTokenLifetime.Seconds()),
		Scope:       scopes,
	}, nil
}

// RefreshTokenPair exchanges a plaintext refresh token for a new token
// pair. The refresh flow has no ambient user session, so this path
// resolves the person from the numeric FK on the refresh token row.
// The old refresh token is rotated.
//
// Auth log linkage: when the refresh token rotates, the auth_logs row
// that pointed at the old token is updated to point at the new one.
// Best-effort — a failure here doesn't fail the refresh, since the
// token pair is already valid; the auth log just falls out of sync.
//
// If a revoked refresh token is presented, all of this client's tokens
// for the owning user are revoked (OAuth 2.1 reuse-detection guidance).
//
// If requestedScopes is non-empty, it must be a subset of the original
// grant.
func (s *OAuthService) RefreshTokenPair(
	ctx context.Context,
	refreshPlaintext string,
	client *entities.OAuthClient,
	requestedScopes string,
) (*entities.OAuthTokenPair, error) {
	stored, err := s.Repos.OAuthRefreshTokens.ReadByTokenHash(HashToken(refreshPlaintext))
	if err != nil {
		return nil, entities.ErrRefreshTokenNotFound
	}

	if stored.Revoked {
		_ = s.Repos.OAuthRefreshTokens.RevokeByClientID(client.ClientID)
		return nil, entities.ErrRefreshTokenReuse
	}
	if stored.IsExpired() {
		return nil, entities.ErrRefreshTokenExpired
	}
	if stored.ClientID != client.ClientID {
		return nil, errors.New("refresh token does not belong to this client")
	}

	grantedScopes := stored.Scopes
	if requestedScopes != "" {
		narrowed, err := NarrowScopes(stored.Scopes, requestedScopes)
		if err != nil {
			return nil, err
		}
		grantedScopes = narrowed
	}

	user, err := s.Repos.Users.ReadByID(stored.UserID)
	if err != nil {
		return nil, fmt.Errorf("resolve person: %w", err)
	}

	if err := s.Repos.OAuthRefreshTokens.Revoke(stored.ID); err != nil {
		return nil, err
	}

	refreshPlaintextNew, persistedRefreshNew, err := s.persistRefreshToken(
		stored.UserID,
		client.ClientID,
		grantedScopes,
	)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.MintAccessToken(ctx, accessTokenClaims{
		Subject:        strconv.FormatInt(user.ID, 10),
		PersonType:     user.UserType,
		ClientID:       client.ClientID,
		Scopes:         grantedScopes,
		PersonPK:       stored.UserID,
		RefreshTokenID: persistedRefreshNew.ID,
	})
	if err != nil {
		return nil, err
	}

	// Update the auth log pointer to follow the rotated refresh token.
	// Best-effort — log on failure but don't fail the refresh, since
	// the new token pair is already valid and useful to the caller.
	if authLog, err := s.Repos.AuthLogs.ReadByID(stored.ID); err == nil {
		if err := s.Repos.AuthLogs.UpdateOAuthRefreshTokenID(
			authLog.ID,
			persistedRefreshNew.ID,
		); err != nil {
			// Non-fatal: token is issued, auth log just stale.
			fmt.Println("authlog: update refresh token pointer failed:", err)
		}
	}

	// Per OIDC: ID token is for authentication, not refresh — no id_token
	// on the refresh response.
	return &entities.OAuthTokenPair{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(AccessTokenLifetime.Seconds()),
		RefreshToken: refreshPlaintextNew,
		Scope:        grantedScopes,
	}, nil
}

// ValidateRefreshToken checks a refresh token is valid and belongs to
// the given client. Exposed for callers that want to verify without
// exchanging.
func (s *OAuthService) ValidateRefreshToken(
	token *entities.OAuthRefreshToken,
	client *entities.OAuthClient,
) error {
	if token.Revoked {
		return entities.ErrRefreshTokenRevoked
	}
	if token.IsExpired() {
		return entities.ErrRefreshTokenExpired
	}
	if token.ClientID != client.ClientID {
		return errors.New("refresh token does not belong to this client")
	}
	return nil
}

// persistRefreshToken generates a plaintext refresh token, stores its
// hash, and returns both the plaintext (for the response) and the
// persisted entity (for callers that want metadata, including the
// BIGINT id needed for the refresh_token_id claim).
func (s *OAuthService) persistRefreshToken(
	userID int64,
	clientID string,
	scopes string,
) (string, *entities.OAuthRefreshToken, error) {
	plaintext, err := generateRandomToken(32)
	if err != nil {
		return "", nil, err
	}

	refresh := entities.NewOAuthRefreshToken()
	refresh.TokenHash = HashToken(plaintext)
	refresh.UserID = userID
	refresh.ClientID = clientID
	refresh.Scopes = scopes
	refresh.ExpiresAt = time.Now().Add(RefreshTokenLifetime)

	persisted, err := s.Repos.OAuthRefreshTokens.Add(refresh)
	if err != nil {
		return "", nil, err
	}
	return plaintext, persisted, nil
}

// scopesInclude returns true if the space-separated scopes string
// contains the given scope.
func scopesInclude(scopes, scope string) bool {
	return strings.Contains(" "+scopes+" ", " "+scope+" ")
}
