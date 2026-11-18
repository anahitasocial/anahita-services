package main

import (
	"encoding/json"
	"net/http"

	"auth/cmd/api/handlers"
	"auth/requests"
	"auth/responses"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
)

// accountChangeRoutes registers what the signed-in viewer changes about how they
// sign in, and proving who they are again before such a change.
func (r *Routes) accountChangeRoutes(api *apidoc.Router) {
	h := r.handler

	// --- Changing the password ---
	// Every route needs a token (r.authorize). It is the same /password path as
	// the public reset pages, in a group of its own on purpose: which routes need
	// a token can be seen at a glance, with no middleware chosen route by route
	// inside one group.
	passwordAuth := api.Group("/password", r.authorize)

	passwordAuth.PATCH("/change", h.Password.Edit).
		Summary("Change the viewer's password").
		Describe("Signs out every other session. Passkeys are kept.").
		Viewer(apidoc.SignedIn).
		StepUp().
		Takes(&requests.PasswordEdit{}).
		Sends(http.StatusOK, handlers.PasswordChanged{}).
		Sends(http.StatusForbidden, refusal, stepUp).
		Refuses(http.StatusConflict, "it is the password the account has").
		Refuses(http.StatusTooManyRequests, "too many tries; wait")

	// --- Changing the email address ---
	// Every route needs a token (r.authorize). Step one also needs the viewer to
	// have just proved who they are; it mails a code to the person's CURRENT
	// address and changes nothing by itself. A group of its own, like /password
	// and /invites, so which paths need a token is legible at a glance.
	emailChangeAuth := api.Group("/email", r.authorize)

	emailChangeAuth.PATCH("/change", h.EmailChange.Edit).
		Summary("Begin changing the viewer's email address").
		Describe("Mails a code to the address the account has now: that inbox approves the move, "+
			"so a stolen session alone cannot.").
		Viewer(apidoc.SignedIn).
		StepUp().
		Takes(&requests.EmailChangeEdit{}).
		SendsNothing(http.StatusNoContent,
			"the code is sent, or the address is the one the account has").
		Sends(http.StatusForbidden, refusal, stepUp).
		Refuses(http.StatusConflict, "another account has that address")

	// Step two: the code that was mailed to the current address, typed back
	// into the app. Signed in, like step one. The code proves the inbox, the
	// session proves whose change it is.
	emailChangeAuth.POST("/change/confirm", h.EmailChange.Confirm).
		Summary("Change the viewer's email address").
		Describe("With the code mailed to the old address. Every session is signed out, this one "+
			"too.").
		Viewer(apidoc.SignedIn).
		Takes(&requests.EmailChangeConfirm{}).
		SendsNothing(http.StatusNoContent, "changed").
		Refuses(http.StatusBadRequest, "it is not six digits; nothing was counted").
		Refuses(http.StatusConflict, "another account has taken the address since").
		Refuses(http.StatusGone, "no change is waiting, or its code ran out, was replaced or had "+
			"too many wrong tries: start again").
		Sends(http.StatusUnprocessableEntity, handlers.CodeMismatch{}, "the code is wrong; it "+
			"says how many tries are left")

	// --- Changing the username ---
	// Every route needs a token (r.authorize). It needs the viewer to have just
	// proved who they are, and unlike the email change it is one step: a handle
	// move does not relocate any recovery path, so there is nothing for the
	// mailbox to approve first. See the note on usernameChangeHandler.
	usernameChangeAuth := api.Group("/username", r.authorize)

	usernameChangeAuth.PATCH("/change", h.UsernameChange.Edit).
		Summary("Change the viewer's username").
		Describe("Which is also the address of their profile.").
		Viewer(apidoc.SignedIn).
		StepUp().
		Takes(&requests.UsernameChangeEdit{}).
		Sends(http.StatusOK, handlers.UsernameChanged{}).
		SendsNothing(http.StatusNoContent, "it is the username they have").
		Sends(http.StatusForbidden, refusal, stepUp).
		Refuses(http.StatusConflict, "it is taken")

	// --- Proving who the viewer is again ---
	// Step-up re-authentication. Every route needs a token (r.authorize). It
	// proves the caller is who the session says they are and records a short-lived
	// marker, which the endpoints that change credentials check for. Nothing here
	// changes anything, which is why it sits behind plain authorize with no
	// step-up gate of its own: it IS the gate.
	//
	// Note the asymmetry with the passkey routes for signing in, which need no
	// token by construction: asserting a passkey to sign in has no session yet,
	// while asserting one to authorise a change starts from a session that is
	// simply not sufficient on its own.
	reauth := api.Group("/reauth", r.authorize)

	reauth.POST("/password", h.Reauth.AddPassword).
		Summary("Prove who the viewer is, with their password").
		Describe("And their passcode, when they have two-factor authentication. Good for five "+
			"minutes, and for one change.").
		Viewer(apidoc.SignedIn).
		Takes(&requests.ReauthPassword{}).
		SendsNothing(http.StatusNoContent, "proved").
		Refuses(http.StatusUnauthorized, "the password is wrong").
		Refuses(http.StatusForbidden, "the passcode is wrong, or missing").
		Refuses(http.StatusTooManyRequests, "too many tries; wait")

	reauth.POST("/passkey/begin", h.Reauth.AddPasskeyBegin).
		Summary("Begin proving who the viewer is, with a passkey").
		Describe("The options to hand to `navigator.credentials.get()`: one of the viewer's own "+
			"passkeys.").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, json.RawMessage{}, "the options (WebAuthn)").
		Refuses(http.StatusConflict, "the viewer has no passkeys: use the password")

	reauth.POST("/passkey/finish", h.Reauth.AddPasskeyFinish).
		Summary("Prove who the viewer is, with a passkey").
		Describe("What `navigator.credentials.get()` answered, as the browser gives it.").
		Viewer(apidoc.SignedIn).
		TakesBody(json.RawMessage{}).
		SendsNothing(http.StatusNoContent, "proved").
		Sends(http.StatusBadRequest, refusal,
			"`ceremony_expired`: it was begun too long ago, or not at all").
		Sends(http.StatusUnauthorized, refusal, "`assertion_failed`: the passkey was not "+
			"accepted; or, without a body, no token")
}

