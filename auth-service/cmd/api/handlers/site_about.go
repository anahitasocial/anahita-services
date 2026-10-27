package handlers

import (
	"net/http"

	"auth/about"
	"auth/constants"

	"github.com/labstack/echo/v4"
)

// What the installation says about itself to the people who use it
// (roadmap row 102): its rules, its promises, and the rules a new group is
// offered to start from. The operator's, in auth-defaults/site.json
// (package about), read again when it changes.
var siteSays = about.FromEnv()

// SiteAbout is what GET /site answers: every text in every language it is
// written in, keyed by the web app's language names ("en-CA", "fr-FR"), for
// the reader's app to choose from.
type SiteAbout struct {
	// The site's rules, and the version agreed to: everybody ticks each
	// before using the site, and again when the version is raised.
	Rules about.Rules `json:"rules"`
	// Shown on the About page, under "Our promises to you".
	Promises []about.Item `json:"promises"`
	// Offered to a group's administrators when they write its rules.
	RuleSuggestions []about.Item `json:"rule_suggestions"`
}

// Site answers what the installation says about itself. Public: it is read
// before signing up.
func (h *nodeInfoHandler) Site(c echo.Context) error {
	said := siteSays.Site().Filled(constants.AccountGraceDays())
	c.Response().Header().Set("Cache-Control", "public, max-age=300")
	return c.JSON(http.StatusOK, SiteAbout{
		Rules:           said.Rules,
		Promises:        said.Promises,
		RuleSuggestions: said.RuleSuggestions,
	})
}
