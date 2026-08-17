package handlers

import (
	// "auth/cmd/api/notifiers"
	"auth/cmd/api/notifiers"
	"auth/cmd/api/permissions"
	"auth/config"
	"auth/data/repositories"
	"auth/data/services"
	"html/template"
	"net/http"

	"anahita-libs/crypt"
	"anahita-libs/event"
	"anahita-libs/sessions"

	"github.com/labstack/echo/v4"
)

type Env struct {
	Config *config.Config
	Crypt  *crypt.Encryption
	// SecretCrypt encrypts secrets at rest (TOTP secrets). Backed by
	// either a local AES key or AWS KMS depending on CRYPT_BACKEND —
	// handlers should not care which, so they go through this
	// interface rather than constructing a backend themselves.
	SecretCrypt crypt.Crypt
	Notif       *notifiers.Notifiers
	OAuth       *services.OAuthService
	Perms       *permissions.Permissions
	Repos       *repositories.Repositories
	Session     sessions.SessionInterface
	Srv         *services.Services
	Publisher   *event.Publisher
}

// renderTemplate parses base.html, the shared partials, and the named
// template, then writes the result to the response. Promoted onto every
// handler that embeds *Env so both the OAuth and password (and future)
// flows share one rendering path.
//
// logo.html has to be in the set: every page template invokes
// {{template "logo" .}}, and a partial that isn't parsed alongside the
// page fails at execute time rather than parse time — so leaving it out
// turns every server-rendered auth page into a 500.
//
// Templates live under web/templates/ relative to the working
// directory of the binary. Each call re-reads from disk — fine for
// our traffic volume and convenient in dev (template edits take
// effect without a restart). If this ever becomes a hotspot, switch
// to a single template.ParseGlob at startup with a sync.Map of
// pre-parsed sets keyed by page name.
//
// Status defaults to 200. Server-rendered form errors don't need
// non-2xx statuses — the error is communicated via the page content,
// and 4xx/5xx on HTML responses confuses browser UA heuristics
// (back-button caching, prerender hints, etc).
func (env *Env) renderTemplate(c echo.Context, name string, data map[string]any) error {
	tmpl, err := template.ParseFiles(
		"web/templates/base.html",
		"web/templates/logo.html",
		"web/templates/"+name,
	)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return tmpl.ExecuteTemplate(c.Response().Writer, "base", data)
}

// renderError renders error.html with a title and message. Used for
// terminal flow failures (invalid client, expired session, invalid
// token, etc) where redirecting back to the caller isn't possible or
// appropriate.
func (env *Env) renderError(c echo.Context, title, message string) error {
	return env.renderTemplate(c, "error.html", map[string]any{
		"ErrorTitle":   title,
		"ErrorMessage": message,
		"FrontendURL":  env.Config.GetFrontendURL(),
	})
}
