package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"auth/data/services"
	"auth/requests"
	"auth/responses"

	"github.com/labstack/echo/v4"
)

// Session keys for the OAuth flow's transient state.
//
// sessionKeyOAuthReturnTo is defined in oauth_return_to.go alongside
// the validation helpers — every read, write, and consume of that key
// flows through StoreReturnTo / consumeReturnTo / returnToParam.
const (
	sessionKeyOAuthAuthRequest = "oauth_auth_request"
	sessionKeyOAuthCSRFToken   = "oauth_csrf_token"
)

// ReadAuthorize handles GET /oauth/authorize
//
// The OAuth authorization endpoint. Validates the inbound request,
// determines whether the user is logged into the IdP, and either:
//   - Redirects to /oauth/login if no IdP session exists
//   - Auto-issues an authorization code for first-party SkipConsent clients
//   - Stores the request in session and redirects to /oauth/consent for
//     all other cases
//
// Public clients MUST present a code_challenge (PKCE). Confidential
// clients may omit it.
func (h *oauthHandler) ReadAuthorize(c echo.Context) error {
	req := requests.NewOAuthAuthorizeRequest().Authorize

	if err := c.Bind(req); err != nil {
		return h.renderError(c, "Invalid Request", "The authorization request was malformed.")
	}
	if err := c.Validate(req); err != nil {
		return h.renderError(c, "Invalid Request", err.Error())
	}

	client, err := h.Repos.OAuthClients.ReadByClientID(req.ClientID)
	if err != nil {
		return h.renderError(c, "Invalid Client", "The application requesting access was not found.")
	}
	if !client.Active {
		return h.renderError(c, "Inactive Client", "This application is currently disabled.")
	}

	if !client.AllowsRedirectURI(req.RedirectURI) {
		return h.renderError(c, "Invalid Redirect", "The provided redirect URI is not registered for this application.")
	}

	// PKCE enforcement for public clients.
	if !client.Confidential && req.CodeChallenge == "" {
		return h.authorizeError(c, req.RedirectURI, req.State,
			"invalid_request", "code_challenge is required for public clients")
	}

	if err := h.OAuth.ValidateScopesForClient(req.Scope, client); err != nil {
		return h.authorizeError(c, req.RedirectURI, req.State, "invalid_scope", err.Error())
	}

	// Check IdP session. Not logged in → save the request and bounce
	// to /oauth/login.
	//
	// StoreReturnTo validates the candidate (must be a relative URL
	// under /oauth/...) before persisting. c.Request().URL.String()
	// on a server-side request yields a relative URL by construction,
	// so this should always succeed; a rejection here would indicate
	// something upstream is rewriting the URL.
	viewer, err := h.readIdPSessionViewer(c)
	if err != nil || viewer == nil || !viewer.Authenticated {
		if !h.StoreReturnTo(c, c.Request().URL.String()) {
			log.Printf("authorize: refused to store return_to=%q", c.Request().URL.String())
		}
		return c.Redirect(http.StatusFound, "/oauth/login")
	}

	// First-party clients with SkipConsent bypass the consent screen.
	// The granted scopes are intersected with what the viewer is
	// actually allowed to grant — a non-admin user can never silently
	// be granted admin scopes just because the client is registeredf
	// for them.
	if client.FirstParty && client.SkipConsent {
		user, err := h.Repos.Users.ReadByID(viewer.ID)
		if err != nil {
			log.Println("authorize: failed to read user:", err)
			return h.authorizeError(c, req.RedirectURI, req.State,
				"server_error", "failed to read user")
		}
		grantable := h.OAuth.AvailableScopesForConsent(client, user)
		grantedScopes := intersectScopes(req.Scope, grantable)

		code, err := h.OAuth.IssueAuthorizationCode(viewer, services.AuthorizationGrant{
			Client:              client,
			RedirectURI:         req.RedirectURI,
			Scopes:              grantedScopes,
			CodeChallenge:       req.CodeChallenge,
			CodeChallengeMethod: req.CodeChallengeMethod,
			Nonce:               req.Nonce,
		})

		if err != nil {
			log.Println("authorize: issue code failed:", err)
			return h.authorizeError(c, req.RedirectURI, req.State,
				"server_error", "failed to issue authorization code")
		}

		return h.respondWithAuthCode(c, req.RedirectURI, code, req.State, client.Name)
	}

	// Otherwise: stash the request and render consent.
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return h.renderError(c, "Server Error", "Failed to prepare authorization request.")
	}
	if _, err := h.Session.Add(c, sessionKeyOAuthAuthRequest, string(reqJSON)); err != nil {
		log.Println("authorize: failed to store auth request:", err)
		return h.renderError(c, "Server Error", "Failed to prepare authorization request.")
	}

	return c.Redirect(http.StatusFound, "/oauth/consent")
}

// authorizeError sends the OAuth-protocol error redirect back to the
// client per RFC 6749 §4.1.2.1. Used when a request makes it past the
// initial validation gates (so we have a trustworthy redirect URI) but
// fails for a reason the client should know about.
func (h *oauthHandler) authorizeError(c echo.Context, redirectURI, state, code, description string) error {
	oauthResp := &responses.OAuthAuthorizeResponse{}
	return c.Redirect(http.StatusFound,
		oauthResp.BuildErrorRedirect(redirectURI, state, code, description))
}

// intersectScopes returns the space-separated subset of `requested`
// scopes that also appears in `allowed`. Order follows `requested`.
func intersectScopes(requested string, allowed []string) string {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, s := range allowed {
		allowedSet[s] = struct{}{}
	}

	var kept []string
	for _, s := range strings.Fields(requested) {
		if _, ok := allowedSet[s]; ok {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, " ")
}
