package handlers

import (
	"log"
	"net/http"

	libs_constants "anahita-libs/constants"

	"github.com/labstack/echo/v4"
)

// Discovery handles GET /.well-known/openid-configuration
//
// Returns the OIDC discovery document with all supported endpoints,
// grant types, scopes, and signing algorithms. Public endpoint — no
// authentication required.
func (h *oauthHandler) Discovery(c echo.Context) error {
	issuer := h.Config.GetOAuthIssuer()

	discovery := map[string]any{
		"issuer":                 issuer,
		"authorization_endpoint": issuer + "/oauth/authorize",
		"token_endpoint":         issuer + "/oauth/token",
		"userinfo_endpoint":      issuer + "/oauth/userinfo",
		"jwks_uri":               issuer + "/.well-known/jwks.json",
		"revocation_endpoint":    issuer + "/oauth/revoke",
		"introspection_endpoint": issuer + "/oauth/introspect",

		"response_types_supported": []string{
			"code",
		},

		"grant_types_supported": []string{
			"authorization_code",
			"refresh_token",
			"client_credentials",
		},

		"subject_types_supported": []string{
			"public",
		},

		"id_token_signing_alg_values_supported": []string{
			"RS256",
		},

		"scopes_supported": supportedScopes(),

		"token_endpoint_auth_methods_supported": []string{
			"client_secret_basic",
			"client_secret_post",
		},

		"claims_supported": []string{
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
			libs_constants.JWTClaimGivenName,
			libs_constants.JWTClaimFamilyName,
			libs_constants.JWTClaimEmail,

			// Anahita custom claims
			libs_constants.JWTClaimPersonType,
			libs_constants.JWTClaimPersonPK,
		},

		"code_challenge_methods_supported": []string{
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

	return c.JSON(http.StatusOK, map[string]any{
		"keys": jwks,
	})
}

// supportedScopes returns the full list of scopes advertised in the
// discovery document. Sourced from the canonical scope registry in
// anahita-libs so additions to the catalog automatically appear in
// discovery without having to edit two places.
func supportedScopes() []string {
	scopes := make([]string, 0, len(libs_constants.OAuthScopeDefinitions))
	for scope := range libs_constants.OAuthScopeDefinitions {
		scopes = append(scopes, scope)
	}
	return scopes
}
