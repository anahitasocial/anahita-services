package middleware

type ConfigInterface interface {
	GetRedisAddr() string
	GetRedisPassword() string
	GetRedisDB() int

	// Environment
	IsDev() bool
	IsStaging() bool
	IsProd() bool

	// OAuth — required by the OAuth middleware.
	//
	// GetOAuthIssuer returns the canonical issuer URL of auth-service.
	// Tokens whose "iss" claim does not match this value are rejected.
	// Must be identical across all microservices that share an
	// auth-service deployment (e.g. "https://auth.<your-domain>").
	GetOAuthIssuer() string

	// GetOAuthJWKSURL returns the URL of auth-service's JWKS endpoint
	// (typically "https://auth.<your-domain>/.well-known/jwks.json").
	// The middleware fetches and caches public keys from here to verify
	// JWT signatures locally.
	GetOAuthJWKSURL() string
}
