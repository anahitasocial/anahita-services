package main

import (
	"net/http"

	"auth/cmd/api/handlers"
	"auth/requests"

	"anahita-libs/apidoc"
	libs_requests "anahita-libs/requests"
)

// assetAndCheckRoutes registers the files the pages of this service load, and what
// the forms ask while they are filled in, before anybody is signed in.
func (r *Routes) assetAndCheckRoutes(api *apidoc.Router) {
	h := r.handler

	// --- Page files ---
	// The styles, scripts and pictures of the sign-in, signup and consent pages.
	// Public: no middleware, so the caller is never known.
	api.Static("/static", "web/static").
		Summary("The pages' styles, scripts and pictures").
		Viewer(apidoc.Anyone).
		Media("application/octet-stream").
		Sends(http.StatusOK, apidoc.File{}, "the file").
		Refuses(http.StatusNotFound, "there is no such file")

	// --- Is it taken ---
	// What the forms ask about an address or a name. Public: no middleware, so the
	// caller is never known.
	auth := api.Group("/auth")

	auth.GET("/is/email", h.Is.Email).
		Summary("Whether an email address has an account").
		Describe("For the forms that ask for one.").
		Viewer(apidoc.Anyone).
		Takes(&libs_requests.EmailRequest{}).
		SendsNothing(http.StatusOK, "it has one").
		Refuses(http.StatusNotFound, "it has none")

	auth.GET("/is/username", h.Is.Username).
		Summary("Whether a username is taken").
		Describe("By a person, a group or an event: a username is the address of a profile, and "+
			"anybody may see it. The signup form asks as the field is left.").
		Viewer(apidoc.Anyone).
		Takes(&requests.UsernameAvailability{}).
		SendsNothing(http.StatusOK, "it is taken").
		Refuses(http.StatusNotFound, "it is free")

	auth.GET("/is/totp", h.Is.Totp).
		Summary("Whether an account has two-factor authentication").
		Describe("For the sign-in page, before there is a session.").
		Viewer(apidoc.Anyone).
		Takes(&libs_requests.EmailRequest{}).
		SendsNothing(http.StatusOK, "it has").
		Refuses(http.StatusNotFound, "it has not, or there is no such account")
}

// passwordResetRoutes registers the pages for somebody who has forgotten their
// password.
func (r *Routes) passwordResetRoutes(api *apidoc.Router) {
	h := r.handler

	// --- Password reset ---
	// Pages a browser is sent to, before anybody is signed in. Public: no
	// middleware, so the caller is never known. The code or the link is what
	// proves who it is. Changing the password while signed in is under /password
	// too, in a group of its own (accountChangeRoutes).
	password := api.Group("/password")

	password.GET("/forgot", h.Password.ForgotForm).
		With(form("The page that asks for a reset code",
			"Where somebody who has forgotten their password starts.")).
		Takes(&handlers.ExpiredPageRequest{})

	password.POST("/forgot", h.Password.Forgot).
		With(submit("Send a reset code",
			"Mails a six-digit code to the address. The answer is the same whether or not it has "+
				"an account.",
			&requests.PasswordForgot{}, "on to the page for the code"))

	password.GET("/reset/verify", h.Password.VerifyForm).
		Summary("The page that asks for the reset code").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Sends(http.StatusOK, page, "the page").
		SendsNothing(http.StatusFound,
			"back to the start, when no code is waiting, or on, when it was given")

	password.POST("/reset/verify", h.Password.Verify).
		With(submit("Check the reset code", "",
			&requests.CodeForm{},
			"on to the new password, or back to the start when the code ran out"))

	password.GET("/reset", h.Password.ResetForm).
		Summary("The page that asks for a new password").
		Describe("Reached from a reset link, or after the code.").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Takes(&handlers.LinkRequest{}).
		Unbound().
		Sends(http.StatusOK, page, "the page").
		SendsNothing(http.StatusFound,
			"back to the start, when there is neither a link nor a code").
		SendsNothing(http.StatusSeeOther, "back to the start, when the code ran out")

	password.POST("/reset", h.Password.Reset).
		Summary("Set a new password").
		Describe("Signs the account out everywhere, and removes its passkeys.").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Takes(&requests.PasswordReset{}).
		Sends(http.StatusOK, page, "a page that says it was changed, or what was wrong").
		SendsNothing(http.StatusSeeOther, "back to the start, when the code ran out")
}

// inviteAcceptRoutes registers the pages an invitation's link opens.
func (r *Routes) inviteAcceptRoutes(api *apidoc.Router) {
	h := r.handler

	// --- Accepting an invitation ---
	// Pages, for somebody with no account yet. Public: no middleware, so the
	// caller is never known. The token in the link is what lets them in. Browsing,
	// sending and taking back invitations are JSON for somebody signed in, under
	// the same path in a group of its own (inviteAdminRoutes).
	invitesPublic := api.Group("/invites")

	invitesPublic.GET("/accept", h.Invite.AcceptForm).
		Summary("The page an invitation's link opens").
		Describe("It asks for a username, a name and a password.").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Takes(&handlers.LinkRequest{}).
		Unbound().
		Sends(http.StatusOK, page, "the page, or one that says why the link does not work")

	invitesPublic.POST("/accept", h.Invite.Accept).
		With(submit("Accept an invitation",
			"Makes the account; where registration is by approval, asks for it instead.",
			&requests.InviteAccept{}, "on to the web app, signed in"))
}

// signupRoutes registers the signup pages, and confirming the address after.
func (r *Routes) signupRoutes(api *apidoc.Router) {
	h := r.handler

	// --- Signing up ---
	// Pages, beside the other ways of proving who somebody is. Public: no
	// middleware, so the caller is never known.
	//
	// The JSON POST /signups the web app used to call is gone. A client cannot
	// enforce whether a site takes registrations, and it was posting a password
	// through a bundle with hundreds of packages behind it.
	api.GET("/signup", h.Signup.Form).
		With(form("The signup page",
			"Or a page that says the site takes no registrations, or only by invitation."))

	api.POST("/signup", h.Signup.Add).
		With(submit("Sign up",
			"Mails a code to confirm the address. Where registration is by approval, the request "+
				"waits for an administrator once it is confirmed.",
			requests.NewSignupRequest().Add, "on to the page for the code"))

	// Confirming the address is public too, like the password reset pages: the
	// person arrives from a link in their mailbox, on a device that may hold no
	// session at all.
	api.GET("/signup/confirm", h.Signup.Confirm).
		Summary("The page that asks for the signup code").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Sends(http.StatusOK, page, "the page").
		SendsNothing(http.StatusFound, "on to ask for a new code, when none is waiting")

	api.POST("/signup/confirm", h.Signup.ConfirmCode).
		With(submit("Confirm the address",
			"Makes the account, or sends the request to the administrators.",
			&requests.CodeForm{}, "on to ask for a new code, when this one ran out"))

	api.GET("/signup/confirm/resend", h.Signup.ResendForm).
		With(form("The page that asks for a new signup code", "")).
		Takes(&handlers.ExpiredPageRequest{})

	api.POST("/signup/confirm/resend", h.Signup.Resend).
		With(submit("Send a new signup code",
			"The answer is the same whether or not a signup is waiting.",
			&requests.ConfirmResend{}, "on to the page for the code"))
}
