package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"auth/data/entities"
	"auth/requests"
	"auth/responses"

	libs_constants "anahita-libs/constants"
	"anahita-libs/sessions"

	"github.com/labstack/echo/v4"
)

const (
	sessionKeyOAuthTokens     = "oauth_tokens"
	oauthRefreshLockKeyPrefix = "oauth:refresh_lock:"
	oauthRefreshLockTTL       = 5 * time.Second
	oauthRefreshLockWait      = 200 * time.Millisecond
)

type storedOAuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// AddSession handles POST /oauth/session
func (h *oauthHandler) AddSession(c echo.Context) error {
	req := requests.NewOAuthTokenRequest().Token

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	client, err := h.OAuth.AuthenticateClientFromRequest(
		c.Request().Header.Get("Authorization"),
		req.ClientID,
		req.ClientSecret,
	)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusUnauthorized, &responses.OAuthErrorResponse{
			Error:       "invalid_client",
			Description: err.Error(),
		})
	}

	authCode, err := h.OAuth.ConsumeAuthorizationCode(
		req.Code,
		client,
		req.RedirectURI,
		req.CodeVerifier,
	)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusBadRequest, &responses.OAuthErrorResponse{
			Error:       "invalid_grant",
			Description: err.Error(),
		})
	}

	viewer, err := h.OAuth.ResolveViewerByID(authCode.UserID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	user, err := h.Repos.Users.ReadByID(authCode.UserID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	tokenPair, err := h.OAuth.IssueTokenPairForViewer(
		c.Request().Context(),
		user,
		client,
		authCode.Scopes,
		authCode.Nonce,
	)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.writeOAuthTokensSession(c, tokenPair); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.recordAuthLogAfterIssuance(c, viewer, client.ClientID, tokenPair.RefreshToken)

	return c.JSON(http.StatusOK, map[string]string{
		"status": "authenticated",
	})
}

// ReadValidate handles GET /oauth/validate
func (h *oauthHandler) ReadValidate(c echo.Context) error {
	ctx := c.Request().Context()

	authHeader := c.Request().Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString != "" {
			if _, err := h.OAuth.ValidateAccessToken(ctx, tokenString); err == nil {
				c.Response().Header().Set("X-OAuth-Token", tokenString)
			}
		}
		return c.NoContent(http.StatusOK)
	}

	tokens, err := h.readOAuthTokensSession(c)
	if err != nil {
		return c.NoContent(http.StatusOK)
	}

	if _, err := h.OAuth.ValidateAccessToken(ctx, tokens.AccessToken); err == nil {
		c.Response().Header().Set("X-OAuth-Token", tokens.AccessToken)
		return c.NoContent(http.StatusOK)
	}

	newAccessToken, err := h.refreshSessionTokens(c, tokens)
	if err != nil {
		log.Println("validate: refresh failed:", err)
		_ = h.Session.Delete(c, sessionKeyOAuthTokens)
		return c.NoContent(http.StatusOK)
	}

	c.Response().Header().Set("X-OAuth-Token", newAccessToken)
	return c.NoContent(http.StatusOK)
}

// ReadLogout handles GET /oauth/logout — navigation-style logout that
// works across the BFF and auth subdomains.
//
// The two auth cookies live on different hosts. Each app has its own
// BFF hostname (api.staging.* for the Anahita web app) where
// oauth_tokens lives; person_session lives on the auth host
// (auth.staging.*). Browsers only accept Set-Cookie responses
// that target the same host the cookie was set on, so a single
// endpoint cannot delete both — we have to walk the chain.
//
// This handler dispatches based on the request's Host:
//   - On a BFF host: revoke the refresh token, delete oauth_tokens,
//     redirect to the same endpoint on the auth host.
//   - On the auth host: delete person_session, redirect to the
//     return_to URL.
//
// The same code serves any BFF host unchanged — IsAuthHost(host)
// returns false for all of them, so each falls into the "first leg"
// branch and clears its per-host oauth_tokens cookie before bouncing
// to the auth host. Additional first-party apps need no changes here.
//
// In dev, both cookies live on localhost so the chain collapses to
// a single hop that clears both.
//
// The frontend kicks off the chain by navigating to:
//
//	GET https://<bff-host>/oauth/logout?return_to=<app-url>
//
// where <bff-host> is the SPA's own BFF (the Anahita web app →
// api.staging.*).
//
// SECURITY: return_to is validated against the Config-driven allowlist
// of registered app origins. An attacker who could craft a link with
// a malicious return_to value would otherwise be able to phish users
// by sending them through a "logout" that lands on a fake login page.
func (h *oauthHandler) ReadLogout(c echo.Context) error {
	returnTo := h.resolveLogoutReturnTo(c)
	host := c.Request().Host

	// Dev: both cookies on localhost, single hop clears both.
	if h.Config.IsDev() {
		h.revokeRefreshTokenIfPresent(c)
		_ = h.Session.Delete(c, sessionKeyOAuthTokens)
		_ = h.Session.Delete(c, libs_constants.UserSessionKey)
		return c.Redirect(http.StatusFound, returnTo)
	}

	// Final leg: on the auth subdomain, clear IdP session and bounce
	// back to the app.
	if h.Config.IsAuthHost(host) {
		_ = h.Session.Delete(c, libs_constants.UserSessionKey)
		return c.Redirect(http.StatusFound, returnTo)
	}

	// First leg: on a BFF host (api.staging.* or admin-api.staging.*),
	// revoke refresh token + clear oauth_tokens, then hop to the auth
	// subdomain to finish.
	h.revokeRefreshTokenIfPresent(c)
	_ = h.Session.Delete(c, sessionKeyOAuthTokens)

	authLogoutURL := fmt.Sprintf("%s/oauth/logout?return_to=%s",
		h.Config.GetAuthURL(),
		url.QueryEscape(returnTo))
	return c.Redirect(http.StatusFound, authLogoutURL)
}

