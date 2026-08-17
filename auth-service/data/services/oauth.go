package services

import (
	"auth/data/repositories"
	"auth/internal/signing"
)

// OAuthService is the root OAuth / OIDC service for Anahita
// auth-service. It composes the repositories and other internal services
// it needs to issue, validate, revoke, and introspect tokens.
//
// Cryptographic signing and verification are delegated to the Signer
// and Verifier interfaces. This lets the same OAuthService code
// operate against either a database-backed local signer (dev, CI) or
// AWS KMS in cloud deployments — OAuthService is unaware of which
// backend is in use. See auth-service/internal/signing for the
// implementations and the factory that selects between them.
type OAuthService struct {
	Repos    *repositories.Repositories
	Srv      *Services
	signer   signing.Signer
	verifier signing.Verifier
	issuer   string
}

// NewOAuthService constructs an OAuthService.
//
// signer and verifier are the configured signing backend and its
// matching verifier. main.go constructs them via signing.NewSigner /
// signing.NewVerifier from config; both must be non-nil.
//
// issuer is the canonical URL of auth-service (e.g.
// "https://auth.anahita.io") and is emitted as the "iss" claim on
// every issued JWT.
func NewOAuthService(
	repos *repositories.Repositories,
	srv *Services,
	signer signing.Signer,
	verifier signing.Verifier,
	issuer string,
) *OAuthService {
	return &OAuthService{
		Repos:    repos,
		Srv:      srv,
		signer:   signer,
		verifier: verifier,
		issuer:   issuer,
	}
}

// Issuer returns the configured issuer URL.
func (s *OAuthService) Issuer() string {
	return s.issuer
}
