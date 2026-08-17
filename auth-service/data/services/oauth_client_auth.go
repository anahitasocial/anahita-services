package services

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"

	"auth/data/entities"
)

// ClientCredentials carries an OAuth client_id and (optional)
// client_secret extracted from a request. For confidential clients,
// SecretProvided will be true and Secret will be populated. For public
// clients (e.g. PKCE-only Electron apps), SecretProvided is false.
type ClientCredentials struct {
	ClientID       string
	Secret         string
	SecretProvided bool
}

// ExtractClientCredentials pulls client credentials from the request,
// supporting both standard OAuth methods:
//
//   - HTTP Basic auth via Authorization header (RFC 6749 §2.3.1).
//     Header value is "Basic <base64(client_id:client_secret)>".
//   - Form fields client_id and client_secret in the request body
//     (RFC 6749 §2.3.1, fallback method).
//
// Per RFC 6749 §2.3.1, clients MUST NOT use both methods in the same
// request. We enforce this defensively: if Basic auth is present, form
// fields are ignored entirely (whichever was the real intent, the spec
// says only one wins).
//
// Returns an error if no credentials are found at all. Returning
// SecretProvided=false (with no error) is reserved for public clients
// that legitimately omit the secret.
func (s *OAuthService) ExtractClientCredentials(
	authHeader string,
	formClientID string,
	formClientSecret string,
) (*ClientCredentials, error) {
	// Try HTTP Basic auth first — it's the RFC's preferred method.
	if strings.HasPrefix(authHeader, "Basic ") {
		encoded := strings.TrimPrefix(authHeader, "Basic ")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, errors.New("invalid Authorization header encoding")
		}

		// The decoded value is "client_id:client_secret". The colon is the
		// only separator; the secret may itself contain colons.
		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, errors.New("malformed Basic auth credentials")
		}

		return &ClientCredentials{
			ClientID:       parts[0],
			Secret:         parts[1],
			SecretProvided: parts[1] != "",
		}, nil
	}

	// Fall back to form fields.
	if formClientID == "" {
		return nil, errors.New("client_id is required")
	}

	return &ClientCredentials{
		ClientID:       formClientID,
		Secret:         formClientSecret,
		SecretProvided: formClientSecret != "",
	}, nil
}

// AuthenticateClient resolves the OAuth client by client_id and
// verifies the credentials match the client's registered authentication
// method.
//
// Authentication rules:
//   - Confidential clients MUST present a secret. The secret is
//     compared against the stored bcrypt hash in constant time.
//   - Public clients MUST NOT present a secret. (PKCE provides the
//     proof of possession at the /authorize and /token steps.) A public
//     client presenting a secret is rejected — sloppy client code
//     shouldn't accidentally compromise the public-client model.
//   - Inactive clients are rejected.
func (s *OAuthService) AuthenticateClient(creds *ClientCredentials) (*entities.OAuthClient, error) {
	if creds == nil || creds.ClientID == "" {
		return nil, errors.New("client_id is required")
	}

	client, err := s.Repos.OAuthClients.ReadByClientID(creds.ClientID)
	if err != nil {
		return nil, errors.New("invalid client")
	}

	if !client.Active {
		return nil, errors.New("client is inactive")
	}

	if client.Confidential {
		if !creds.SecretProvided {
			return nil, errors.New("client_secret is required for confidential clients")
		}
		if !verifyClientSecret(creds.Secret, client.SecretHash) {
			return nil, errors.New("invalid client credentials")
		}
		return client, nil
	}

	// Public client.
	if creds.SecretProvided {
		return nil, errors.New("public clients must not present a client_secret")
	}
	return client, nil
}

// AuthenticateClientFromRequest is a convenience wrapper for handlers:
// extracts credentials and authenticates in one call. Used by every
// OAuth endpoint that requires client authentication (/token, /revoke,
// /introspect).
func (s *OAuthService) AuthenticateClientFromRequest(
	authHeader string,
	formClientID string,
	formClientSecret string,
) (*entities.OAuthClient, error) {
	creds, err := s.ExtractClientCredentials(authHeader, formClientID, formClientSecret)
	if err != nil {
		return nil, err
	}
	return s.AuthenticateClient(creds)
}

// verifyClientSecret compares a plaintext secret against a stored hash
// in constant time. The hash format is whatever HashToken produced at
// client creation — a SHA-256 digest, hex-encoded.
//
// We use HashToken (SHA-256) rather than bcrypt for client secrets
// because client secrets are high-entropy random strings (48 bytes from
// generateRandomString), not user-chosen passwords. The
// computational-cost defense bcrypt provides is wasted on inputs that
// are already infeasible to brute-force; constant-time comparison is
// the only property we need.
func verifyClientSecret(plaintext, storedHash string) bool {
	if plaintext == "" || storedHash == "" {
		return false
	}
	hashOfPresented := HashToken(plaintext)
	return subtle.ConstantTimeCompare([]byte(hashOfPresented), []byte(storedHash)) == 1
}