// revokeRefreshTokenIfPresent reads the OAuth tokens from the BFF
// session and revokes the refresh token in the database. No-op if
// there's no session or no refresh token in it.
func (h *oauthHandler) revokeRefreshTokenIfPresent(c echo.Context) {
	tokens, err := h.readOAuthTokensSession(c)
	if err != nil || tokens.RefreshToken == "" {
		return
	}
	hash := h.OAuth.HashToken(tokens.RefreshToken)
	rt, err := h.Repos.OAuthRefreshTokens.ReadByTokenHash(hash)
	if err != nil {
		return
	}
	_ = h.Repos.OAuthRefreshTokens.Revoke(rt.ID)
}

// resolveLogoutReturnTo extracts the return_to query param and
// validates it against the Config allowlist. Falls back to the
// default frontend URL on missing or invalid values, with the
// rejection logged so suspicious traffic is auditable.
func (h *oauthHandler) resolveLogoutReturnTo(c echo.Context) string {
	requested := c.QueryParam("return_to")
	if requested == "" {
		return h.Config.GetFrontendURL()
	}
	if !h.isAllowedLogoutReturnTo(requested) {
		log.Println("logout: rejected return_to:", requested)
		return h.Config.GetFrontendURL()
	}
	return requested
}

// isAllowedLogoutReturnTo reports whether the candidate URL is safe
// to redirect to after logout. We accept only URLs whose origin
// (scheme + host) matches one of the entries returned by
// Config.GetAllowedAppOrigins for the current environment.
//
// Outside of dev, only HTTPS schemes are accepted — an attacker who
// could downgrade to HTTP would otherwise have a position to capture
// any state the app puts on the URL.
func (h *oauthHandler) isAllowedLogoutReturnTo(candidate string) bool {
	parsed, err := url.Parse(candidate)
	if err != nil {
		return false
	}
	if !h.Config.IsDev() && parsed.Scheme != "https" {
		return false
	}
	candidateOrigin := parsed.Scheme + "://" + parsed.Host
	for _, allowed := range h.Config.GetAllowedAppOrigins() {
		if candidateOrigin == allowed {
			return true
		}
	}
	return false
}

func (h *oauthHandler) refreshSessionTokens(c echo.Context, tokens *storedOAuthTokens) (string, error) {
	if tokens.RefreshToken == "" {
		return "", fmt.Errorf("no refresh token in session")
	}

	ctx := c.Request().Context()

	lockKey := oauthRefreshLockKeyPrefix + h.OAuth.HashToken(tokens.RefreshToken)[:16]

	locked, err := h.Session.SetNX(lockKey, "1", oauthRefreshLockTTL)
	if err != nil {
		return "", fmt.Errorf("acquire refresh lock: %w", err)
	}

	if !locked {
		time.Sleep(oauthRefreshLockWait)
		fresh, err := h.readOAuthTokensSession(c)
		if err != nil {
			return "", fmt.Errorf("re-read after lock contention: %w", err)
		}
		if _, err := h.OAuth.ValidateAccessToken(ctx, fresh.AccessToken); err == nil {
			return fresh.AccessToken, nil
		}
		return "", fmt.Errorf("token still invalid after sibling refresh")
	}
	defer h.Session.Del(lockKey)

	storedRefresh, err := h.Repos.OAuthRefreshTokens.ReadByTokenHash(h.OAuth.HashToken(tokens.RefreshToken))
	if err != nil {
		return "", fmt.Errorf("refresh token not found: %w", err)
	}

	client, err := h.Repos.OAuthClients.ReadByClientID(storedRefresh.ClientID)
	if err != nil {
		return "", fmt.Errorf("client lookup: %w", err)
	}

	newPair, err := h.OAuth.RefreshTokenPair(ctx, tokens.RefreshToken, client, "")
	if err != nil {
		return "", fmt.Errorf("refresh token pair: %w", err)
	}

	if err := h.writeOAuthTokensSession(c, newPair); err != nil {
		return "", fmt.Errorf("write new session: %w", err)
	}

	return newPair.AccessToken, nil
}

func (h *oauthHandler) writeOAuthTokensSession(c echo.Context, pair *entities.OAuthTokenPair) error {
	payload := storedOAuthTokens{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		Scope:        pair.Scope,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if err := h.Session.Edit(c, sessionKeyOAuthTokens, string(raw)); err == nil {
		return nil
	}

	_, err = h.Session.AddWithTTL(c, sessionKeyOAuthTokens, string(raw), sessions.OAuthSessionTimeout)
	return err
}

func (h *oauthHandler) readOAuthTokensSession(c echo.Context) (*storedOAuthTokens, error) {
	raw, err := h.Session.Read(c, sessionKeyOAuthTokens)
	if err != nil {
		return nil, err
	}
	tokens := &storedOAuthTokens{}
	if err := json.Unmarshal([]byte(raw), tokens); err != nil {
		return nil, err
	}
	return tokens, nil
}
