package main

import (
	"net/http"

	"auth/cmd/api/handlers"
	"auth/responses"

	"anahita-libs/apidoc"
	libs_responses "anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	authorize echo.MiddlewareFunc
	setViewer echo.MiddlewareFunc
}

func NewRoutes(
	handler *handlers.Handlers,
	authorize echo.MiddlewareFunc,
	setViewer echo.MiddlewareFunc,
) *Routes {
	return &Routes{
		handler:   handler,
		authorize: authorize,
		setViewer: setViewer,
	}
}

// What several areas of routes share, below. Each area registers its routes
// in a method of its own, in the routes_*.go files beside this one.

// stepUp is the refusal of a change that needs the viewer to have proved who
// they are again just before it (StepUp).
const stepUp = "`step_up_required`: the viewer must prove who they are again first (`/reauth`)"

// refusal is the shape of a refusal that says why in a code of its own.
var refusal = libs_responses.Refusal{}

// oauthError is the shape OAuth 2.0 refuses in (RFC 6749, 5.2).
var oauthError = responses.OAuthErrorResponse{}

// page stands for an HTML page in what a route sends: what is described is a
// page, with no shape of its own.
const page = ""

// form describes a page of the sign-in, sign-up and recovery flows, which a
// browser is sent to. Every one answers 200 with the page, a refusal among
// them: an error is said on the page.
func form(summary, describe string) func(*apidoc.Route) {
	return func(route *apidoc.Route) {
		route.Summary(summary).
			Describe(describe).
			Viewer(apidoc.Anyone).
			Media("text/html").
			Sends(http.StatusOK, page, "the page")
	}
}

// submit describes what such a page posts. 200 is the page again, saying
// what was wrong; 303 is on to `next`.
func submit(summary, describe string, takes any, next string) func(*apidoc.Route) {
	return func(route *apidoc.Route) {
		route.Summary(summary).
			Describe(describe).
			Viewer(apidoc.Anyone).
			Media("text/html").
			Takes(takes).
			Unbound().
			Sends(http.StatusOK, page,
				"the page again, saying what was wrong, or a page that says what happened").
			SendsNothing(http.StatusSeeOther, next)
	}
}

// Init registers the routes, each with its description, and answers with
// the descriptions: docs/api/auth-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("auth-service", "Accounts and signing in",
		"Signing up, signing in and out, and the OAuth 2.0 and OpenID Connect server every app "+
			"signs in through (oauth.md); what proves who somebody is (passwords, codes, passkeys, "+
			"two-factor authentication) and changing it; invitations and the approval queue; and the "+
			"installation's description of itself (NodeInfo).")
	api := doc.On(e)

	// The pages' files, and whether an address or a name is taken.
	r.assetAndCheckRoutes(api)

	// The pages for a forgotten password.
	r.passwordResetRoutes(api)

	// The pages an invitation's link opens.
	r.inviteAcceptRoutes(api)

	// The signup pages, and confirming the address.
	r.signupRoutes(api)

	// The terms, the rules and the walk-through, for the signed-in viewer.
	r.agreementRoutes(api)

	// What the installation says about itself: OpenID Connect, NodeInfo, the site.
	r.discoveryRoutes(api)

	// Signing in and out, the pages and an app's session.
	r.signInRoutes(api)

	// The OAuth 2.0 endpoints apps call, and userinfo.
	r.oauthProtocolRoutes(api)

	// The apps and the signing keys, for a super administrator.
	r.oauthAdminRoutes(api)

	// Changing the password, the email address and the username.
	r.accountChangeRoutes(api)

	// The sessions somebody is signed in with.
	r.authLogRoutes(api)

	// The invitations the viewer sends.
	r.inviteAdminRoutes(api)

	// The approval queue and the accounts, for administrators.
	r.adminRoutes(api)

	// Two-factor authentication.
	r.totpRoutes(api)

	// Passkeys.
	r.webAuthnRoutes(api)

	return doc
}
