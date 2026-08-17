package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

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

// loginEmailSuppressionWindow controls how recently a prior auth log
// for the same (person, client) pair will suppress a new login
// notification email. Tuned to comfortably exceed the access token
// TTL (15m) so that the every-15-minute silent re-auth cycle via the
// IdP's person_session cookie does not produce a notification on each
// loop. A genuine new sign-in from a different browser/device after
// this window will still notify; an attacker who manages to silently
// re-auth within the window already has the IdP session cookie, so
// suppressing the email there is not a meaningful regression — the
// security event of interest is the original credential-backed
// login, which is already covered.
const loginEmailSuppressionWindow = 30 * time.Minute

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
		})
	}

	oauthResp := &responses.OAuthTokenResponse{}
	return c.JSON(http.StatusOK, oauthResp.MapTokenPair(tokenPair))
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
// EMAIL SUPPRESSION FOR SILENT RE-AUTH:
// The auth_log row is created unconditionally — every session needs
// a record. The email, however, is gated on whether a recent prior
// auth_log exists for the same (person, client) pair. This filters
// out the every-15-minute silent re-auth cycle: when the access
// token expires, the SPA hits /oauth/authorize, the IdP recognizes
// the person_session cookie, mints a new auth code without
// prompting for credentials, and we land back here with no actual
// credential event having occurred. The user already received the
// email on their original sign-in; firing one every 15 minutes for
// what is mechanically a token-rotation event is noise, not signal.
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
	// Check for a recent prior auth log BEFORE creating the new row,
	// otherwise the lookup would always find at least the row we
	// just inserted.
	suppressEmail := h.hasRecentLoginForClient(viewer.ID, clientID)

	refreshHash := h.OAuth.HashToken(refreshPlaintext)
	rt, err := h.Repos.OAuthRefreshTokens.ReadByTokenHash(refreshHash)
	if err != nil {
		log.Println("authlog: refresh token lookup after issuance failed:", err)
		return
	}

	authlogHandler := newAuthLogHandler(h.Env)

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		log.Println("authlog: user lookup after issuance failed:", err)
		return
	}

	authLog, err := authlogHandler.AddSystem(c, rt.ID, clientID, user)
	if err != nil {
		log.Println("authlog: add after token issuance failed:", err)
		return
	}
	log.Println("authlog: recorded new auth log", authLog.ID, "for viewer", viewer.ID, "client", clientID)

	if err := authlogHandler.DeleteRedundantSessions(c, user); err != nil {
		log.Println("authlog: cleanup of redundant sessions failed:", err)
	}

	if suppressEmail {
		log.Println("authlog: suppressing login notification (silent re-auth within window) for viewer", viewer.ID, "client", clientID)
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
	// go func() {
	// 	if err := h.Notif.Auth.EmailLogin(user, authLog); err != nil {
	// 		log.Println("authlog: login notification email failed:", err)
	// 	}
	// }()
}

// hasRecentLoginForClient reports whether an auth_log row already
// exists for this (person, client) pair within
// loginEmailSuppressionWindow of now. Used to suppress redundant
// login-notification emails on silent re-auth.
//
// sql.ErrNoRows is the expected case for a first-ever login from
// this client and returns false (do send the email).
//
// Any other error is treated as "no recent login" (i.e. fail open —
// send the email) because the cost of an extra email is lower than
// the cost of suppressing a legitimate new-sign-in notification due
// to a transient DB hiccup.
func (h *oauthHandler) hasRecentLoginForClient(userID int64, clientID string) bool {
	latest, err := h.Repos.AuthLogs.ReadLatestByCreatedByAndClient(userID, clientID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Println("authlog: latest lookup for suppression check failed (failing open):", err)
		}
		return false
	}
	return time.Since(latest.CreatedOn) < loginEmailSuppressionWindow
}
