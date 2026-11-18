package main

import (
	"net/http"

	"auth/cmd/api/handlers"

	"anahita-libs/apidoc"
)

// discoveryRoutes registers what this installation says about itself: to apps, to
// other servers, and to people about to sign up.
func (r *Routes) discoveryRoutes(api *apidoc.Router) {
	h := r.handler

	// --- OpenID Connect discovery ---
	// Where an app finds each endpoint, and the keys a token's signature is
	// checked with. Public: no middleware, so the caller is never known.
	api.GET("/.well-known/openid-configuration", h.OAuth.Discovery).
		Summary("The OpenID Connect discovery document").
		Describe("Where each endpoint is, and what is supported (OpenID Connect Discovery 1.0).").
		Viewer(apidoc.Anyone).
		Sends(http.StatusOK, handlers.OpenIDConfiguration{})

	api.GET("/.well-known/jwks.json", h.OAuth.JWKS).
		Summary("The keys tokens are signed with").
		Describe("The public halves, as a JSON Web Key Set (RFC 7517): what every service checks "+
			"a token's signature with.").
		Viewer(apidoc.Anyone).
		Sends(http.StatusOK, handlers.JWKSet{})

	// --- NodeInfo ---
	// How this installation describes itself to the fediverse and to its own web
	// app. Public: no middleware, so the caller is never known. That is the point:
	// a server which has never spoken to this one can read it.
	//
	// Two hops because that is the protocol: the well-known document is a list of
	// links, and a consumer follows the schema version it knows.
	api.GET("/.well-known/nodeinfo", h.NodeInfo.Discovery).
		Summary("Where the NodeInfo document is").
		Viewer(apidoc.Anyone).
		Sends(http.StatusOK, handlers.NodeInfoLinks{})

	api.GET("/nodeinfo/2.1", h.NodeInfo.Read).
		Summary("What this installation is (NodeInfo 2.1)").
		Describe("The software, the protocols it speaks, whether it takes registrations, and how "+
			"many people use it. Its names are NodeInfo's, in camelCase.").
		Viewer(apidoc.Anyone).
		Sends(http.StatusOK, handlers.NodeInfoDocument{})

	// --- The site ---
	// What the installation says about itself to the people who use it: its rules,
	// its promises, and rules a new group may start from (roadmap row 102).
	// Public: no middleware, so the caller is never known. It is read before
	// signing up.
	api.GET("/site", h.NodeInfo.Site).
		Summary("What this site says about itself").
		Describe("Its rules, which everybody agrees to by ticking each one, with the version "+
			"agreed to; its promises, for its About page; and the rules a new group is offered to "+
			"start from. Every text in every language it is written in, by the web app's language "+
			"names. The operator's, in `site.json` (configuration.md, `SITE_DEFAULTS_PATH`).").
		Viewer(apidoc.Anyone).
		Sends(http.StatusOK, handlers.SiteAbout{})
}
