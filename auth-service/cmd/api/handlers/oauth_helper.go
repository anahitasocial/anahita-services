package handlers

import (
	"html/template"
	"net/http"
	"net/url"

	"auth/responses"

	"github.com/labstack/echo/v4"
)

// respondWithAuthCode dispatches the success response after issuing
// an authorization code, choosing between a browser-style 302 and a
// native-app handoff page based on the client's redirect URI scheme.
//
// For web clients (http/https) the 302 takes the browser straight to
// the SPA, which then renders its own signed-in UI. For native clients
// (custom URI schemes like anahita-desktop://callback) a 302 hands the
// URL to the OS, which launches the app — but the browser tab stays
// on whatever rendered the redirect, with no visual confirmation
// that anything happened. Showing the handoff page closes that gap:
// the person sees "you're signed in, you can close this tab," the OS
// is triggered to launch the app, and there's a manual fallback link
// for cases where auto-launch is blocked.
//
// appName is shown to the person on the handoff page — pass the
// OAuth client's display name.
func (h *oauthHandler) respondWithAuthCode(
	c echo.Context,
	redirectURI, code, state, appName string,
) error {
	oauthResp := &responses.OAuthAuthorizeResponse{}
	target := oauthResp.BuildRedirect(redirectURI, code, state)

	if isNativeRedirectURI(redirectURI) {
		// CallbackURL is wrapped in template.URL to bypass Go's
		// html/template URL-context sanitization. The sanitizer has
		// a hardcoded allowlist of "safe" schemes (http, https,
		// mailto, etc.) and replaces anything else with the
		// "#ZgotmplZ" sentinel — including legitimate custom
		// schemes like anahita-desktop://. Without this wrap,
		// the auto-launch script in native_success.html ends up
		// clicking <a href="#ZgotmplZ"> instead of the real URL.
		//
		// template.URL is a typed string that tells html/template
		// "I've already vetted this." That promise is safe here
		// because:
		//   1. `redirectURI` was validated against the client's
		//      registered list earlier in the flow
		//      (AllowsRedirectURI in ReadAuthorize), so an
		//      attacker can't slip in a javascript: URL
		//   2. `target` is constructed by BuildRedirect from
		//      that vetted URI plus server-issued code/state,
		//      none of which are user-controlled
		// If client-side redirect URI registration ever becomes
		// user-editable (custom apps, etc.), the registration code
		// itself must reject non-allowlisted schemes before storing
		// — the trust boundary moves up to that point.
		return h.renderTemplate(c, "native_success.html", map[string]any{
			"AppName":     appName,
			"CallbackURL": template.URL(target),
		})
	}

	return c.Redirect(http.StatusFound, target)
}

// isNativeRedirectURI reports whether a redirect URI targets a native
// app via a custom URI scheme (e.g. myapp://callback) rather
// than a web browser context (http/https).
//
// Loopback addresses (http://127.0.0.1:PORT/...) intentionally count
// as web — the browser navigates to the local server and the OAuth
// client renders its own success page from there. Only custom schemes
// need the in-IdP handoff page.
//
// The redirect URI has already been validated against the client's
// registered list before this function is called, so we trust that
// the scheme reflects an intentional client configuration.
func isNativeRedirectURI(redirectURI string) bool {
	u, err := url.Parse(redirectURI)
	if err != nil || u.Scheme == "" {
		return false
	}
	return u.Scheme != "http" && u.Scheme != "https"
}
