package handlers

import (
	"errors"
	"log"
	"net/http"

	"auth/data/entities"
	"auth/requests"
	"auth/responses"

	libs_entities "anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

// OAuth grant types supported by AddToken.
const (
	OAuthGrantTypeAuthorizationCode = "authorization_code"
	OAuthGrantTypeRefreshToken      = "refresh_token"
	OAuthGrantTypeClientCredentials = "client_credentials"
)

// AddToken handles POST /oauth/token
func (h *oauthHandler) AddToken(c echo.Context) error {
	var (
		req = requests.NewOAuthTokenRequest().Token
	)

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

	switch req.GrantType {
	case OAuthGrantTypeAuthorizationCode:
		return h.tokenAuthorizationCode(c, req, client)
	case OAuthGrantTypeRefreshToken:
		return h.tokenRefreshToken(c, req, client)
	case OAuthGrantTypeClientCredentials:
		return h.tokenClientCredentials(c, req, client)
	default:
		return c.JSON(http.StatusBadRequest, &responses.OAuthErrorResponse{
			Error:       "unsupported_grant_type",
			Description: "grant type not supported",
		})
	}
}

func (h *oauthHandler) tokenAuthorizationCode(
	c echo.Context,
	req *requests.OAuthTokenReq,
	client *entities.OAuthClient,
) error {
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

	h.recordAuthLogAfterIssuance(c, viewer, client.ClientID, tokenPair.RefreshToken)

	oauthResp := &responses.OAuthTokenResponse{}
	return c.JSON(http.StatusOK, oauthResp.MapTokenPair(tokenPair))
}

func (h *oauthHandler) tokenRefreshToken(
	c echo.Context,
	req *requests.OAuthTokenReq,
	client *entities.OAuthClient,
) error {
	tokenPair, err := h.OAuth.RefreshTokenPair(
		c.Request().Context(),
		req.RefreshToken,
		client,
		req.Scope,
	)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusBadRequest, &responses.OAuthErrorResponse{
			Error:       "invalid_grant",
			Description: err.Error(),
			ErrorCode:   refreshErrorCode(err),
		})
	}

	oauthResp := &responses.OAuthTokenResponse{}
	return c.JSON(http.StatusOK, oauthResp.MapTokenPair(tokenPair))
}

// refreshErrorCode maps a refresh failure to a machine-readable code so
// clients don't have to string-match the description. An unrecognised
// error returns "", which clients treat as terminal.
func refreshErrorCode(err error) string {
	switch {
	case errors.Is(err, entities.ErrRefreshTokenExpired):
		return responses.OAuthErrorCodeRefreshExpired
	case errors.Is(err, entities.ErrRefreshTokenNotFound):
		return responses.OAuthErrorCodeRefreshNotFound
	case errors.Is(err, entities.ErrRefreshTokenReuse):
		return responses.OAuthErrorCodeRefreshReuse
	case errors.Is(err, entities.ErrRefreshTokenRaced):
		return responses.OAuthErrorCodeRefreshRaced
	case errors.Is(err, entities.ErrRefreshTokenRevoked):
		return responses.OAuthErrorCodeRefreshRevoked
	default:
		return ""
	}
}

func (h *oauthHandler) tokenClientCredentials(
	c echo.Context,
	req *requests.OAuthTokenReq,
	client *entities.OAuthClient,
) error {
	if !client.AllowsGrantType(OAuthGrantTypeClientCredentials) {
		return c.JSON(http.StatusForbidden, &responses.OAuthErrorResponse{
			Error:       "unauthorized_client",
			Description: "client_credentials grant not allowed for this client",
		})
	}

	if err := h.OAuth.ValidateScopesForClient(req.Scope, client); err != nil {
		return c.JSON(http.StatusBadRequest, &responses.OAuthErrorResponse{
			Error:       "invalid_scope",
			Description: err.Error(),
		})
	}

	tokenPair, err := h.OAuth.IssueClientAccessToken(
		c.Request().Context(),
		client,
		req.Scope,
	)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	oauthResp := &responses.OAuthTokenResponse{}
	return c.JSON(http.StatusOK, oauthResp.MapAccessToken(tokenPair))
}

