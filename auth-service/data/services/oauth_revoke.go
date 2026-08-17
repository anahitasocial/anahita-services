package services

import (
	"context"
	"strconv"
	"time"

	"auth/data/entities"
)

// RevokeToken implements RFC 7009 token revocation.
//
// Anahita revokes refresh tokens only. Access tokens are
// short-lived (15 minutes) and rely on natural expiry.
func (s *OAuthService) RevokeToken(
	token string,
	tokenTypeHint string,
	client *entities.OAuthClient,
) error {
	if tokenTypeHint == "access_token" {
		return nil
	}

	stored, err := s.Repos.OAuthRefreshTokens.ReadByTokenHash(HashToken(token))
	if err != nil {
		// Per RFC 7009 §2.2: respond 200 even if the token wasn't found.
		return nil
	}

	if stored.ClientID != client.ClientID {
		return nil
	}

	return s.Repos.OAuthRefreshTokens.Revoke(stored.ID)
}

// IntrospectToken implements RFC 7662 token introspection.
//
// Returns Active=true for live tokens, Active=false for expired,
// revoked, or unknown tokens.
//
// Introspection runs without an ambient user session, so the refresh-
// token path resolves the person from the token row. The access-token
// path is cheap because the identifiers are already in the JWT claims.
func (s *OAuthService) IntrospectToken(
	ctx context.Context,
	token string,
	tokenTypeHint string,
	client *entities.OAuthClient,
) (*entities.OAuthTokenInfo, error) {
	if tokenTypeHint != "refresh_token" {
		if info := s.introspectAccessToken(ctx, token, client); info != nil {
			return info, nil
		}
	}

	if tokenTypeHint != "access_token" {
		if info := s.introspectRefreshToken(token, client); info != nil {
			return info, nil
		}
	}

	return &entities.OAuthTokenInfo{Active: false}, nil
}

func (s *OAuthService) introspectAccessToken(
	ctx context.Context,
	token string,
	client *entities.OAuthClient,
) *entities.OAuthTokenInfo {
	parsed, err := s.ValidateAccessToken(ctx, token)
	if err != nil || !parsed.Valid {
		return nil
	}
	info, err := s.ExtractAccessTokenClaims(parsed)
	if err != nil {
		return nil
	}

	if info.ClientID != client.ClientID {
		return &entities.OAuthTokenInfo{Active: false}
	}

	return &entities.OAuthTokenInfo{
		Active:    true,
		Scope:     info.Scopes,
		ClientID:  info.ClientID,
		Sub:       info.Subject,
		Exp:       info.ExpiresAt.Unix(),
		Iat:       info.IssuedAt.Unix(),
		Iss:       s.issuer,
		Jti:       info.JTI,
		TokenType: "Bearer",
	}
}

// introspectRefreshToken does not take a context because none of its
// underlying calls are context-aware (the gRPC clients in this
// codebase use their own internal timeouts). Add a context parameter
// here only when adding genuinely ctx-aware operations to this path.
func (s *OAuthService) introspectRefreshToken(
	token string,
	client *entities.OAuthClient,
) *entities.OAuthTokenInfo {
	stored, err := s.Repos.OAuthRefreshTokens.ReadByTokenHash(HashToken(token))
	if err != nil {
		return nil
	}
	if stored.Revoked || time.Now().After(stored.ExpiresAt) {
		return &entities.OAuthTokenInfo{Active: false}
	}
	if stored.ClientID != client.ClientID {
		return &entities.OAuthTokenInfo{Active: false}
	}

	user, err := s.Repos.Users.ReadByID(stored.UserID)
	if err != nil {
		return nil
	}

	return &entities.OAuthTokenInfo{
		Active:    true,
		Scope:     stored.Scopes,
		ClientID:  stored.ClientID,
		Sub:       strconv.FormatInt(user.ID, 10),
		Exp:       stored.ExpiresAt.Unix(),
		Iat:       stored.CreatedAt.Unix(),
		Iss:       s.issuer,
		TokenType: "refresh_token",
	}
}
