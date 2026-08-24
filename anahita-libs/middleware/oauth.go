package middleware

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// getViewer returns the Viewer set on the echo context. Returns nil if
// no Viewer is present — callers should never see this on a route
// guarded by Authorize, but SetViewer-guarded routes may.
func getViewer(c echo.Context) *entities.Viewer {
	v, ok := c.Get(constants.ViewerContextKey).(*entities.Viewer)
	if !ok {
		return nil
	}
	return v
}

// GetViewer is the public accessor that handlers call to read the
// Viewer from the echo context. Returns a non-nil unauthenticated
// Viewer for guest contexts so handlers don't need nil checks.
func GetViewer(c echo.Context) *entities.Viewer {
	if v := getViewer(c); v != nil {
		return v
	}
	return entities.NewViewer()
}

// Authorize is a middleware that requires a valid JWT access token.
// Extracts the token from the Authorization header (Bearer) or the
// X-OAuth-Token header (set by nginx auth_request for first-party
// cookie sessions), validates its signature against the JWKS endpoint,
// and constructs a Viewer that downstream handlers can read via
// GetViewer.
//
// Rejects requests with 401 Unauthorized on any validation failure.
func Authorize(config ConfigInterface, jwks *JWKSClient) echo.MiddlewareFunc {
	return func(h echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			viewer, err := authenticateOAuth(c, config, jwks)
			if viewer == nil || err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error":       "unauthorized",
					"description": errString(err),
				})
			}

			c.Set(constants.ViewerContextKey, viewer)
			return h(c)
		}
	}
}

// SetViewer is a middleware that populates the Viewer if a valid token
// is present, but does not reject the request if absent or invalid.
// Use for routes that are publicly accessible but still want to know
// who the caller is when authenticated.
func SetViewer(config ConfigInterface, jwks *JWKSClient) echo.MiddlewareFunc {
	return func(h echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			viewer, err := authenticateOAuth(c, config, jwks)
			if viewer != nil && err == nil {
				c.Set(constants.ViewerContextKey, viewer)
			} else {
				c.Set(constants.ViewerContextKey, entities.NewViewer())
			}
			return h(c)
		}
	}
}

// RequireScope is a middleware that requires the authenticated viewer
// to have a specific scope. Must be chained AFTER Authorize.
func RequireScope(scope string) echo.MiddlewareFunc {
	return func(h echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			viewer := getViewer(c)
			if viewer == nil || !viewer.Authenticated {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "unauthorized",
				})
			}
			if !viewer.HasScope(scope) {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error":       "insufficient_scope",
					"description": fmt.Sprintf("requires scope: %s", scope),
				})
			}
			return h(c)
		}
	}
}

// authenticateOAuth extracts and validates a JWT, returning a
// constructed Viewer on success.
func authenticateOAuth(c echo.Context, config ConfigInterface, jwks *JWKSClient) (*entities.Viewer, error) {
	tokenString, err := extractToken(c)
	if err != nil {
		return nil, err
	}

	parsed, err := jwt.Parse(
		tokenString,
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			kid, _ := token.Header["kid"].(string)
			if kid == "" {
				return nil, errors.New("token missing kid header")
			}
			return jwks.PublicKeyForKID(kid)
		},
		jwt.WithIssuer(config.GetOAuthIssuer()),

		// Pin the algorithm before the keyfunc runs. The RSA type
		// assertion above already blocks alg confusion, so this is
		// defence in depth rather than a hole being closed — but it
		// rejects the token earlier and narrows "any RSA method" to the
		// one algorithm auth-service actually signs with.
		//
		// auth-service pins RS256 in both the local and KMS signers
		// (internal/signing/factory.go). If the planned ES256 migration
		// happens, this list and the SigningMethodRSA assertion above
		// both need updating, and both will fail closed until they are.
		jwt.WithValidMethods([]string{"RS256"}),

		// exp and nbf are otherwise validated with zero tolerance for
		// clock skew, against a token that only lives 15 minutes. A few
		// seconds of drift between the signing pod and a verifying one
		// is ordinary; rejecting on it produces intermittent 401s that
		// look like nothing at all.
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		log.Println("oauth middleware: token parse failed:", err)
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token claims")
	}

	viewer, err := BuildViewer(claims)
	if err != nil {
		log.Println("oauth middleware: build viewer failed:", err)
		return nil, err
	}
	return viewer, nil
}

