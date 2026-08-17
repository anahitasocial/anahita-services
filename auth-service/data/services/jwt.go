package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"auth/data/entities"
	"auth/internal/signing"

	libs_constants "anahita-libs/constants"

	"github.com/golang-jwt/jwt/v5"
)

// Token lifetimes.
const (
	AccessTokenLifetime = 15 * time.Minute
	IDTokenLifetime     = 15 * time.Minute
)

// accessTokenClaims carries the inputs needed to mint an access token.
// Kept as a struct so call sites are self-documenting.
type accessTokenClaims struct {
	Subject    string // person.id int64 string, or client.client_id for client_credentials
	PersonType string // person.type enum value; empty for client_credentials
	ClientID   string
	Scopes     string

	// PersonPK is the internal BIGINT identifier for the
	// person row. Included so downstream services
	// can do permission checks and DB queries without a per-request
	// hydration lookup. Zero for client_credentials grants.
	PersonPK int64

	// RefreshTokenID is the BIGINT id of the oauth_refresh_tokens row
	// that minted (or re-minted via rotation) this access token. Lets
	// downstream code identify "which session does this token belong
	// to" without an extra lookup — used by the auth log UI to flag
	// the viewer's current session, and by future force-logout flows.
	// Zero for client_credentials grants.
	RefreshTokenID int64
}

// MintAccessToken issues a new JWT access token for the given subject.
// Access tokens carry only:
//   - identity (sub, org_id, person_type, person_pk, org_pk)
//   - infrastructure routing (org_country)
//   - session linkage (refresh_token_id)
//   - authorization (scope)
//   - provenance (iss, client_id, aud, jti, iat, exp, typ)
//
// Profile data (name, email) is NOT in the access token — it lives in
// the ID token and /userinfo response. This keeps tokens small and
// avoids stale profile data being trusted across the access token's
// 15-minute lifetime.
//
// person_type IS included because authz decisions in downstream services
// depend on it (IsAdmin, IsSuperAdmin) and we don't want every protected
// request to require a roundtrip back to auth-service. The
// 15-minute staleness window is acceptable for role changes; security-
// sensitive demotions can be paired with refresh token revocation for
// faster propagation.
//
// person_pk (BIGINTs) is included for the same reason at a
// different layer: protected handlers use them as foreign keys for
// notifications, subscriptions, etc. Without them, every authenticated
// request would need a UUID→BIGINT lookup. BIGINTs are immutable for
// the lifetime of the row, so 15-min token staleness is a non-issue.
//
// refresh_token_id ties the access token to its underlying session.
// Auth log UI uses this to highlight "this is your current device";
// force-logout flows use it to revoke the right session.
//
// Cryptographic signing is delegated to s.signer. The signer encapsulates
// whether the private key lives in the database (local mode) or AWS KMS
// (cloud mode); MintAccessToken is unaware of which.
func (s *OAuthService) MintAccessToken(ctx context.Context, claims accessTokenClaims) (string, error) {
	// Resolve the active kid before constructing the token so it can be
	// stamped onto the JWT header. The signer is the source of truth for
	// which key is currently active — callers must not pick a kid.
	kid, err := s.signer.ActiveKeyID(ctx)
	if err != nil {
		return "", fmt.Errorf("get active signing key: %w", err)
	}

	now := time.Now()
	mapClaims := jwt.MapClaims{
		libs_constants.JWTClaimIss:       s.issuer,
		libs_constants.JWTClaimSub:       claims.Subject,
		libs_constants.JWTClaimAud:       claims.ClientID,
		libs_constants.JWTClaimClientID:  claims.ClientID,
		libs_constants.JWTClaimScope:     claims.Scopes,
		libs_constants.JWTClaimExp:       now.Add(AccessTokenLifetime).Unix(),
		libs_constants.JWTClaimIat:       now.Unix(),
		libs_constants.JWTClaimTokenType: libs_constants.JWTTypAccessToken,
	}
	if claims.PersonType != "" {
		mapClaims[libs_constants.JWTClaimPersonType] = claims.PersonType
	}
	if claims.PersonPK != 0 {
		mapClaims[libs_constants.JWTClaimPersonPK] = claims.PersonPK
	}
	if claims.RefreshTokenID != 0 {
		mapClaims[libs_constants.JWTClaimRefreshTokenID] = claims.RefreshTokenID
	}

	// The custom signing method delegates to s.signer when SignedString
	// is called. The "key" argument to SignedString is unused — the
	// signer holds its own key material.
	method := signing.NewJWTSigningMethod(ctx, s.signer)
	token := jwt.NewWithClaims(method, mapClaims)
	token.Header["kid"] = kid
	token.Header["typ"] = libs_constants.JWTTypAccessToken

	return token.SignedString(nil)
}

// idTokenInput carries the inputs needed to mint an OIDC ID token.
type idTokenInput struct {
	Subject  string
	ClientID string
	Nonce    string
	User     *entities.User
	Scopes   string
}

