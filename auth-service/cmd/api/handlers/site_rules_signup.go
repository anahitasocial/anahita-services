package handlers

import (
	"log"

	"anahita-libs/about"
	"strings"

	"auth/data/entities"

	"github.com/labstack/echo/v4"
)

// The site's rules where an account is made (roadmap row 102, part 2b; the
// user, 2026-10-19: the first super administrator reminded "on the server
// side when signup happens"): every page auth-service renders to make an
// account shows them as a plain checklist, and the account is made with the
// agreement, at the version shown, recorded as the Terms are. Without every
// rule ticked at the version the site has now, the server refuses.

// ruleLine is one rule as a page shows it, in the reader's language.
type ruleLine struct {
	Title string
	Body  string
}

// languageOf is the first language the browser asks for, "en-CA" when it
// asks for none.
func languageOf(c echo.Context) string {
	header := c.Request().Header.Get("Accept-Language")
	first := strings.TrimSpace(strings.SplitN(strings.SplitN(header, ",", 2)[0], ";", 2)[0])
	if first == "" {
		return "en-CA"
	}
	return first
}

// rulesToTick is the site's rules for a page, and their version: none when
// the site has none, or when SITE_RULES is off and `always` is not asked
// for. `always` is the first super administrator, reminded whatever the
// setting.
func (env *Env) rulesToTick(c echo.Context, always bool) ([]ruleLine, string) {
	if !always && !env.Config.SiteRulesOn() {
		return nil, ""
	}
	rules := siteSays.Site().Filled(0).Rules
	language := languageOf(c)
	lines := make([]ruleLine, 0, len(rules.Items))
	for _, item := range rules.Items {
		lines = append(lines, ruleLine{Title: item.Title.In(language), Body: item.Body.In(language)})
	}
	if len(lines) == 0 {
		return nil, ""
	}
	return lines, rules.Version
}

// rulesData is what a page's template is given for the checklist.
func (env *Env) rulesData(c echo.Context, always bool) map[string]any {
	lines, version := env.rulesToTick(c, always)
	return map[string]any{"Rules": lines, "RulesVersion": version}
}

// The pledge of whoever installs Anahita, its first super administrator,
// on the page where they set their first password: the creator's conditions
// on the software itself (about.PledgeItems), whatever the site's rules and
// SITE_RULES say. Shown in place of the site's rules, which come to them
// afterwards as to anybody, in the web app.

// pledgeVersion is what the form carries, told apart from a site's rules.
const pledgeFormVersion = "pledge-" + about.PledgeVersion

// pledgeData is what the set-password page is given for the pledge.
func pledgeData(c echo.Context) map[string]any {
	language := languageOf(c)
	lines := make([]ruleLine, 0, len(about.PledgeItems))
	for _, item := range about.PledgeItems {
		lines = append(lines, ruleLine{Title: item.In(language)})
	}
	return map[string]any{
		"Rules":        lines,
		"RulesVersion": pledgeFormVersion,
		"RulesTitle":   "Make a pledge",
		// The page is their sign-up, not a reset: worded so.
		"Installing": true,
		"RulesIntro": about.PledgeIntro.In(language),
		"RulesLead":  about.PledgeLead.In(language),
	}
}

// pledgeTicked says whether the form ticked every item of the pledge.
func pledgeTicked(c echo.Context) bool {
	if c.FormValue("rules_version") != pledgeFormVersion {
		return false
	}
	params, err := c.FormParams()
	if err != nil {
		return false
	}
	ticked := map[string]bool{}
	for _, value := range params["rules_ticked"] {
		ticked[value] = true
	}
	return len(ticked) == len(about.PledgeItems)
}

// rulesTicked says whether the form that was sent ticked every rule the site
// has now, at its version: true when there are none to tick.
func (env *Env) rulesTicked(c echo.Context, always bool) bool {
	lines, version := env.rulesToTick(c, always)
	if len(lines) == 0 {
		return true
	}
	if c.FormValue("rules_version") != version {
		return false
	}
	params, err := c.FormParams()
	if err != nil {
		return false
	}
	ticked := map[string]bool{}
	for _, value := range params["rules_ticked"] {
		ticked[value] = true
	}
	return len(ticked) == len(lines)
}

// recordRules records that an account agreed to the site's rules at
// `version`, with where from, as the Terms are recorded. A failure is
// logged: the account stands, and the web app asks again at sign-in.
func (env *Env) recordRules(c echo.Context, personID int64, version string) {
	if version == "" || personID <= 0 {
		return
	}
	consent := &entities.Consent{}
	if err := acceptInto(c, consent, version); err != nil {
		log.Printf("site rules: recording for %d: %v", personID, err)
		return
	}
	if err := env.Repos.Users.EditRulesConsent(personID, consent); err != nil {
		log.Printf("site rules: recording for %d: %v", personID, err)
	}
}

// siteRulesVersion is the version the site's rules are at now.
func siteRulesVersion() string {
	return siteSays.Site().Rules.Version
}
