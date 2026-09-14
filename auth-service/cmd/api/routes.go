package main

import (
	"auth/cmd/api/handlers"

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

func (r *Routes) Init(e *echo.Echo) {
	h := r.handler

	// Static assets for the auth-service login/consent UI.
	e.Static("/static", "web/static")

	// ---------------------------------------------------------------
	// Public unauthenticated endpoints.
	// ---------------------------------------------------------------

	auth := e.Group("/auth")
	auth.GET("/is/email", h.Is.Email)
	auth.GET("/is/username", h.Is.Username)
	auth.GET("/is/totp", h.Is.Totp)

	// Password — forgot and reset are server-rendered, unauthenticated
	// flows. Change (PATCH) is JSON-only and authenticated; registered
	// further down with the other JWT-protected routes so it picks up
	// the authorize middleware.
	// Email-change confirmation. Public and token-gated, like the
	// password reset pages beside it: the person arrives from a link in
	// their mailbox, on a device that may hold no session at all — which
	// is the point, since the whole control is "the current inbox has to
	// approve this".
	//
	// The token in the link is the only credential, so these must NOT
	// carry authorize.
	emailChange := e.Group("/email/change")
	emailChange.GET("/confirm", h.EmailChange.ConfirmForm)
	emailChange.POST("/confirm", h.EmailChange.Confirm)

	password := e.Group("/password")
	password.GET("/forgot", h.Password.ForgotForm)
	password.POST("/forgot", h.Password.Forgot)
	password.GET("/reset", h.Password.ResetForm)
	password.POST("/reset", h.Password.Reset)

	// Invites — accept is server-rendered, token-gated, unauthenticated.
	// Browse, Add, and Delete are JSON admin operations; registered
	// further down with the other JWT-protected routes.
	invitesPublic := e.Group("/invites")
	invitesPublic.GET("/accept", h.Invite.AcceptForm)
	invitesPublic.POST("/accept", h.Invite.Accept)

	// Signup — server-rendered, beside the other credential flows.
	//
	// The JSON POST /signups the web app used to call is gone. A client
	// cannot enforce whether a site takes registrations, and it was posting a
	// password through a bundle with hundreds of packages behind it.
	e.GET("/signup", h.Signup.Form)
	e.POST("/signup", h.Signup.Add)

	// Email confirmation. Public and token-gated, like the password reset
	// pages beside it — the person arrives from a link in their mailbox, on a
	// device that may hold no session at all.
	e.GET("/signup/confirm", h.Signup.Confirm)
	e.GET("/signup/confirm/resend", h.Signup.ResendForm)
	e.POST("/signup/confirm/resend", h.Signup.Resend)

	// The agreement versions the signup form has to send back with an
	// acceptance. Public: the page that reads it belongs to somebody who does
	// not have an account yet.
	e.GET("/agreements", h.Agreements.Read)

	// OIDC discovery and JWKS — fully public.
	e.GET("/.well-known/openid-configuration", h.OAuth.Discovery)
	e.GET("/.well-known/jwks.json", h.OAuth.JWKS)

	// NodeInfo — how this installation describes itself to the fediverse and
	// to its own web app. Public by definition: the point is that a server
	// which has never spoken to this one can read it.
	//
	// Two hops because that is the protocol: the well-known document is a
	// list of links, and a consumer follows the schema version it knows.
	e.GET("/.well-known/nodeinfo", h.NodeInfo.Discovery)
	e.GET("/nodeinfo/2.1", h.NodeInfo.Read)

	// ---------------------------------------------------------------
	// OAuth user-facing endpoints (HTML flows + BFF endpoints).
	//
	// All read the IdP session cookie or oauth_tokens cookie directly.
	// They do NOT use JWT middleware — these endpoints either come
	// before a JWT exists in the flow (login, authorize, consent) or
	// they ARE the JWT-issuing/validating endpoints (session, validate,
	// logout).
	//
	// Login is a two-step flow: POST /login validates credentials and,
	// if the account has TOTP enrolled, redirects to GET /login/totp
	// for code entry. Accounts without TOTP finalize the IdP session
	// directly from POST /login.
	//
	// The passkey routes are a parallel, independently sufficient path
	// to the same IdP session. They are unauthenticated for the same
	// reason POST /login is — they are how a person authenticates. No
	// TOTP step follows: user verification is Required in the RP
	// config, so the biometric or PIN gesture already supplied the
	// second factor. See data/services/webauthn.go.
	// ---------------------------------------------------------------

	oauth := e.Group("/oauth")
	oauth.GET("/login", h.OAuth.ReadLogin)
	oauth.POST("/login", h.OAuth.AddCredentials)
	oauth.GET("/login/totp", h.OAuth.ReadTotp)
	oauth.POST("/login/totp", h.OAuth.AddTotp)
	oauth.POST("/login/passkey/begin", h.OAuth.AddPasskeyLoginBegin)
	oauth.POST("/login/passkey/finish", h.OAuth.AddPasskeyLoginFinish)

	// Account restore, reached only from the login form after a correct
	// password on an account scheduled for deletion. Unauthenticated by
	// construction, like the rest of this group — the account is disabled, so
	// there is no session to hold and no way to authorize this except by the
	// password that was just checked.
	oauth.GET("/login/restore", h.OAuth.ReadRestore)
	oauth.POST("/login/restore", h.OAuth.AddRestore)
	oauth.GET("/authorize", h.OAuth.ReadAuthorize)
	oauth.GET("/consent", h.OAuth.ReadConsent)
	oauth.POST("/consent", h.OAuth.AddConsent)

	// BFF endpoints.
	oauth.POST("/session", h.OAuth.AddSession)
	oauth.GET("/validate", h.OAuth.ReadValidate)
	oauth.GET("/logout", h.OAuth.ReadLogout)

	// ---------------------------------------------------------------
	// OAuth protocol endpoints — client-authenticated, not user-authenticated.
	// ---------------------------------------------------------------

	oauthProto := e.Group("/oauth")
	oauthProto.POST("/token", h.OAuth.AddToken)
	oauthProto.POST("/revoke", h.OAuth.AddRevoke)
	oauthProto.POST("/introspect", h.OAuth.AddIntrospect)

	// ---------------------------------------------------------------
	// OIDC userinfo — user-authenticated via JWT middleware.
	// ---------------------------------------------------------------

	oauthUser := e.Group("/oauth")
	oauthUser.Use(r.authorize)
	oauthUser.GET("/userinfo", h.OAuth.ReadUserInfo)

	// ---------------------------------------------------------------
	// OAuth client management (admin only).
	// ---------------------------------------------------------------

	clients := e.Group("/clients")
	clients.Use(r.authorize)
	clients.GET("", h.OAuth.BrowseClients)
	clients.GET("/:id", h.OAuth.ReadClient)
	clients.POST("", h.OAuth.AddClient)
	clients.PATCH("/:id", h.OAuth.EditClient)
	clients.DELETE("/:id", h.OAuth.DeleteClient)
	clients.POST("/:id/rotate-secret", h.OAuth.RotateClientSecret)

	// ---------------------------------------------------------------
	// OAuth signing key management (admin only).
	// ---------------------------------------------------------------

	signingKeys := e.Group("/oauth-signing-keys")
	signingKeys.Use(r.authorize)
	signingKeys.GET("", h.OAuth.BrowseSigningKeys)
	signingKeys.GET("/:kid", h.OAuth.ReadSigningKey)
	signingKeys.PATCH("/:kid", h.OAuth.EditSigningKey)

	// ---------------------------------------------------------------
	// Protected user/account routes — JWT required.
	// ---------------------------------------------------------------

	// Password change. Same /password path as the public forgot/reset
	// routes above, but PATCH is gated by authorize. The split into
	// two groups is intentional: lets us see at a glance which methods
	// require auth and which don't, without method-level middleware
	// branching inside a single group.
	passwordAuth := e.Group("/password")
	passwordAuth.Use(r.authorize)
	passwordAuth.PATCH("/change", h.Password.Edit)

	// Email change, step one. Authenticated and step-up gated; mails a
	// confirmation link to the person's CURRENT address and changes
	// nothing by itself.
	//
	// A separate group from /email/change above rather than one group
	// with method-level middleware, matching the split used for
	// /password and /invites: it makes which paths require auth legible
	// at a glance. Echo resolves the more specific path first, so
	// /email/change/confirm is unaffected by the authorize on this one.
	emailChangeAuth := e.Group("/email")
	emailChangeAuth.Use(r.authorize)
	emailChangeAuth.PATCH("/change", h.EmailChange.Edit)

	// Username change. Authenticated and step-up gated, and unlike the
	// email change it is one step: a handle move does not relocate any
	// recovery path, so there is nothing for the mailbox to approve
	// first. See the note on usernameChangeHandler.
	usernameChangeAuth := e.Group("/username")
	usernameChangeAuth.Use(r.authorize)
	usernameChangeAuth.PATCH("/change", h.UsernameChange.Edit)

	// Step-up re-authentication. Proves the caller is who the session
	// says they are and records a short-lived marker; the endpoints
	// that mutate credentials check for it. Nothing here changes
	// anything, which is why it can sit behind plain authorize with no
	// step-up gate of its own — it IS the gate.
	//
	// Note the asymmetry with the passkey LOGIN routes, which are
	// unauthenticated by construction: asserting a passkey to sign in
	// has no session yet, while asserting one to authorise a change
	// starts from a session that is simply not sufficient on its own.
	reauth := e.Group("/reauth")
	reauth.Use(r.authorize)
	reauth.POST("/password", h.Reauth.AddPassword)
	reauth.POST("/passkey/begin", h.Reauth.AddPasskeyBegin)
	reauth.POST("/passkey/finish", h.Reauth.AddPasskeyFinish)

	authlogs := e.Group("/authlogs")
	authlogs.Use(r.authorize)
	authlogs.GET("/:id", h.Authlog.Browse)
	// Path is /authlogs/{userID}/{authlogID}. The param names must match
	// the `param:` tags on AuthlogDeleteRequest — Echo binds by name, so
	// a mismatch here leaves the field zero and fails validation rather
	// than erroring at bind time.
	authlogs.DELETE("/:uid/:id", h.Authlog.Delete)

	// Invite admin operations — same /invites path as the public
	// accept routes above, but Browse, Add, and Delete are gated by
	// authorize. Same split-into-two-groups pattern as /password.
	invitesAuth := e.Group("/invites")
	invitesAuth.Use(r.authorize)
	invitesAuth.GET("", h.Invite.Browse)
	invitesAuth.POST("", h.Invite.Add)
	invitesAuth.DELETE("/:id", h.Invite.Delete)

	totps := e.Group("/totp")
	totps.Use(r.authorize)
	// Status read. Registered on "" so the path is /totp, matching the
	// POST and PATCH below — Echo matches paths literally, so /totp/
	// with a trailing slash is a different path and answers 404.
	totps.GET("", h.Totp.Read)
	totps.POST("/verify", h.Totp.Verify)
	totps.POST("", h.Totp.Add)
	totps.PATCH("", h.Totp.Edit)
	totps.DELETE("/:person_id", h.Totp.Delete)
	totps.POST("/recoverycodes", h.Totp.AddRecoveryCodes)
	totps.GET("/is/expired", h.Totp.IsExpired)

	// Passkey (WebAuthn) credential management. Every route here is a
	// person acting on their own credentials — there is no admin view,
	// deliberately. See permissions/webauthn.go.
	//
	// Registration is split begin/finish because the ceremony is
	// stateful: /begin issues a challenge and stashes it, /finish
	// verifies the authenticator's signature over that challenge.
	// Neither half is meaningful alone.
	//
	// Note the asymmetry with the login routes above: enrolling a
	// passkey requires an existing authenticated session (this group
	// carries authorize), while signing in with one does not.
	webAuthn := e.Group("/webauthn")
	webAuthn.Use(r.authorize)
	webAuthn.GET("/credentials", h.WebAuthn.Browse)
	webAuthn.POST("/credentials/register/begin", h.WebAuthn.AddRegisterBegin)
	webAuthn.POST("/credentials/register/finish", h.WebAuthn.AddRegisterFinish)
	webAuthn.PATCH("/credentials/:id", h.WebAuthn.Edit)
	webAuthn.DELETE("/credentials/:id", h.WebAuthn.Delete)

	// Revoke-all. Global admins only (self-service is the exception —
	// acting on your own account needs no elevation). Deliberately has
	// no listing counterpart: an admin can strip someone's passkeys
	// without ever seeing which devices they held. See
	// permissions/webauthn.go.
	//
	// The extra path segment keeps this out of the way of
	// /credentials/:id above — Echo matches the static "person"
	// segment before falling back to the parameter.
	webAuthn.GET("/credentials/user/:user_id", h.WebAuthn.ReadCountForPerson)
	webAuthn.DELETE("/credentials/user/:user_id", h.WebAuthn.DeleteAll)
}
