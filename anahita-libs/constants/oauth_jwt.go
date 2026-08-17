package constants

// JWT claim names used by Anahita OAuth tokens. Source of truth
// for both auth-service (which mints tokens) and the Anahita OAuth
// middleware (which consumes them in every microservice).
//
// Standard OIDC/OAuth claims use the lowercase abbreviations from
// RFC 7519 / OIDC Core. anahita-specific claims are namespaced
// to avoid collision with future standard claims.
const (
	// Standard OAuth / OIDC claims.
	JWTClaimSub      = "sub"
	JWTClaimIss      = "iss"
	JWTClaimAud      = "aud"
	JWTClaimScope    = "scope"
	JWTClaimExp      = "exp"
	JWTClaimIat      = "iat"
	JWTClaimJti      = "jti"
	JWTClaimNonce    = "nonce"
	JWTClaimClientID = "client_id"

	// JWT type header / claim (RFC 9068).
	JWTClaimTokenType = "typ"
	JWTTypAccessToken = "at+JWT"
	JWTTypIDToken     = "ID"

	// OIDC standard Anahita claims (in ID token / /userinfo response).
	JWTClaimName       = "name"
	JWTClaimGivenName  = "given_name"
	JWTClaimFamilyName = "family_name"
	JWTClaimEmail      = "email"

	// Anahita-specific claims (in access token).
	// usertype role value: guest, registered, administrator,
	// super-administrator. Hydrated into Viewer.UserType, which is what
	// IsAdmin/IsSuperAdmin/IsGuest read — NOT Viewer.Type, which is the
	// node entity type.
	JWTClaimPersonType = "person_type"

	// Stable internal identifiers — included in access tokens to avoid
	// per-request DB lookups for permission checks. BIGINTs are immutable
	// for the lifetime of the row, so 15-min token staleness is not a
	// concern. Decoded as float64 in Go (JSON number convention).
	JWTClaimPersonPK = "person_pk"

	JWTClaimRefreshTokenID = "refresh_token_id"
)
