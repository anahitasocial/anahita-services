package services

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"auth/data/entities"

	libs_entities "anahita-libs/data/entities"

	"github.com/labstack/echo/v4"

	"anahita-libs/sessions"
)

// AuthorizationCodeLifetime is the validity window for an authorization
// code (RFC 6749 recommends 10 minutes maximum).
const AuthorizationCodeLifetime = 10 * time.Minute

// Session keys used during the /authorize -> consent -> issue flow.
const (
	sessionKeyAuthRequest = "oauth_auth_request"
	sessionKeyCSRFToken   = "oauth_csrf_token"
)

// StoreAuthRequest saves the JSON-serialized authorization request to the
// viewer's session so it can be retrieved on the consent POST.
func (s *OAuthService) StoreAuthRequest(session sessions.SessionInterface, c echo.Context, reqJSON string) error {
	_, err := session.Add(c, sessionKeyAuthRequest, reqJSON)
	return err
}

// RetrieveAuthRequest reads the stored authorization request from session.
func (s *OAuthService) RetrieveAuthRequest(session sessions.SessionInterface, c echo.Context) (string, error) {
	return session.Read(c, sessionKeyAuthRequest)
}

// ClearAuthRequest removes the authorization request from session. Called
// after the code is issued or on any terminal error.
func (s *OAuthService) ClearAuthRequest(session sessions.SessionInterface, c echo.Context) error {
	return session.Delete(c, sessionKeyAuthRequest)
}

// GenerateCSRFToken creates a random token and stores it in the session
// under the oauth_csrf_token key. The token is single-use and removed by
// ValidateCSRFToken.
func (s *OAuthService) GenerateCSRFToken(session sessions.SessionInterface, c echo.Context) (string, error) {
	token, err := generateRandomToken(32)
	if err != nil {
		return "", err
	}
	if _, err := session.Add(c, sessionKeyCSRFToken, token); err != nil {
		return "", err
	}
	return token, nil
}

// ValidateCSRFToken verifies a submitted CSRF token against the session
// and deletes it afterwards (single-use semantics).
func (s *OAuthService) ValidateCSRFToken(session sessions.SessionInterface, c echo.Context, submitted string) error {
	stored, err := session.Read(c, sessionKeyCSRFToken)
	if err != nil {
		return errors.New("csrf token not found in session")
	}
	if !ConstantTimeEqual(stored, submitted) {
		return errors.New("csrf token mismatch")
	}
	_ = session.Delete(c, sessionKeyCSRFToken)
	return nil
}

// AuthorizationGrant describes the user's granted authorization request
// that is about to become a persisted authorization code. It exists so
// that callers pass one struct rather than six positional arguments.
type AuthorizationGrant struct {
	Client              *entities.OAuthClient
	RedirectURI         string
	Scopes              string
	CodeChallenge       string
	CodeChallengeMethod string
	Nonce               string
}

// IssueAuthorizationCode generates and persists a single-use authorization
// code on behalf of the authenticated Viewer. Returns the plaintext code
// (which is only ever returned here — the database only stores the hash).
//
// This is called from the /authorize consent POST handler after the user
// has approved the request.
func (s *OAuthService) IssueAuthorizationCode(
	viewer *libs_entities.Viewer,
	grant AuthorizationGrant,
) (string, error) {
	if viewer == nil {
		return "", errors.New("viewer is required to issue authorization code")
	}
	if grant.Client == nil {
		return "", errors.New("client is required to issue authorization code")
	}

	code, err := generateRandomToken(32)
	if err != nil {
		return "", err
	}

	authCode := entities.NewOAuthAuthorization()
	authCode.CodeHash = HashToken(code)
	authCode.ClientID = grant.Client.ClientID
	authCode.UserID = viewer.ID
	authCode.RedirectURI = grant.RedirectURI
	authCode.Scopes = grant.Scopes
	authCode.CodeChallenge = grant.CodeChallenge
	authCode.CodeChallengeMethod = grant.CodeChallengeMethod
	authCode.Nonce = grant.Nonce
	authCode.ExpiresAt = time.Now().Add(AuthorizationCodeLifetime)

	if _, err := s.Repos.OAuthAuthorizations.Add(authCode); err != nil {
		return "", fmt.Errorf("failed to persist authorization code: %w", err)
	}

	return code, nil
}

// ConsumeAuthorizationCode exchanges a plaintext authorization code for
// its stored record, verifying client identity, redirect URI, expiry, and
// PKCE challenge in one call. Deletes the code on success or on any
// terminal failure to prevent replay.
func (s *OAuthService) ConsumeAuthorizationCode(
	code string,
	client *entities.OAuthClient,
	redirectURI string,
	codeVerifier string,
) (*entities.OAuthAuthorization, error) {
	authCode, err := s.Repos.OAuthAuthorizations.ReadByCodeHash(HashToken(code))
	if err != nil {
		return nil, entities.ErrAuthorizationNotFound
	}

	// Delete first to guarantee single-use semantics even if validation
	// below fails and callers retry. Single-use is a security property,
	// not a convenience — an attacker who obtains a code should not be
	// able to use it twice regardless of intermediate errors.
	defer func() {
		_ = s.Repos.OAuthAuthorizations.Delete(authCode.ID)
	}()

	if authCode.IsExpired() {
		return nil, entities.ErrAuthorizationExpired
	}
	if authCode.ClientID != client.ClientID {
		return nil, entities.ErrAuthorizationClientMis
	}
	if authCode.RedirectURI != redirectURI {
		return nil, entities.ErrAuthorizationRedirectMis
	}

	if authCode.RequiresPKCE() || !client.Confidential {
		// Public clients MUST use PKCE (OAuth 2.1).
		if codeVerifier == "" {
			return nil, entities.ErrPKCERequired
		}
		if err := verifyPKCE(authCode.CodeChallenge, authCode.CodeChallengeMethod, codeVerifier); err != nil {
			return nil, err
		}
	}

	return authCode, nil
}

// verifyPKCE validates a code_verifier against a stored code_challenge
// per RFC 7636.
func verifyPKCE(challenge, method, verifier string) error {
	switch method {
	case entities.CodeChallengeMethodS256, "":
		hash := sha256.Sum256([]byte(verifier))
		computed := base64.RawURLEncoding.EncodeToString(hash[:])
		if !ConstantTimeEqual(computed, challenge) {
			return entities.ErrPKCEVerificationFailed
		}
	case entities.CodeChallengeMethodPlain:
		if !ConstantTimeEqual(verifier, challenge) {
			return entities.ErrPKCEVerificationFailed
		}
	default:
		return entities.ErrUnsupportedChallengeMethod
	}
	return nil
}