// authLogRoutes registers the sessions somebody is signed in with.
func (r *Routes) authLogRoutes(api *apidoc.Router) {
	h := r.handler

	// --- Sessions ---
	// Every route needs a token (r.authorize). Each is about the viewer's own
	// sessions, or anybody's for an administrator, which the handlers check.
	authLogs := api.Group("/auth-logs", r.authorize)

	authLogs.GET("/:id", h.AuthLog.Browse).
		Summary("Where somebody is signed in").
		Describe("Each session: the device, roughly where, when it began, whether it is still "+
			"live, and which is the viewer's own. Their own, or anybody's, for an administrator.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountRead).
		Takes(requests.NewAuthLogRequest().Browse).
		Sends(http.StatusOK, []responses.AuthLogItem{}).
		Refuses(http.StatusForbidden,
			"it is somebody else, and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such person")

	// Path is /auth-logs/{userID}/{authLogID}. The param names must match
	// the `param:` tags on AuthLogDeleteRequest — Echo binds by name, so
	// a mismatch here leaves the field zero and fails validation rather
	// than erroring at bind time.
	authLogs.DELETE("/:uid/:id", h.AuthLog.Delete).
		Summary("Sign a session out").
		Describe("Somebody whose session an administrator ended is told by mail.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountWrite).
		Takes(requests.NewAuthLogRequest().Delete).
		SendsNothing(http.StatusOK, "signed out").
		Refuses(http.StatusForbidden,
			"it is somebody else's, and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such person, or no such session of theirs")
}
