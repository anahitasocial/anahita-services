package handlers

import (
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
)

// sessionKeyOAuthReturnTo holds the in-progress OAuth /authorize URL
// that the person should be sent back to after authenticating.
// Owned by this file because every read, write, and consume of this
// key flows through one of the helpers below.
const sessionKeyOAuthReturnTo = "oauth_return_to"

// StoreReturnTo validates a return_to candidate and writes it to
// the IdP session. Call this from /oauth/authorize (and any other
// site that needs to set the post-login destination) instead of
// writing the session key directly — that centralizes validation at
// the only write site.
//
// Returns true when stored, false when the candidate was rejected
// or persistence failed. Rejected candidates are silently dropped:
// the post-login flow falls through to Config.GetFrontendURL().
func (h *oauthHandler) StoreReturnTo(c echo.Context, candidate string) bool {
	if !isSafeReturnTo(candidate) {
		if candidate != "" {
			log.Printf("oauth: rejected unsafe return_to=%q", candidate)
		}
		return false
	}
	if _, err := h.Session.Add(c, sessionKeyOAuthReturnTo, candidate); err != nil {
		log.Println("oauth: return_to write failed:", err)
		return false
	}
	return true
}

// redirectToReturnTo sends the person to the stored return_to URL
// after validating it, or to the frontend if no valid return_to
// exists. The previous implementation fell back to "/" which on
// this service surfaces the nginx root rather than an app the
// person can actually use.
//
// The reason string is logged so the "ended up at the wrong place
// after login" symptom can be diagnosed from logs alone.
func (h *oauthHandler) redirectToReturnTo(c echo.Context, reason string) error {
	target, ok := h.consumeReturnTo(c)
	if !ok {
		fallback := h.Config.GetFrontendURL()
		log.Printf("oauth: redirect to fallback=%s (reason=%s)", fallback, reason)
		return c.Redirect(http.StatusFound, fallback)
	}
	log.Printf("oauth: redirect to return_to=%s (reason=%s)", target, reason)
	return c.Redirect(http.StatusFound, target)
}

// consumeReturnTo reads, validates, and clears the stored return_to.
// Returns (value, true) on success; ("", false) when missing,
// unreadable, or invalid. Invalid stored values are cleared so the
// bad state doesn't persist across requests.
func (h *oauthHandler) consumeReturnTo(c echo.Context) (string, bool) {
	raw, err := h.Session.Read(c, sessionKeyOAuthReturnTo)
	if err != nil || raw == "" {
		return "", false
	}
	// Always clear — a malformed value should not leak into
	// subsequent attempts.
	_ = h.Session.Delete(c, sessionKeyOAuthReturnTo)

	if !isSafeReturnTo(raw) {
		log.Printf("oauth: stored return_to failed validation, dropping: %q", raw)
		return "", false
	}
	return raw, true
}

// isSafeReturnTo enforces a strict allowlist: relative URLs rooted
// at this host, under a known IdP path namespace. This blocks
// open-redirect attacks via:
//   - schemes  ("https://evil.com")
//   - authority-less URLs ("//evil.com")
//   - backslash variants that some browsers normalize
//   - CR/LF and other control characters (header injection)
//   - paths outside the IdP's own routes
//
// OAuth client redirect URIs — including Electron custom schemes
// like anahita-desktop-app://callback — are NOT validated here. Those are
// handled by /oauth/authorize against the registered client list.
// The return_to we store is always the IdP's own /oauth/authorize
// path with the original query string.
func isSafeReturnTo(returnTo string) bool {
	if returnTo == "" {
		return false
	}

	// Single leading slash, no protocol-relative URLs.
	if !strings.HasPrefix(returnTo, "/") || strings.HasPrefix(returnTo, "//") {
		return false
	}

	// Reject backslash (Windows path that some browsers normalize
	// to forward slashes, defeating the "//" check above) and any
	// control characters that could fragment the redirect header.
	for i := 0; i < len(returnTo); i++ {
		ch := returnTo[i]
		if ch == '\\' || ch < 0x20 || ch == 0x7f {
			return false
		}
	}

	u, err := url.Parse(returnTo)
	if err != nil {
		return false
	}
	// Belt-and-braces: even with a leading slash, an URL with
	// userinfo or a host slipped through shouldn't pass.
	if u.Scheme != "" || u.Host != "" || u.User != nil {
		return false
	}
	if !isKnownIdPPath(u.Path) {
		return false
	}
	return true
}

// isKnownIdPPath enforces that return_to points to an IdP route we
// recognize. Intentionally conservative: even with the scheme/host
// checks above, restricting paths prevents return_to from being used
// to bounce through arbitrary IdP endpoints (for example, ones that
// themselves issue further redirects).
func isKnownIdPPath(p string) bool {
	switch {
	case p == "/":
		return false // never useful as a post-login destination
	case strings.HasPrefix(p, "/oauth/"):
		return true
	default:
		return false
	}
}

// returnToParam returns a URL-encoded "?return_to=…" fragment that
// the frontend signup and reset-password pages can consume to bounce
// the person back into the OAuth flow once they're done.
//
// Empty when there's no in-progress OAuth flow or when the stored
// value fails validation.
func (h *oauthHandler) returnToParam(c echo.Context) string {
	raw, err := h.Session.Read(c, sessionKeyOAuthReturnTo)
	if err != nil || raw == "" {
		return ""
	}
	if !isSafeReturnTo(raw) {
		return ""
	}
	scheme := "https"
	if c.Request().TLS == nil {
		scheme = "http"
	}
	absolute := scheme + "://" + c.Request().Host + raw
	return "?return_to=" + url.QueryEscape(absolute)
}
