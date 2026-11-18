package main

import (
	"net/http"

	"auth/cmd/api/handlers"
	"auth/requests"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
	libs_responses "anahita-libs/responses"
)

// agreementRoutes registers what the signed-in viewer has agreed to, and whether
// they have been through the walk-through after signing up.
func (r *Routes) agreementRoutes(api *apidoc.Router) {
	h := r.handler

	// --- Agreements ---
	// Accepting the Terms of Service, the Privacy Policy and the site's rules, one
	// document per request. Every route needs a token (r.authorize). The writes
	// act on the signed-in viewer and take no person id, so nobody can record an
	// acceptance on somebody else's behalf.
	//
	// accept describes accepting the Terms or the Privacy Policy, whose routes
	// differ only in the document's name.
	accept := func(document string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Accept the "+document).
				Describe("The viewer's own acceptance, of the version the web app shows.").
				Viewer(apidoc.SignedIn).
				Takes(&requests.AgreementAccept{}).
				Sends(http.StatusOK, handlers.Accepted{}).
				Sends(http.StatusBadRequest, refusal, "`version_malformed`: the version is not a "+
					"dotted number; `not_accepted`: `accepted` is false; or, without a body, the "+
					"request is not well formed")
		}
	}
	agreements := api.Group("/agreements", r.authorize)

	agreements.PATCH("/tos", h.Agreements.EditTos).
		With(accept("Terms of Service"))

	agreements.PATCH("/privacy", h.Agreements.EditPrivacy).
		With(accept("Privacy Policy"))

	agreements.PATCH("/rules", h.Agreements.EditRules).
		Summary("Agree to the site's rules").
		Describe("The viewer's own agreement, each rule ticked, of the version `GET /site` has "+
			"now (roadmap row 102).").
		Viewer(apidoc.SignedIn).
		Takes(&requests.AgreementAccept{}).
		Sends(http.StatusOK, handlers.Accepted{}).
		Sends(http.StatusBadRequest, refusal,
			"`version_malformed`: the version is not a dotted number; `not_accepted`: `accepted` "+
				"is false; or, without a body, the request is not well formed").
		Sends(http.StatusConflict, refusal,
			"`version_not_current`: the site's rules are at another version now")

	// Reading takes a person id where the two writes take none, and the
	// asymmetry is deliberate: accepting on somebody's behalf is
	// meaningless and stays unexpressible, while reading an acceptance
	// is meaningful both to its subject and to an administrator working
	// out who is behind on the current terms.
	agreements.GET("/:person_id", h.Agreements.Read).
		Summary("What somebody has accepted").
		Describe("Their own, or anybody's, for an administrator.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountRead).
		Takes(&handlers.PersonRequest{}).
		Unbound().
		Sends(http.StatusOK, libs_responses.Item[handlers.Agreements]{}).
		Refuses(http.StatusBadRequest, "the id is not a number").
		Refuses(http.StatusForbidden,
			"it is somebody else, and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such person")

	// --- Onboarding ---
	// Whether the viewer has been through the walk-through the web app offers
	// after the agreements. Every route needs a token (r.authorize). Like
	// /agreements it takes no person id, because finishing somebody else's
	// onboarding means nothing.
	onboarding := api.Group("/onboarding", r.authorize)

	onboarding.GET("", h.Onboarding.Read).
		Summary("What the walk-through after signing up needs").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, handlers.OnboardingState{})

	onboarding.PATCH("", h.Onboarding.Edit).
		Summary("Say the viewer has been through the walk-through").
		Describe("Whether they finished it or skipped it: either way they are not sent back. Only "+
			"the first time is kept.").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, handlers.Onboarded{})
}
