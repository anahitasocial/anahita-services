package handlers

import (
	"slices"

	"log"
	"net/http"

	libs_constants "anahita-libs/constants"

	"github.com/labstack/echo/v4"
)

// OpenIDConfiguration is the OpenID Connect discovery document (OpenID
// Connect Discovery 1.0, section 3): where each endpoint is, and what the
// installation supports.
type OpenIDConfiguration struct {
	// Who issues the tokens: the `iss` in each.
	Issuer string `json:"issuer"`
	// `/oauth/authorize`.
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	// `/oauth/token`.
	TokenEndpoint string `json:"token_endpoint"`
	// `/oauth/userinfo`.
	UserinfoEndpoint string `json:"userinfo_endpoint"`
	// `/.well-known/jwks.json`.
	JWKSURI string `json:"jwks_uri"`
	// `/oauth/revoke`.
	RevocationEndpoint string `json:"revocation_endpoint"`
	// `/oauth/introspect`.
	IntrospectionEndpoint string `json:"introspection_endpoint"`
	// `code`.
	ResponseTypesSupported []string `json:"response_types_supported"`
	// `authorization_code`, `refresh_token` and `client_credentials`.
	GrantTypesSupported []string `json:"grant_types_supported"`
	// `public`.
	SubjectTypesSupported []string `json:"subject_types_supported"`
	// `RS256`.
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	// Every scope there is (oauth.md).
	ScopesSupported []string `json:"scopes_supported"`
	// `client_secret_basic` and `client_secret_post`.
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	// What a token or userinfo may say.
	ClaimsSupported []string `json:"claims_supported"`
	// `S256` and `plain`.
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
}

// JWKSet is the keys a token's signature is checked with (RFC 7517).
type JWKSet struct {
	// Each a JSON Web Key: `kid`, `kty`, `alg`, `use`, `n` and `e`.
	Keys []map[string]any `json:"keys"`
}

// Discovery handles GET /.well-known/openid-configuration
//
// Returns the OIDC discovery document with all supported endpoints,
// grant types, scopes, and signing algorithms. Public endpoint — no
// authentication required.
func (h *oauthHandler) Discovery(c echo.Context) error {
	issuer := h.Config.GetOAuthIssuer()

	discovery := &OpenIDConfiguration{
		Issuer:                issuer,
		AuthorizationEndpoint: issuer + "/oauth/authorize",
		TokenEndpoint:         issuer + "/oauth/token",
		UserinfoEndpoint:      issuer + "/oauth/userinfo",
		JWKSURI:               issuer + "/.well-known/jwks.json",
		RevocationEndpoint:    issuer + "/oauth/revoke",
		IntrospectionEndpoint: issuer + "/oauth/introspect",

		ResponseTypesSupported: []string{
			"code",
		},

		GrantTypesSupported: []string{
			"authorization_code",
			"refresh_token",
			"client_credentials",
		},

		SubjectTypesSupported: []string{
			"public",
		},

		IDTokenSigningAlgValuesSupported: []string{
			"RS256",
		},

		ScopesSupported: supportedScopes(),

		TokenEndpointAuthMethodsSupported: []string{
			"client_secret_basic",
			"client_secret_post",
		},

		ClaimsSupported: []string{
			// Standard OIDC / OAuth claims
			libs_constants.JWTClaimSub,
			libs_constants.JWTClaimIss,
			libs_constants.JWTClaimAud,
			libs_constants.JWTClaimExp,
			libs_constants.JWTClaimIat,
			libs_constants.JWTClaimNonce,
			libs_constants.JWTClaimScope,
			libs_constants.JWTClaimClientID,

			// OIDC profile claims (in ID token / userinfo)
			libs_constants.JWTClaimName,
			libs_constants.JWTClaimEmail,

			// Anahita custom claims
			libs_constants.JWTClaimPersonType,
			libs_constants.JWTClaimPersonPK,
		},

		CodeChallengeMethodsSupported: []string{
			"S256",
			"plain",
		},
	}

	return c.JSON(http.StatusOK, discovery)
}

// JWKS handles GET /.well-known/jwks.json
//
// Returns the public signing keys in JWK Set format for token
// verification by clients and resource servers. Public endpoint — no
// authentication required. The middleware in every microservice fetches
// this on startup and on kid-miss to validate JWTs locally.
func (h *oauthHandler) JWKS(c echo.Context) error {
	keys, err := h.Repos.OAuthSigningKeys.BrowseVerificationKeys()
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	jwks := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		jwk, err := h.OAuth.BuildJWK(key)
		if err != nil {
			log.Println("failed to build JWK for key", key.KeyID, err)
			continue
		}
		jwks = append(jwks, jwk)
	}

	return c.JSON(http.StatusOK, &JWKSet{Keys: jwks})
}

// supportedScopes returns the full list of scopes advertised in the
// discovery document. Sourced from the canonical scope registry in
// anahita-libs so additions to the catalog automatically appear in
// discovery without having to edit two places.
//
// Deliberately unfiltered. Public is false for all but the four OIDC
// scopes, so filtering on it would advertise almost nothing and mislead
// the first-party apps that are the only consumers today. Per RFC 8414
// scopes_supported is documentation, not authorization — the real gates
// are ValidateScopesForClient and AvailableScopesForConsent. Revisit
// when the first third-party client is registered.
//
// Sorted because map iteration order is random, which would otherwise
// change the discovery document on every request.
func supportedScopes() []string {
	scopes := make([]string, 0, len(libs_constants.OAuthScopeDefinitions))
	for scope := range libs_constants.OAuthScopeDefinitions {
		scopes = append(scopes, scope)
	}
	slices.Sort(scopes)
	return scopes
}