// MintIDToken issues an OIDC ID token. Called only when the "openid"
// scope was granted. Includes profile/email claims according to the
// scopes on the original grant.
func (s *OAuthService) MintIDToken(ctx context.Context, in idTokenInput) (string, error) {
	kid, err := s.signer.ActiveKeyID(ctx)
	if err != nil {
		return "", fmt.Errorf("get active signing key: %w", err)
	}

	now := time.Now()
	claims := jwt.MapClaims{
		libs_constants.JWTClaimIss: s.issuer,
		libs_constants.JWTClaimSub: in.Subject,
		libs_constants.JWTClaimAud: in.ClientID,
		libs_constants.JWTClaimExp: now.Add(IDTokenLifetime).Unix(),
		libs_constants.JWTClaimIat: now.Unix(),
	}
	if in.Nonce != "" {
		claims[libs_constants.JWTClaimNonce] = in.Nonce
	}

	granted := scopeSet(in.Scopes)
	if granted[libs_constants.OAuthScopeProfile] && in.User != nil {
		claims[libs_constants.JWTClaimName] = strings.TrimSpace(in.User.GivenName + " " + in.User.FamilyName)
		claims[libs_constants.JWTClaimGivenName] = in.User.GivenName
		claims[libs_constants.JWTClaimFamilyName] = in.User.FamilyName
	}
	if granted[libs_constants.OAuthScopeEmail] && in.User != nil {
		claims[libs_constants.JWTClaimEmail] = in.User.Email
	}

	method := signing.NewJWTSigningMethod(ctx, s.signer)
	token := jwt.NewWithClaims(method, claims)
	token.Header["kid"] = kid

	return token.SignedString(nil)
}

// ValidateAccessToken parses a JWT access token, verifies its signature
// via s.verifier, and validates the registered claims (exp, nbf).
//
// The implementation manually splits the JWT and dispatches verification
// to the Verifier interface rather than using jwt.Parse with a keyfunc.
// This is necessary because our signing.NewJWTSigningMethod adapter
// deliberately rejects the SigningMethod.Verify path — verification
// belongs to the Verifier interface, which has the right shape (kid +
// alg + signing input + signature) and is independent of the signing
// backend.
func (s *OAuthService) ValidateAccessToken(ctx context.Context, tokenString string) (*jwt.Token, error) {
	// ParseUnverified populates the token structure (header, claims) and
	// returns the three base64url-encoded segments. It does NOT verify
	// the signature or validate registered claims — both happen below.
	parser := jwt.NewParser()
	token, parts, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("parse token structure: %w", err)
	}

	kid, _ := token.Header["kid"].(string)
	alg, _ := token.Header["alg"].(string)
	if kid == "" {
		return nil, errors.New("token missing kid header")
	}
	if alg == "" {
		return nil, errors.New("token missing alg header")
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	// Signing input is the first two base64url segments joined by "."
	// — the exact bytes the signer signed when minting. ParseUnverified
	// guarantees parts has length 3 when err is nil.
	signingInput := []byte(parts[0] + "." + parts[1])

	// Verify the signature. The Verifier handles kid lookup, the
	// alg-confusion defense (token alg must match the key's registered
	// alg), and the cryptographic check.
	if err := s.verifier.Verify(ctx, kid, alg, signingInput, signature); err != nil {
		return nil, fmt.Errorf("verify signature: %w", err)
	}

	// Validate registered time-based claims. Match jwt.Parse's default
	// behavior: exp and nbf are checked when present, not required.
	// Our minted tokens always include exp, but a missing exp on a
	// caller-supplied token isn't necessarily forged (it could be an
	// older token format), so don't reject on absence.
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("unexpected claims type")
	}
	now := time.Now().Unix()
	if exp, ok := claimFloat(claims, libs_constants.JWTClaimExp); ok {
		if now > int64(exp) {
			return nil, errors.New("token expired")
		}
	}
	if nbf, ok := claimFloat(claims, "nbf"); ok {
		if now < int64(nbf) {
			return nil, errors.New("token not yet valid")
		}
	}

	token.Signature = signature
	token.Valid = true
	return token, nil
}

// AccessTokenInfo is the typed representation of claims extracted from a
// parsed access token.
type AccessTokenInfo struct {
	Subject    string
	PersonType string
	ClientID   string
	Scopes     string
	JTI        string
	IssuedAt   time.Time
	ExpiresAt  time.Time

	// Internal BIGINT identifiers — populated when the token carries
	// person_pk claims. Zero for client_credentials grants
	// or older tokens minted before these claims existed.
	PersonPK int64

	// Refresh token row ID — populated for tokens minted via the
	// authorization_code or refresh_token grant. Zero for
	// client_credentials grants or tokens minted before this claim
	// existed.
	RefreshTokenID int64
}

// ExtractAccessTokenClaims pulls Anahita claims out of a parsed
// access token into a typed struct.
func (s *OAuthService) ExtractAccessTokenClaims(token *jwt.Token) (*AccessTokenInfo, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	info := &AccessTokenInfo{
		Subject:    claimString(claims, libs_constants.JWTClaimSub),
		PersonType: claimString(claims, libs_constants.JWTClaimPersonType),
		ClientID:   claimString(claims, libs_constants.JWTClaimClientID),
		Scopes:     claimString(claims, libs_constants.JWTClaimScope),
		JTI:        claimString(claims, libs_constants.JWTClaimJti),
	}

	if exp, ok := claimFloat(claims, libs_constants.JWTClaimExp); ok {
		info.ExpiresAt = time.Unix(int64(exp), 0)
	}
	if iat, ok := claimFloat(claims, libs_constants.JWTClaimIat); ok {
		info.IssuedAt = time.Unix(int64(iat), 0)
	}

	if pk, ok := claimFloat(claims, libs_constants.JWTClaimPersonPK); ok {
		info.PersonPK = int64(pk)
	}
	if rt, ok := claimFloat(claims, libs_constants.JWTClaimRefreshTokenID); ok {
		info.RefreshTokenID = int64(rt)
	}

	// client_id claim was added to make parsing unambiguous; fall back to
	// aud for older tokens.
	if info.ClientID == "" {
		info.ClientID = claimString(claims, libs_constants.JWTClaimAud)
	}

	return info, nil
}

// claimString safely extracts a string claim, returning "" if missing
// or of the wrong type.
func claimString(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// claimFloat safely extracts a numeric claim as float64 (JWT numeric
// claims decode as float64 in jwt.MapClaims).
func claimFloat(claims jwt.MapClaims, key string) (float64, bool) {
	v, ok := claims[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}