// extractToken gets the JWT from the Authorization header (third-party
// clients, service-to-service, Electron apps) or from an internal
// header set by nginx auth_request for first-party cookie sessions.
//
// Belt-and-suspenders: if both headers are present, the Authorization
// header wins. This prevents a misconfigured ingress that allows
// inbound X-OAuth-Token from leaking through the Bearer path.
func extractToken(c echo.Context) (string, error) {
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			return "", errors.New("empty bearer token")
		}
		return token, nil
	}

	internalToken := c.Request().Header.Get("X-OAuth-Token")
	if internalToken != "" {
		return internalToken, nil
	}

	return "", errors.New("no token provided")
}

// BuildViewer constructs a Viewer from JWT claims. Claims expected:
//
//	sub                — person UUID string (required)
//	person_type        — usertype role value (guest, registered,
//	                     administrator, super-administrator)
//	person_pk — internal BIGINT identifiers
//	scope              — space-separated scope string
//	client_id          — OAuth client identifier
//	aud                — audience; falls back to client_id if client_id absent
//	refresh_token_id   — BIGINT id of the oauth_refresh_tokens row backing this token
//
// Profile data (name, email) is NOT in the access token — fetch via
// /oauth/userinfo if needed. The middleware deliberately keeps the
// Viewer thin to avoid trusting stale claims after profile changes.
//
// person_type IS populated because authz checks (IsAdmin, IsSuperAdmin)
// depend on it and we don't want every protected request to require a
// gRPC roundtrip.
//
// refresh_token_id is populated so the auth log UI can flag the
// viewer's current session, and so future force-logout flows can
// identify which session a request belongs to without an extra lookup.
func BuildViewer(claims jwt.MapClaims) (*entities.Viewer, error) {
	subStr, _ := claims[constants.JWTClaimSub].(string)
	if subStr == "" {
		return nil, errors.New("missing sub claim")
	}

	personID, err := strconv.ParseInt(subStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid sub int64: %w", err)
	}

	viewer := entities.NewViewer()
	viewer.ID = personID
	viewer.Authenticated = true

	// person_type carries the ROLE (see constants/users.go), which is what
	// IsAdmin/IsSuperAdmin/IsGuest read. It is deliberately not assigned to
	// viewer.Type — that field holds the node entity type set by NewViewer
	// and the node helpers depend on it.
	if personType, ok := claims[constants.JWTClaimPersonType].(string); ok && personType != "" {
		viewer.UserType = personType
	}

	if scope, ok := claims[constants.JWTClaimScope].(string); ok {
		viewer.Scopes = strings.Fields(scope)
	}

	// Internal BIGINT identifiers — used by permissions and DB queries
	// without requiring a per-request hydration lookup.
	if pk, ok := claims[constants.JWTClaimPersonPK].(float64); ok {
		viewer.ID = int64(pk)
	}

	// Refresh token row id — links the viewer back to the
	// oauth_refresh_tokens row that minted the current access token.
	// Empty for client_credentials grants or older tokens.
	if rt, ok := claims[constants.JWTClaimRefreshTokenID].(float64); ok {
		viewer.OAuthRefreshTokenID = int64(rt)
	}

	if clientID, ok := claims[constants.JWTClaimClientID].(string); ok && clientID != "" {
		viewer.ClientID = clientID
	} else if aud, ok := claims[constants.JWTClaimAud].(string); ok {
		viewer.ClientID = aud
	}

	return viewer, nil
}

// errString returns err.Error() or "" if err is nil. Avoids nil-deref
// when constructing error response bodies.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
