package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"auth/data/services"
	"auth/requests"
	"auth/responses"

	"github.com/labstack/echo/v4"
)

// ReadConsent handles GET /oauth/consent
//
// Renders the consent screen for the OAuth request stashed in session
// by /oauth/authorize. Generates a CSRF token used to verify that the
// subsequent POST came from the rendered form.
func (h *oauthHandler) ReadConsent(c echo.Context) error {
	viewer, err := h.readIdPSessionViewer(c)
	if err != nil || viewer == nil || !viewer.Authenticated {
		return c.Redirect(http.StatusFound, "/oauth/login")
	}

	authReq, err := h.loadStoredAuthRequest(c)
	if err != nil {
		return h.renderError(c, "Invalid Session", "No pending authorization request found.")
	}

	client, err := h.Repos.OAuthClients.ReadByClientID(authReq.ClientID)
	if err != nil {
		return h.renderError(c, "Invalid Client", "The application requesting access was not found.")
	}

	csrfToken, err := generateCSRFToken()
	if err != nil {
		log.Println("consent: csrf generation failed:", err)
		return h.renderError(c, "Server Error", "Failed to prepare consent page.")
	}
	if _, err := h.Session.Add(c, sessionKeyOAuthCSRFToken, csrfToken); err != nil {
		log.Println("consent: csrf session write failed:", err)
		return h.renderError(c, "Server Error", "Failed to prepare consent page.")
	}

	// Use the requested scopes for display, narrowed to what the user
	// can actually grant.
	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		log.Println("consent: failed to read user:", err)
		return h.renderError(c, "Server Error", "Failed to prepare consent page.")
	}
	grantable := h.OAuth.AvailableScopesForConsent(client, user)
	displayedScopes := filterScopes(authReq.Scope, grantable)

	return h.renderTemplate(c, "consent.html", map[string]any{
		"ClientName": client.Name,
		"ViewerName": strings.TrimSpace(user.Name),
		// Descriptions, not raw identifiers — "View your notifications"
		// rather than "notifications:read". This is the only place a
		// scope string would otherwise be end-user-visible text.
		"Scopes":      h.OAuth.ScopeDescriptions(displayedScopes),
		"RedirectURI": authReq.RedirectURI,
		"CSRFToken":   csrfToken,
	})
}

// AddConsent handles POST /oauth/consent
//
// Processes the user's allow/deny decision. On approval, issues an
// authorization code and either redirects to the client (web) or
// renders the native handoff page (custom-scheme redirect URI). On
// denial, redirects with the standard access_denied error.
//
// Requires a valid CSRF token matching the one set when the consent
// page was rendered.
func (h *oauthHandler) AddConsent(c echo.Context) error {
	viewer, err := h.readIdPSessionViewer(c)
	if err != nil || viewer == nil || !viewer.Authenticated {
		return c.Redirect(http.StatusFound, "/oauth/login")
	}

	req := requests.NewOAuthAuthorizeRequest().Callback
	if err := c.Bind(req); err != nil {
		return h.renderError(c, "Invalid Request", "The consent form could not be read.")
	}
	if err := c.Validate(req); err != nil {
		return h.renderError(c, "Invalid Request", "The consent form could not be read.")
	}

	// CSRF check.
	storedCSRF, err := h.Session.Read(c, sessionKeyOAuthCSRFToken)
	if err != nil || storedCSRF == "" || storedCSRF != req.CSRFToken {
		return h.renderError(c, "Page Expired", "This page expired. Go back to the application and try again.")
	}

	authReq, err := h.loadStoredAuthRequest(c)
	if err != nil {
		return h.renderError(c, "Invalid Session", "No pending authorization request found.")
	}

	// One-shot session state — clear regardless of outcome.
	_ = h.Session.Delete(c, sessionKeyOAuthAuthRequest)
	_ = h.Session.Delete(c, sessionKeyOAuthCSRFToken)

	client, err := h.Repos.OAuthClients.ReadByClientID(authReq.ClientID)
	if err != nil {
		return h.renderError(c, "Invalid Client", "The application is no longer registered.")
	}

	// User denied. Error redirect is sent regardless of redirect URI
	// scheme — the client (web or native) gets the access_denied
	// signal and can show its own "sign-in canceled" UI.
	if !req.Approved {
		oauthResp := &responses.OAuthAuthorizeResponse{}
		return c.Redirect(http.StatusFound,
			oauthResp.BuildErrorRedirect(authReq.RedirectURI, authReq.State,
				"access_denied", "user denied the request"))
	}

	// Narrow requested scopes to what the user can grant.
	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		log.Println("consent: failed to read user:", err)
		oauthResp := &responses.OAuthAuthorizeResponse{}
		return c.Redirect(http.StatusFound,
			oauthResp.BuildErrorRedirect(authReq.RedirectURI, authReq.State,
				"server_error", "failed to read user"))
	}
	grantable := h.OAuth.AvailableScopesForConsent(client, user)
	grantedScopes := intersectScopes(authReq.Scope, grantable)

	code, err := h.OAuth.IssueAuthorizationCode(viewer, services.AuthorizationGrant{
		Client:              client,
		RedirectURI:         authReq.RedirectURI,
		Scopes:              grantedScopes,
		CodeChallenge:       authReq.CodeChallenge,
		CodeChallengeMethod: authReq.CodeChallengeMethod,
		Nonce:               authReq.Nonce,
	})
	if err != nil {
		log.Println("consent: issue code failed:", err)
		oauthResp := &responses.OAuthAuthorizeResponse{}
		return c.Redirect(http.StatusFound,
			oauthResp.BuildErrorRedirect(authReq.RedirectURI, authReq.State,
				"server_error", "failed to issue authorization code"))
	}

	return h.respondWithAuthCode(c, authReq.RedirectURI, code, authReq.State, client.Name)
}

// loadStoredAuthRequest reads the stashed authorization request from
// session and unmarshals it.
func (h *oauthHandler) loadStoredAuthRequest(c echo.Context) (*requests.OAuthAuthorizeReq, error) {
	raw, err := h.Session.Read(c, sessionKeyOAuthAuthRequest)
	if err != nil {
		return nil, err
	}
	authReq := &requests.OAuthAuthorizeReq{}
	if err := json.Unmarshal([]byte(raw), authReq); err != nil {
		return nil, err
	}
	return authReq, nil
}

// generateCSRFToken returns a 32-byte URL-safe random token.
func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// filterScopes returns the subset of `requested` scopes (space-
// separated) that appear in `allowed`. Returns a slice for template
// iteration.
func filterScopes(requested string, allowed []string) []string {
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
	return kept
}
