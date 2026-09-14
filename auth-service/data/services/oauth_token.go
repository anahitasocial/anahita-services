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

// ReuseGracePeriod is how recently a refresh token must have been revoked
// for a second presentation of it to be forgiven as a rotation race rather
// than punished as a replay.
//
// Refresh tokens are single-use, so presenting a revoked one normally means
// an attacker is replaying a stolen token, and the response is to revoke
// every token the user holds for that client. A client that races itself is
// indistinguishable at the protocol level: two concurrent refreshes with the
// same token, one rotates it, the other arrives milliseconds later to find
// it revoked. Punishing that signs out a user who did nothing wrong,
// including their sessions on other devices.
//
// The trade is explicit: an attacker replaying a stolen token inside this
// window escapes the containment cascade. They still get no token — the
// request fails either way — so what is given up is only the automatic
// revocation of the user's other sessions for that window. RFC 6819 and the
// OAuth 2.1 draft both allow a grace period here for exactly this reason.
//
// 60s comfortably covers a racing client's retry cycle while keeping the
// window where containment is suspended short.
const ReuseGracePeriod = 60 * time.Second

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
		PersonType:     user.PersonType,
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
// If a refresh token that was revoked longer ago than ReuseGracePeriod is
// presented, all of this client's tokens for the owning user are revoked
// (OAuth 2.1 reuse-detection guidance). The blast radius is deliberately
// user+client, never client-wide — a client-wide revocation here would
// sign out every user of the client on every device. Inside the grace
// period the presentation is read as a rotation race instead and nothing
// is revoked; see ReuseGracePeriod.
//
// Rotation is made atomic by RevokeIfActive: the old token is claimed with
// a conditional update before any new state is written, so two requests
// racing with the same plaintext cannot both mint a token pair. The loser
// gets ErrRefreshTokenRaced — not reuse, because cascading would revoke the
// pair the winner just issued to a user who did nothing wrong.
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
		// Revoked moments ago: a concurrent or retrying client presenting a
		// token that was still valid when it read it. Refuse the exchange —
		// the token is spent — but do not cascade, because the request that
		// won the race has already minted a pair the user is relying on.
		//
		// A client genuinely stuck on a dead token still converges: it gets
		// ErrRefreshTokenRaced for the first minute and ErrRefreshTokenReuse
		// after it, so it is eventually told to re-authenticate rather than
		// retrying forever.
		//
		// A nil RevokedAt means the row was revoked before the column
		// existed, which is treated as long ago — fail toward containment.
		if stored.RevokedAt != nil && time.Since(*stored.RevokedAt) < ReuseGracePeriod {
			return nil, entities.ErrRefreshTokenRaced
		}

		// Scope the cascade to the token's OWN client, not the presenting
		// one. This branch runs before the ownership check below, so the
		// two can differ: a public client needs no secret, so an attacker
		// holding a stolen anahita-web token can present it as
		// anahita-mobile. Cascading on client.ClientID there would revoke
		// the user's mobile sessions and leave the actually-compromised
		// web tokens live — containment aimed at the wrong family.
		_ = s.Repos.OAuthRefreshTokens.RevokeByUserAndClient(stored.UserID, stored.ClientID)
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

	// Claim the old token. This is the serialization point for the whole
	// rotation: everything above is read-only, so two requests presenting
	// the same plaintext both reach here, but the conditional update lets
	// exactly one proceed to mint a pair.
	//
	// It sits after the user lookup deliberately. That lookup is read-only
	// and can fail transiently; claiming before it would burn the user's
	// refresh token on a blip and force a needless re-login. Claiming here
	// is just as atomic and leaves the token usable when the lookup fails.
	claimed, err := s.Repos.OAuthRefreshTokens.RevokeIfActive(stored.ID)
	if err != nil {
		return nil, err
	}
	if !claimed {
		// Another request rotated this token between our read and now.
		// Not reuse — see ErrRefreshTokenRaced. No cascade revocation.
		return nil, entities.ErrRefreshTokenRaced
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
		PersonType:     user.PersonType,
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
	if authLog, err := s.Repos.AuthLogs.ReadByOAuthRefreshTokenID(stored.ID); err == nil {
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
