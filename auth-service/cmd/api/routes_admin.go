package main

import (
	"net/http"

	"auth/cmd/api/handlers"
	"auth/requests"
	"auth/responses"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
	libs_requests "anahita-libs/requests"
	libs_responses "anahita-libs/responses"
)

// inviteAdminRoutes registers the invitations the viewer sends.
func (r *Routes) inviteAdminRoutes(api *apidoc.Router) {
	h := r.handler

	// --- Invitations ---
	// Browsing, sending and taking back invitations, in JSON. Every route needs a
	// token (r.authorize). It is the same /invites path as the public accept
	// pages, in a group of its own, as with /password.
	invitesAuth := api.Group("/invites", r.authorize)

	invitesAuth.GET("", h.Invite.Browse).
		Summary("The invitations the viewer has sent").
		Describe("Their own only, administrators' too: an invitation carries somebody's address. "+
			"With how many are waiting; how many may is the viewer's entitlement, in userinfo.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeActorsRead).
		Takes(libs_requests.NewBrowse()).
		Sends(http.StatusOK, responses.InviteList{}).
		Refuses(http.StatusForbidden, "the viewer may not invite")

	invitesAuth.POST("", h.Invite.Add).
		Summary("Invite somebody").
		Describe("Mails them a link. Who may invite is the installation's choice "+
			"(`INVITES_FROM`), and so is how many may wait at once (`INVITE_PENDING_MAX`).").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeActorsWrite).
		Takes(&requests.InviteAdd{}).
		Sends(http.StatusOK, responses.InviteItem{}).
		Sends(http.StatusForbidden, refusal, "`invitations_closed`: the site takes no one new; "+
			"or, without a body, the viewer may not invite").
		Sends(http.StatusConflict, refusal,
			"`too_many_invitations`: the viewer has as many waiting as they may, and it says how "+
				"one frees; or, without a body, the address has an account").
		Refuses(http.StatusTooManyRequests,
			"too many invitations, from the viewer or to the address, for now")

	invitesAuth.DELETE("/:id", h.Invite.Delete).
		Summary("Take an invitation back").
		Describe("Its link stops working, and the person invited is told.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeActorsWrite).
		Takes(&requests.InviteDelete{}).
		Sends(http.StatusOK, responses.InviteItem{}, "the invitation as it was").
		Refuses(http.StatusForbidden,
			"it is somebody else's, and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such invitation")
}

// adminRoutes registers what administrators do about the site's accounts: the
// approval queue, cleaning up, and account types.
func (r *Routes) adminRoutes(api *apidoc.Router) {
	h := r.handler

	// --- The approval queue ---
	// Every route needs a token (r.authorize). Only an administrator may use it,
	// which the handlers check. JSON: the screen that uses it belongs with the
	// other settings work.
	//
	// approve and reject are named in the path rather than being a bare POST and
	// DELETE the way follow requests are answered. Ignoring a follow request is a
	// silent dismissal; rejecting a signup request records a decision note, stamps
	// the deciding administrator and sends mail, and the route says so.

	// undecidable is the conflict both answers to a request share.
	undecidable := "`already_decided`: it has been answered; `not_verified`: they have not " +
		"confirmed their address"
	signupRequests := api.Group("/signup-requests", r.authorize)

	signupRequests.GET("", h.SignupRequests.Browse).
		Summary("The requests to join waiting for an answer").
		Describe("Where registration is by approval: oldest first, each with whether the username "+
			"asked for is still free.").
		Viewer(apidoc.Admin).
		Scope(libs_constants.OAuthScopeActorsRead).
		Takes(&requests.SignupRequestBrowse{}).
		Sends(http.StatusOK, libs_responses.List[handlers.SignupRequestItem]{})

	signupRequests.POST("/:id/approve", h.SignupRequests.Approve).
		Summary("Approve a request to join").
		Describe("Makes the account, and tells them by mail.").
		Viewer(apidoc.Admin).
		Scope(libs_constants.OAuthScopeActorsWrite).
		Takes(&requests.SignupRequestDecision{}).
		Sends(http.StatusOK, handlers.SignupDecided{}).
		Refuses(http.StatusNotFound, "there is no such request").
		Sends(http.StatusConflict, refusal, undecidable+"; `username_taken`: somebody has the "+
			"username now")

	signupRequests.POST("/:id/reject", h.SignupRequests.Reject).
		Summary("Turn down a request to join").
		Describe("They are told by mail; the note is not sent.").
		Viewer(apidoc.Admin).
		Scope(libs_constants.OAuthScopeActorsWrite).
		Takes(&requests.SignupRequestDecision{}).
		Sends(http.StatusOK, handlers.SignupDecided{}).
		Refuses(http.StatusNotFound, "there is no such request").
		Sends(http.StatusConflict, refusal, undecidable)

	// --- Accounts ---
	// The administration list of people and groups, for site cleanup. Every route
	// needs a token (r.authorize). Super administrators only, enforced in the
	// handler, except changing a person's type, which is an administrator's too.
	// Removing what it lists is person-service's and group-service's purge.
	accounts := api.Group("/accounts", r.authorize)

	accounts.GET("", h.Accounts.Browse).
		Summary("People, groups, events or channels, for cleaning up a site").
		Describe("Least recently active first, by default; purged ones are not listed. Removing "+
			"them is the purge (`DELETE /people`, `DELETE /groups`, account-lifecycle.md).").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeActorsRead).
		Takes(&requests.AccountBrowseRequest{}).
		Sends(http.StatusOK, libs_responses.List[handlers.AccountItem]{})

	// How much is public, and making all of it registered-only in one step.
	// For an installation that has stopped allowing new public content.
	accounts.GET("/public-content", h.Accounts.PublicContent).
		Summary("How much is public").
		Describe("What making it all registered-only would change.").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeActorsRead).
		Sends(http.StatusOK, handlers.PublicContent{})

	accounts.POST("/make-registered", h.Accounts.MakeRegistered).
		Summary("Make everything public registered-only").
		Describe("Every public profile, group and post, for an installation that has set "+
			"`MIN_CONTENT_ACCESS=registered`. It cannot be undone: the number is typed to confirm "+
			"it.").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeActorsWrite).
		StepUp().
		Takes(&handlers.MakeRegisteredRequest{}).
		Sends(http.StatusOK, handlers.PublicContent{}, "what was changed").
		Sends(http.StatusBadRequest, handlers.ConfirmationMismatch{}, "`confirmation_mismatch`: "+
			"the number is not how much is public now, which it says; or, without a body, the "+
			"request is not well formed").
		Sends(http.StatusForbidden, refusal, stepUp+"; or, without a body, the viewer may not").
		Sends(http.StatusConflict, refusal,
			"`public_content_allowed`: new content may still be public here")

	// A person's type, for an administrator, never their own (the user,
	// 2026-10-18). See handlers/person_type.go.
	accounts.PATCH("/:id/person-type", h.Accounts.EditPersonType).
		Summary("Change a person's type").
		Describe("Registered, administrator or super administrator. An administrator may make "+
			"somebody registered an administrator; making a super administrator, and taking a "+
			"type away, are a super administrator's. Nobody changes their own, so the last super "+
			"administrator cannot be demoted: one who wants to step down makes somebody else a "+
			"super administrator first. Recorded with who did it (`made-registered`, "+
			"`made-admin`, `made-super-admin`). It takes effect at the person's next token, "+
			"within fifteen minutes.").
		Viewer(apidoc.Admin).
		Scope(libs_constants.OAuthScopeActorsWrite).
		StepUp().
		Takes(&handlers.PersonTypeRequest{}).
		Sends(http.StatusOK, handlers.PersonTypeChanged{}, "the type they have now and had "+
			"before; `changed` is false when they had it already, and no proof was needed").
		Sends(http.StatusForbidden, refusal, stepUp+"; `person_type_not_allowed`: an "+
			"administrator asked for what is a super administrator's; or, without a body, the "+
			"viewer is not an administrator").
		Sends(http.StatusNotFound, refusal, "`not_a_person`: no person here has that id; a group, "+
			"a profile from another server and a deleted person have no type to change").
		Sends(http.StatusConflict, refusal, "`own_person_type`: it is the viewer's own")
}