// recordAuthLogAfterIssuance creates an auth_logs row for a freshly
// minted refresh token, conditionally sends a login-notification
// email, and trims redundant sessions. Errors are logged and
// swallowed.
//
// clientID is the OAuth client (e.g. "anahita-web") that requested
// the tokens — denormalized onto the auth log row so the UI can show
// "Signed in via Anahita Web App" without a join.
//
// This function is ONLY called from the authorization_code branch.
// The refresh_token branch goes through RefreshTokenPair, which
// updates the existing auth log's OAuthRefreshTokenID pointer rather
// than creating a new row — and doesn't touch this code path.
//
// THE EMAIL IS FOR A SIGN-IN, NOT FOR A CODE:
// The auth_log row is created unconditionally — every session needs
// a record. The email is sent only when the person has just signed in
// with a password, a code or a passkey (consumeFreshSignIn,
// oauth_fresh_signin.go).
// A browser whose IdP session is still alive is given a new code at
// /oauth/authorize without being asked for anything, and lands here
// with no sign-in having happened; an email then would announce one
// that did not. A reused auth log row stays quiet too: it is the same
// browser carrying on.
//
// The email send runs in a goroutine so a slow/failing mail server
// can't delay the token response. Errors are logged but never
// surface to the OAuth client — auth log + email are side effects of
// the sign-in, not preconditions for it.
func (h *oauthHandler) recordAuthLogAfterIssuance(
	c echo.Context,
	viewer *libs_entities.Viewer,
	clientID string,
	refreshPlaintext string,
) {
	// Consumed first, whatever follows, so that a mark never outlives
	// the exchange it was set for.
	freshSignIn := h.consumeFreshSignIn(viewer.ID)

	refreshHash := h.OAuth.HashToken(refreshPlaintext)
	rt, err := h.Repos.OAuthRefreshTokens.ReadByTokenHash(refreshHash)
	if err != nil {
		log.Println("auth log: refresh token lookup after issuance failed:", err)
		return
	}

	authLogHandler := newAuthLogHandler(h.Env)

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		log.Println("auth log: user lookup after issuance failed:", err)
		return
	}

	authLog, reused, err := authLogHandler.AddOrReuseSystem(c, rt.ID, clientID, user)
	if err != nil {
		log.Println("auth log: add after token issuance failed:", err)
		return
	}
	if reused {
		log.Println("auth log: reused auth log", authLog.ID, "for viewer", viewer.ID, "client", clientID)
	} else {
		log.Println("auth log: recorded new auth log", authLog.ID, "for viewer", viewer.ID, "client", clientID)
	}
	sendEmail, suppressReason := loginEmailDue(freshSignIn, reused)

	if err := authLogHandler.DeleteRedundantSessions(c, user); err != nil {
		log.Println("auth log: cleanup of redundant sessions failed:", err)
	}

	if !sendEmail {
		log.Printf("auth log: suppressing login notification for viewer %d client %s — %s",
			viewer.ID, clientID, suppressReason)
		return
	}

	// Login-notification email. Async because the SMTP/notification-
	// service round trip can take a few hundred ms in the worst case,
	// and the user shouldn't wait on it to see their app load.
	//
	// A captured copy of the person and auth log avoids any race with
	// later mutations of viewer state (the goroutine reads its own
	// values, not whatever the request handler has by the time the
	// email actually sends).
	notifyUser := *user
	notifyLog := *authLog
	go func() {
		if err := h.Notif.Auth.EmailLogin(&notifyUser, &notifyLog); err != nil {
			log.Println("auth log: login notification email failed:", err)
		}
	}()
}

// loginEmailDue is whether a code exchange sends the "New sign-in" email,
// and why not when it does not. Two reasons to stay quiet, told apart in
// the log so that whoever is looking for a missing notification looks at
// the right one: no password, code or passkey was given (a browser whose IdP
// session is alive was handed a new code), or the auth log row was reused
// (the same browser carrying on, even after a sign-in).
func loginEmailDue(freshSignIn, reused bool) (bool, string) {
	if reused {
		return false, "silent re-auth reused an existing session row"
	}
	if !freshSignIn {
		return false, "no sign-in with a password, a code or a passkey just now: a silent re-authorization"
	}
	return true, ""
}
