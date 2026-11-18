package main

import (
	"encoding/json"
	"net/http"

	"auth/cmd/api/handlers"
	"auth/requests"
	"auth/responses"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
	libs_responses "anahita-libs/responses"
)

// totpRoutes registers two-factor authentication with the passcodes of an
// authenticator app.
func (r *Routes) totpRoutes(api *apidoc.Router) {
	h := r.handler

	// --- Two-factor authentication ---
	// Every route needs a token (r.authorize). Each is the viewer's own, except
	// reading and turning off somebody else's, which is an administrator's; the
	// handlers check.
	totps := api.Group("/totp", r.authorize)

	// Status read. Registered on "" so the path is /totp, matching the
	// POST and PATCH below — Echo matches paths literally, so /totp/
	// with a trailing slash is a different path and answers 404.
	totps.GET("", h.Totp.Read).
		Summary("Whether the viewer has two-factor authentication").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, libs_responses.Item[handlers.TotpStatus]{})

	totps.POST("/verify", h.Totp.Verify).
		Summary("Check a passcode").
		Describe("Signing in asks for one again once the last is too long ago; this is that.").
		Viewer(apidoc.SignedIn).
		Takes(&requests.Totp{}).
		SendsNothing(http.StatusOK, "it is right").
		Refuses(http.StatusForbidden,
			"it is wrong, or the viewer has no two-factor authentication").
		Refuses(http.StatusNotFound, "the viewer has no account")

	totps.POST("", h.Totp.Add).
		Summary("Begin setting up two-factor authentication").
		Describe("Answers the QR code to pair an authenticator with. It is not on until "+
			"`/totp/recovery-codes` and `PATCH /totp`.").
		Viewer(apidoc.SignedIn).
		Media("image/png").
		Takes(&requests.TotpPassword{}).
		Sends(http.StatusCreated, apidoc.File{}, "the QR code").
		Refuses(http.StatusUnauthorized, "the password is wrong; or no token").
		Refuses(http.StatusConflict, "it is on: turn it off first").
		Refuses(http.StatusNotFound, "the viewer has no account")

	totps.PATCH("", h.Totp.Edit).
		Summary("Turn two-factor authentication on").
		Describe("Once it is set up: paired, and the recovery codes handed out.").
		Viewer(apidoc.SignedIn).
		SendsNothing(http.StatusOK, "on").
		Refuses(http.StatusConflict, "it is not set up").
		Refuses(http.StatusNotFound, "the viewer has no account")

	totps.DELETE("/:person_id", h.Totp.Delete).
		Summary("Turn two-factor authentication off").
		Describe("The viewer's own; or, for an administrator, somebody's who has lost their "+
			"authenticator. They are told by mail.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeActorsWrite).
		StepUp().
		Takes(requests.NewTotpRequest().Delete).
		SendsNothing(http.StatusNoContent, "off, or it was").
		Sends(http.StatusForbidden, refusal, stepUp+"; `forbidden`: the viewer may not act on "+
			"this account").
		Refuses(http.StatusNotFound, "there is no such person")

	totps.POST("/recovery-codes", h.Totp.AddRecoveryCodes).
		Summary("Get recovery codes").
		Describe("With a passcode from the authenticator just paired, which proves it works. Each "+
			"code signs in once in place of a passcode. A new set replaces the old, and the owner "+
			"is told by mail.").
		Viewer(apidoc.SignedIn).
		Takes(&requests.TotpRecoveryCodes{}).
		Sends(http.StatusOK, []string{}, "the codes, shown this once").
		Refuses(http.StatusForbidden, "the passcode is wrong").
		Refuses(http.StatusNotFound, "the viewer has no account")

	totps.GET("/is/expired", h.Totp.IsExpired).
		Summary("Whether the viewer's last passcode is too long ago").
		Viewer(apidoc.SignedIn).
		SendsNothing(http.StatusOK, "it is").
		SendsNothing(http.StatusNoContent, "it is not").
		Refuses(http.StatusNotFound, "the viewer has no account")

	// The admin counterpart to GET /totp, which answers for the viewer.
	// Registered after /is/expired so the static path wins: Echo matches
	// literal segments before parameters, but the ordering says the
	// intent out loud rather than relying on it.
	totps.GET("/:person_id", h.Totp.ReadForPerson).
		Summary("Whether somebody has two-factor authentication").
		Describe("For an administrator about to turn it off.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountRead).
		Takes(&handlers.PersonRequest{}).
		Unbound().
		Sends(http.StatusOK, libs_responses.Item[handlers.TotpStatus]{}).
		Refuses(http.StatusBadRequest, "the id is not a number").
		Refuses(http.StatusForbidden,
			"it is somebody else, and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such person")
}

// webAuthnRoutes registers the passkeys the viewer signs in with.
func (r *Routes) webAuthnRoutes(api *apidoc.Router) {
	h := r.handler

	// --- Passkeys ---
	// Passkey (WebAuthn) credential management. Every route needs a token
	// (r.authorize). Every route but the last two is a person acting on their own
	// credentials: there is no admin view, deliberately. See
	// permissions/webauthn.go.
	//
	// Registration is split begin/finish because the ceremony is stateful: /begin
	// issues a challenge and stashes it, /finish verifies the authenticator's
	// signature over that challenge. Neither half is meaningful alone.
	//
	// Note the asymmetry with the routes for signing in: enrolling a passkey
	// requires a session that is signed in (this group carries authorize), while
	// signing in with one does not.

	// webAuthnReq holds what the routes on one passkey, or on one person's, take.
	webAuthnReq := requests.NewWebAuthnCredentialRequest()
	webAuthn := api.Group("/webauthn", r.authorize)

	webAuthn.GET("/credentials", h.WebAuthn.Browse).
		Summary("The viewer's passkeys").
		Describe("Enough to tell them apart: never the keys.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountRead).
		Sends(http.StatusOK, libs_responses.Unpaged[*responses.WebAuthnCredentialItem]{})

	webAuthn.POST("/credentials/register/begin", h.WebAuthn.AddRegisterBegin).
		Summary("Begin adding a passkey").
		Describe("The options to hand to `navigator.credentials.create()`.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountWrite).
		Sends(http.StatusOK, json.RawMessage{}, "the options (WebAuthn)").
		Sends(http.StatusConflict, refusal, "`credential_limit_reached`: remove a passkey first")

	webAuthn.POST("/credentials/register/finish", h.WebAuthn.AddRegisterFinish).
		Summary("Add a passkey").
		Describe("What `navigator.credentials.create()` answered, as the browser gives it. It is "+
			"named after the device; the owner is told by mail.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountWrite).
		TakesBody(json.RawMessage{}).
		Sends(http.StatusCreated, libs_responses.Item[*responses.WebAuthnCredentialItem]{}).
		Sends(http.StatusBadRequest, refusal, "`ceremony_expired`: it was begun too long ago, or "+
			"not at all; `registration_failed`: the passkey could not be checked")

	webAuthn.PATCH("/credentials/:id", h.WebAuthn.Edit).
		Summary("Rename a passkey").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountWrite).
		Takes(webAuthnReq.Edit).
		Sends(http.StatusOK, libs_responses.Item[*responses.WebAuthnCredentialItem]{}).
		Refuses(http.StatusNotFound, "the viewer has no such passkey")

	webAuthn.DELETE("/credentials/:id", h.WebAuthn.Delete).
		Summary("Remove a passkey").
		Describe("The owner is told by mail.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountWrite).
		Takes(webAuthnReq.Delete).
		SendsNothing(http.StatusOK, "removed").
		Refuses(http.StatusNotFound, "the viewer has no such passkey")

	// Revoke-all. Global admins only (self-service is the exception —
	// acting on your own account needs no elevation). Deliberately has
	// no listing counterpart: an admin can strip someone's passkeys
	// without ever seeing which devices they held. See
	// permissions/webauthn.go.
	//
	// The extra path segment keeps this out of the way of
	// /credentials/:id above — Echo matches the static "person"
	// segment before falling back to the parameter.
	webAuthn.GET("/credentials/user/:user_id", h.WebAuthn.ReadCountForPerson).
		Summary("How many passkeys somebody has").
		Describe("A number only: which devices they hold is theirs. Their own, or anybody's, for "+
			"an administrator.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountWrite).
		Takes(webAuthnReq.User).
		Sends(http.StatusOK, libs_responses.Item[handlers.PasskeyCount]{}).
		Refuses(http.StatusForbidden,
			"it is somebody else, and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such person")

	webAuthn.DELETE("/credentials/user/:user_id", h.WebAuthn.DeleteAll).
		Summary("Remove all of somebody's passkeys").
		Describe("Their own, or anybody's, for an administrator, who must have just proved who "+
			"they are; the owner is told by mail. They still sign in with their password.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountWrite).
		StepUp().
		Takes(webAuthnReq.User).
		Sends(http.StatusOK, libs_responses.Item[handlers.PasskeysRevoked]{}).
		Sends(http.StatusForbidden, refusal, stepUp+"; or, without a body, it is somebody else, "+
			"and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such person")

}
