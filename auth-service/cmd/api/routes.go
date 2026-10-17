package main

import (
	"encoding/json"
	"net/http"

	"auth/cmd/api/handlers"
	"auth/requests"
	"auth/responses"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
	libs_requests "anahita-libs/requests"
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

// Init registers the routes, each with its description, and answers with
// the descriptions: docs/api/auth-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	h := r.handler
	doc := apidoc.NewService("auth-service", "Accounts and signing in",
		"Signing up, signing in and out, and the OAuth 2.0 and OpenID Connect server every app signs in through (oauth.md); what proves who somebody is (passwords, codes, passkeys, two-factor authentication) and changing it; invitations and the approval queue; and the installation's description of itself (NodeInfo).")
	api := doc.On(e)

	stepUp := "`step_up_required`: the viewer must prove who they are again first (`/reauth`)"
	refusal := libs_responses.Refusal{}
	oauthError := responses.OAuthErrorResponse{}
	page := ""

	// A page of the sign-in, sign-up and recovery flows, which a browser
	// is sent to. Every one answers 200 with the page, a refusal among
	// them: an error is said on the page.
	form := func(summary, describe string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary(summary).
				Describe(describe).
				Viewer(apidoc.Anyone).
				Media("text/html").
				Sends(http.StatusOK, page, "the page")
		}
	}
	// What a page posts. 200 is the page again, saying what was wrong.
	submit := func(summary, describe string, takes any, next string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary(summary).
				Describe(describe).
				Viewer(apidoc.Anyone).
				Media("text/html").
				Takes(takes).
				Unbound().
				Sends(http.StatusOK, page, "the page again, saying what was wrong, or a page that says what happened").
				SendsNothing(http.StatusSeeOther, next)
		}
	}

	// Static assets for the auth-service login/consent UI.
	api.Static("/static", "web/static").
		Summary("The pages' styles, scripts and pictures").
		Viewer(apidoc.Anyone).
		Media("application/octet-stream").
		Sends(http.StatusOK, apidoc.File{}, "the file").
		Refuses(http.StatusNotFound, "there is no such file")

	// ---------------------------------------------------------------
	// Public unauthenticated endpoints.
	// ---------------------------------------------------------------

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
		Describe("By a person, a group or an event: a username is the address of a profile, and anybody may see it. The signup form asks as the field is left.").
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

	password := api.Group("/password")
	password.GET("/forgot", h.Password.ForgotForm).
		With(form("The page that asks for a reset code",
			"Where somebody who has forgotten their password starts.")).
		Takes(&handlers.ExpiredPageRequest{})
	password.POST("/forgot", h.Password.Forgot).
		With(submit("Send a reset code",
			"Mails a six-digit code to the address. The answer is the same whether or not it has an account.",
			&requests.PasswordForgot{}, "on to the page for the code"))
	password.GET("/reset/verify", h.Password.VerifyForm).
		Summary("The page that asks for the reset code").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Sends(http.StatusOK, page, "the page").
		SendsNothing(http.StatusFound, "back to the start, when no code is waiting, or on, when it was given")
	password.POST("/reset/verify", h.Password.Verify).
		With(submit("Check the reset code", "",
			&requests.CodeForm{}, "on to the new password, or back to the start when the code ran out"))
	password.GET("/reset", h.Password.ResetForm).
		Summary("The page that asks for a new password").
		Describe("Reached from a reset link, or after the code.").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Takes(&handlers.LinkRequest{}).
		Unbound().
		Sends(http.StatusOK, page, "the page").
		SendsNothing(http.StatusFound, "back to the start, when there is neither a link nor a code").
		SendsNothing(http.StatusSeeOther, "back to the start, when the code ran out")
	password.POST("/reset", h.Password.Reset).
		Summary("Set a new password").
		Describe("Signs the account out everywhere, and removes its passkeys.").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Takes(&requests.PasswordReset{}).
		Sends(http.StatusOK, page, "a page that says it was changed, or what was wrong").
		SendsNothing(http.StatusSeeOther, "back to the start, when the code ran out")

	// Invites — accept is server-rendered, token-gated, unauthenticated.
	// Browse, Add, and Delete are JSON admin operations; registered
	// further down with the other JWT-protected routes.
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

	// Signup — server-rendered, beside the other credential flows.
	//
	// The JSON POST /signups the web app used to call is gone. A client
	// cannot enforce whether a site takes registrations, and it was posting a
	// password through a bundle with hundreds of packages behind it.
	api.GET("/signup", h.Signup.Form).
		With(form("The signup page",
			"Or a page that says the site takes no registrations, or only by invitation."))
	api.POST("/signup", h.Signup.Add).
		With(submit("Sign up",
			"Mails a code to confirm the address. Where registration is by approval, the request waits for an administrator once it is confirmed.",
			requests.NewSignupRequest().Add, "on to the page for the code"))

	// Email confirmation. Public and token-gated, like the password reset
	// pages beside it — the person arrives from a link in their mailbox, on a
	// device that may hold no session at all.
	api.GET("/signup/confirm", h.Signup.Confirm).
		Summary("The page that asks for the signup code").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Sends(http.StatusOK, page, "the page").
		SendsNothing(http.StatusFound, "on to ask for a new code, when none is waiting")
	api.POST("/signup/confirm", h.Signup.ConfirmCode).
		With(submit("Confirm the address", "Makes the account, or sends the request to the administrators.",
			&requests.CodeForm{}, "on to ask for a new code, when this one ran out"))
	api.GET("/signup/confirm/resend", h.Signup.ResendForm).
		With(form("The page that asks for a new signup code", "")).
		Takes(&handlers.ExpiredPageRequest{})
	api.POST("/signup/confirm/resend", h.Signup.Resend).
		With(submit("Send a new signup code", "The answer is the same whether or not a signup is waiting.",
			&requests.ConfirmResend{}, "on to the page for the code"))

	// Accepting the Terms and the Privacy Policy, one document per request.
	// Authenticated, and acting on the signed-in viewer — no person id, so
	// nobody can record an acceptance on somebody else's behalf.
	accept := func(document string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Accept the "+document).
				Describe("The viewer's own acceptance, of the version the web app shows.").
				Viewer(apidoc.SignedIn).
				Takes(&requests.AgreementAccept{}).
				Sends(http.StatusOK, handlers.Accepted{}).
				Sends(http.StatusBadRequest, refusal, "`version_malformed`: the version is not a dotted number; `not_accepted`: `accepted` is false; or, without a body, the request is not well formed")
		}
	}
	agreements := api.Group("/agreements", r.authorize)
	agreements.PATCH("/tos", h.Agreements.EditTos).
		With(accept("Terms of Service"))
	agreements.PATCH("/privacy", h.Agreements.EditPrivacy).
		With(accept("Privacy Policy"))
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
		Refuses(http.StatusForbidden, "it is somebody else, and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such person")

	// Onboarding — whether the signed-in viewer has been through the
	// walk-through the web app offers after the agreements. Same shape as
	// /agreements: authenticated, and no person id, because finishing
	// somebody else's onboarding means nothing.
	onboarding := api.Group("/onboarding", r.authorize)
	onboarding.GET("", h.Onboarding.Read).
		Summary("What the walk-through after signing up needs").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, handlers.OnboardingState{})
	onboarding.PATCH("", h.Onboarding.Edit).
		Summary("Say the viewer has been through the walk-through").
		Describe("Whether they finished it or skipped it: either way they are not sent back. Only the first time is kept.").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, handlers.Onboarded{})

	// OIDC discovery and JWKS — fully public.
	api.GET("/.well-known/openid-configuration", h.OAuth.Discovery).
		Summary("The OpenID Connect discovery document").
		Describe("Where each endpoint is, and what is supported (OpenID Connect Discovery 1.0).").
		Viewer(apidoc.Anyone).
		Sends(http.StatusOK, handlers.OpenIDConfiguration{})
	api.GET("/.well-known/jwks.json", h.OAuth.JWKS).
		Summary("The keys tokens are signed with").
		Describe("The public halves, as a JSON Web Key Set (RFC 7517): what every service checks a token's signature with.").
		Viewer(apidoc.Anyone).
		Sends(http.StatusOK, handlers.JWKSet{})

	// NodeInfo — how this installation describes itself to the fediverse and
	// to its own web app. Public by definition: the point is that a server
	// which has never spoken to this one can read it.
	//
	// Two hops because that is the protocol: the well-known document is a
	// list of links, and a consumer follows the schema version it knows.
	api.GET("/.well-known/nodeinfo", h.NodeInfo.Discovery).
		Summary("Where the NodeInfo document is").
		Viewer(apidoc.Anyone).
		Sends(http.StatusOK, handlers.NodeInfoLinks{})
	api.GET("/nodeinfo/2.1", h.NodeInfo.Read).
		Summary("What this installation is (NodeInfo 2.1)").
		Describe("The software, the protocols it speaks, whether it takes registrations, and how many people use it. Its names are NodeInfo's, in camelCase.").
		Viewer(apidoc.Anyone).
		Sends(http.StatusOK, handlers.NodeInfoDocument{})

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

	signedIn := "on to where the person was going: back to the authorization that sent them to sign in, or to the web app"
	oauth := api.Group("/oauth")
	oauth.GET("/login", h.OAuth.ReadLogin).
		With(form("The sign-in page", "")).
		Takes(&handlers.LoginPageRequest{}).
		SendsNothing(http.StatusFound, "on, when the browser is signed in already")
	oauth.POST("/login", h.OAuth.AddCredentials).
		Summary("Sign in with a password").
		Describe("For an account with two-factor authentication, on to the page for the passcode. For one being deleted, the page that offers to restore it.").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Takes(requests.NewOAuthLoginRequest().Login).
		Unbound().
		Sends(http.StatusOK, page, "the page again, saying what was wrong, or the page that offers to restore the account").
		SendsNothing(http.StatusFound, signedIn+"; or on to the page for the passcode")
	// Signing in with an emailed code in place of the password. Public for
	// the same reason the password form is. See oauth_login_code.go.
	oauth.GET("/login/code", h.OAuth.ReadLoginCode).
		With(form("The page that asks who is signing in with a code", "")).
		Takes(&handlers.LoginCodePageRequest{})
	oauth.POST("/login/code", h.OAuth.AddLoginCode).
		With(submit("Say who is signing in with a code", "An email address or a username.",
			&requests.OAuthLoginCode{}, "on to choose where the code goes, or back when it was not given"))
	oauth.GET("/login/code/method", h.OAuth.ReadLoginCodeMethod).
		Summary("The page that asks where the code should go").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Sends(http.StatusOK, page, "the page").
		SendsNothing(http.StatusFound, "back to the start, or on, when there is only one place to send it")
	oauth.POST("/login/code/method", h.OAuth.AddLoginCodeMethod).
		With(submit("Send the code", "The answer is the same whether or not there is such an account.",
			&requests.OAuthLoginCodeMethod{}, "on to the page for the code"))
	oauth.GET("/login/code/verify", h.OAuth.ReadLoginCodeVerify).
		Summary("The page that asks for the code").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Sends(http.StatusOK, page, "the page").
		SendsNothing(http.StatusFound, "back to the start, when no code is waiting")
	oauth.POST("/login/code/verify", h.OAuth.AddLoginCodeVerify).
		Summary("Sign in with the code").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Takes(&requests.OAuthLoginCodeVerify{}).
		Unbound().
		Sends(http.StatusOK, page, "the page again, saying what was wrong and how many tries are left").
		SendsNothing(http.StatusFound, signedIn+"; or on to the page for the passcode").
		SendsNothing(http.StatusSeeOther, "back to the start, when the code ran out")
	oauth.POST("/login/code/resend", h.OAuth.AddLoginCodeResend).
		With(submit("Send a new code", "",
			&requests.OAuthLoginCodeMethod{}, "back to the page for the code"))
	oauth.GET("/login/totp", h.OAuth.ReadTotp).
		Summary("The page that asks for the passcode").
		Describe("The second step of signing in, for an account with two-factor authentication.").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Sends(http.StatusOK, page, "the page").
		SendsNothing(http.StatusFound, "back to the sign-in page, when the first step ran out")
	oauth.POST("/login/totp", h.OAuth.AddTotp).
		Summary("Sign in with the passcode").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Takes(&requests.OAuthLoginTotp{}).
		Unbound().
		Sends(http.StatusOK, page, "the page again, saying what was wrong").
		SendsNothing(http.StatusFound, signedIn+"; or back to the sign-in page, when the first step ran out or there were too many wrong passcodes")
	oauth.POST("/login/passkey/begin", h.OAuth.AddPasskeyLoginBegin).
		Summary("Begin signing in with a passkey").
		Describe("The options to hand to `navigator.credentials.get()`. The browser offers whichever of its passkeys are for this site.").
		Viewer(apidoc.Anyone).
		Sends(http.StatusOK, json.RawMessage{}, "the options (WebAuthn)")
	oauth.POST("/login/passkey/finish", h.OAuth.AddPasskeyLoginFinish).
		Summary("Sign in with a passkey").
		Describe("What `navigator.credentials.get()` answered, as the browser gives it. No passcode follows: the gesture that unlocked the passkey is the second factor.").
		Viewer(apidoc.Anyone).
		TakesBody(json.RawMessage{}).
		Sends(http.StatusOK, handlers.PasskeySignedIn{}).
		Sends(http.StatusBadRequest, refusal, "`ceremony_expired`: it was begun too long ago, or not at all").
		Sends(http.StatusUnauthorized, refusal, "`assertion_failed`: the passkey was not accepted")

	// Account restore, reached only from the login form after a correct
	// password on an account scheduled for deletion. Unauthenticated by
	// construction, like the rest of this group — the account is disabled, so
	// there is no session to hold and no way to authorize this except by the
	// password that was just checked.
	oauth.GET("/login/restore", h.OAuth.ReadRestore).
		Summary("The page that offers to restore an account being deleted").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Sends(http.StatusOK, page, "the page").
		SendsNothing(http.StatusFound, "back to the sign-in page, when no password was just given for such an account")
	oauth.POST("/login/restore", h.OAuth.AddRestore).
		Summary("Restore an account being deleted").
		Describe("Then the person signs in again.").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Sends(http.StatusOK, page, "a page that says it was restored, or why not").
		SendsNothing(http.StatusFound, "back to the sign-in page, when no password was just given for such an account")
	oauth.GET("/authorize", h.OAuth.ReadAuthorize).
		Summary("Ask to act for somebody (OAuth 2.0)").
		Describe("The authorization endpoint (RFC 6749, 4.1.1), with PKCE (RFC 7636). The person signs in first if they have to, and is asked whether to allow it unless the app is the installation's own.").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Takes(requests.NewOAuthAuthorizeRequest().Authorize).
		Sends(http.StatusOK, page, "a page that says why it cannot go on: no such app, or a redirect address it has not registered; or, for an app on a device, the page that hands it the code").
		SendsNothing(http.StatusFound, "back to the app's redirect address with `code` and `state`, or with `error` and `error_description`; or on to sign in, or to the consent page")
	oauth.GET("/consent", h.OAuth.ReadConsent).
		Summary("The page that asks whether to allow an app").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Sends(http.StatusOK, page, "the page, or one that says why it cannot go on").
		SendsNothing(http.StatusFound, "on to sign in, when the browser is not signed in")
	oauth.POST("/consent", h.OAuth.AddConsent).
		Summary("Allow an app, or not").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Takes(requests.NewOAuthAuthorizeRequest().Callback).
		Unbound().
		Sends(http.StatusOK, page, "a page that says why it cannot go on; or, for an app on a device, the page that hands it the code").
		SendsNothing(http.StatusFound, "back to the app's redirect address with `code` and `state`, or with `error=access_denied`; or on to sign in")

	// BFF endpoints.
	oauth.POST("/session", h.OAuth.AddSession).
		Summary("Start an app's session").
		Describe("For an app's own server (oauth.md): the authorization code is exchanged as `/oauth/token` would, and the tokens are kept in the session, not sent.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewOAuthTokenRequest().Token).
		Sends(http.StatusOK, handlers.SessionStarted{}).
		Sends(http.StatusBadRequest, oauthError, "`invalid_grant`: the code is wrong, used or expired; or, without a body, the request is not well formed").
		Sends(http.StatusUnauthorized, oauthError, "`invalid_client`: the app did not prove which it is")
	oauth.GET("/validate", h.OAuth.ReadValidate).
		Summary("The access token of the session").
		Describe("For the gateway, on every request: the token is in the `X-OAuth-Token` header of the answer, refreshed when it had expired, and missing when there is none. Always 200.").
		Viewer(apidoc.Anyone).
		SendsNothing(http.StatusOK, "the token, in a header, or none")
	oauth.GET("/logout", h.OAuth.ReadLogout).
		Summary("Sign out").
		Describe("Navigated to, not called: it signs out the app's session, then the sign-in session, each on its own host, and ends at `return_to`.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.LogoutRequest{}).
		Unbound().
		SendsNothing(http.StatusFound, "on to the next host, or to `return_to`")

	// ---------------------------------------------------------------
	// OAuth protocol endpoints — client-authenticated, not user-authenticated.
	// ---------------------------------------------------------------

	invalidClient := "`invalid_client`: the app did not prove which it is"
	oauthProto := api.Group("/oauth")
	oauthProto.POST("/token", h.OAuth.AddToken).
		Summary("Get tokens (OAuth 2.0)").
		Describe("The token endpoint (RFC 6749, 3.2): an authorization code, a refresh token or the app's own credentials for an access token. A refresh token is used once, and the answer has the next. A refusal of a refresh token says which in `error_code`: `refresh_raced` alone is not the end of the session, but another request refreshing first.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewOAuthTokenRequest().Token).
		Sends(http.StatusOK, responses.OAuthTokenResponse{}).
		Sends(http.StatusBadRequest, oauthError, "`invalid_grant`: the code or refresh token is wrong, used or expired; `unsupported_grant_type`: the app may not use this grant; `invalid_scope`: it asked for a scope it does not have; or, without a body, the request is not well formed").
		Sends(http.StatusUnauthorized, oauthError, invalidClient).
		Sends(http.StatusForbidden, oauthError, "`unauthorized_client`: the app may not act for itself")
	oauthProto.POST("/revoke", h.OAuth.AddRevoke).
		Summary("Revoke a token (OAuth 2.0)").
		Describe("RFC 7009. A refresh token is revoked; an access token is short-lived and runs out by itself. The answer is the same whether or not the token was known.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewOAuthTokenRequest().Revoke).
		SendsNothing(http.StatusOK, "done").
		Sends(http.StatusUnauthorized, oauthError, invalidClient)
	oauthProto.POST("/introspect", h.OAuth.AddIntrospect).
		Summary("What a token is (OAuth 2.0)").
		Describe("RFC 7662: whether it is live, whose it is, and what it may do.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewOAuthTokenRequest().Introspect).
		Sends(http.StatusOK, responses.OAuthIntrospectItem{}).
		Sends(http.StatusUnauthorized, oauthError, invalidClient)

	// ---------------------------------------------------------------
	// OIDC userinfo — user-authenticated via JWT middleware.
	// ---------------------------------------------------------------

	oauthUser := api.Group("/oauth", r.authorize)
	oauthUser.GET("/userinfo", h.OAuth.ReadUserInfo).
		Summary("Who the token is for (OpenID Connect)").
		Describe("The claims its scopes allow: `sub` always; with `profile`, `name`, `preferred_username`, `alias`, `avatar_urls`, `id`, `type`, `enabled`, `person_type`, `totp_enabled`, `last_visited_at` and the rest of what the web app reads of the viewer; with `email`, `email` and `email_verified`. With `profile`, also `entitlements`: what the person may do, and how much, where the installation sets a limit; `entitlements.invites.max_pending` is how many invitations may wait at once, and `invites` is null for somebody who may not invite.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeOpenID).
		Sends(http.StatusOK, map[string]any{}).
		Sends(http.StatusUnauthorized, oauthError, "`invalid_token`: no token, or one that has expired").
		Sends(http.StatusForbidden, oauthError, "`insufficient_scope`: the token has no `openid`")

	// ---------------------------------------------------------------
	// OAuth client management (admin only).
	// ---------------------------------------------------------------

	clientsReq := requests.NewOAuthClientRequest()
	clients := api.Group("/clients", r.authorize)
	clients.GET("", h.OAuth.BrowseClients).
		Summary("The apps that may sign people in").
		Describe("Every one, without its secret: a secret is shown once, when it is made.").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeOAuthRead).
		Sends(http.StatusOK, libs_responses.Unpaged[*responses.OAuthClientItem]{})
	clients.GET("/:id", h.OAuth.ReadClient).
		Summary("One app").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeOAuthRead).
		Takes(clientsReq.Read).
		Sends(http.StatusOK, responses.OAuthClientItem{}).
		Refuses(http.StatusNotFound, "there is no such app")
	clients.POST("", h.OAuth.AddClient).
		Summary("Register an app").
		Describe("A confidential one is answered with its secret, this once.").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeOAuthWrite).
		StepUp().
		Takes(clientsReq.Add).
		Sends(http.StatusCreated, responses.OAuthClientCreatedItem{}, "with `client_secret` when it is confidential").
		Sends(http.StatusBadRequest, refusal, "`invalid_scope`: a scope that does not exist; or, without a body, the request is not well formed").
		Sends(http.StatusForbidden, refusal, stepUp)
	clients.PATCH("/:id", h.OAuth.EditClient).
		Summary("Change an app").
		Describe("Everything but its id and secret: a new secret is `rotate-secret`.").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeOAuthWrite).
		StepUp().
		Takes(clientsReq.Edit).
		Sends(http.StatusOK, responses.OAuthClientItem{}).
		Sends(http.StatusBadRequest, refusal, "`invalid_scope`: a scope that does not exist; or, without a body, the request is not well formed").
		Sends(http.StatusForbidden, refusal, stepUp).
		Refuses(http.StatusNotFound, "there is no such app")
	clients.DELETE("/:id", h.OAuth.DeleteClient).
		Summary("Remove an app").
		Describe("Its tokens stop working.").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeOAuthWrite).
		StepUp().
		Takes(clientsReq.Delete).
		Sends(http.StatusOK, responses.OAuthClientItem{}, "the app as it was").
		Sends(http.StatusForbidden, refusal, stepUp).
		Refuses(http.StatusNotFound, "there is no such app")
	clients.POST("/:id/rotate-secret", h.OAuth.RotateClientSecret).
		Summary("Give an app a new secret").
		Describe("The old one stops working, and so do the tokens issued with it.").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeOAuthWrite).
		StepUp().
		Takes(clientsReq.Read).
		Sends(http.StatusOK, responses.OAuthClientCreatedItem{}, "with the new `client_secret`, this once").
		Sends(http.StatusBadRequest, refusal, "`public_client`: the app is public and has no secret; or, without a body, the request is not well formed").
		Sends(http.StatusForbidden, refusal, stepUp).
		Refuses(http.StatusNotFound, "there is no such app")

	// ---------------------------------------------------------------
	// OAuth signing key management (admin only).
	// ---------------------------------------------------------------

	keyForbidden := "`insufficient_scope`: the token has no `oauth:read`; `forbidden`: the viewer is not a super administrator"
	signingKeys := api.Group("/oauth-signing-keys", r.authorize)
	signingKeys.GET("", h.OAuth.BrowseSigningKeys).
		Summary("The keys tokens are signed with").
		Describe("Every one, live or not, newest first.").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeOAuthRead).
		Sends(http.StatusOK, libs_responses.Unpaged[*responses.OAuthSigningKeyItem]{}).
		Sends(http.StatusForbidden, refusal, keyForbidden)
	signingKeys.GET("/:kid", h.OAuth.ReadSigningKey).
		Summary("One signing key").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeOAuthRead).
		Takes(requests.NewOAuthSigningKeyRequest().Read).
		Sends(http.StatusOK, responses.OAuthSigningKeyItem{}).
		Sends(http.StatusForbidden, refusal, keyForbidden).
		Refuses(http.StatusNotFound, "there is no such key")
	signingKeys.PATCH("/:kid", h.OAuth.EditSigningKey).
		Summary("Sign with a key, or stop").
		Describe("A key no longer signed with still checks the tokens it signed.").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeOAuthWrite).
		StepUp().
		Takes(requests.NewOAuthSigningKeyRequest().Edit).
		Sends(http.StatusOK, responses.OAuthSigningKeyItem{}).
		Sends(http.StatusForbidden, refusal, stepUp+"; `insufficient_scope`: the token has no `oauth:write`; `forbidden`: the viewer is not a super administrator").
		Refuses(http.StatusNotFound, "there is no such key").
		Sends(http.StatusConflict, refusal, "`last_active_key`: it is the only key signed with, and nothing could be signed without it")

	// ---------------------------------------------------------------
	// Protected user/account routes — JWT required.
	// ---------------------------------------------------------------

	// Password change. Same /password path as the public forgot/reset
	// routes above, but PATCH is gated by authorize. The split into
	// two groups is intentional: lets us see at a glance which methods
	// require auth and which don't, without method-level middleware
	// branching inside a single group.
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

	// Email change, step one. Authenticated and step-up gated; mails a
	// confirmation link to the person's CURRENT address and changes
	// nothing by itself.
	//
	// A separate group from /email/change above rather than one group
	// with method-level middleware, matching the split used for
	// /password and /invites: it makes which paths require auth legible
	// at a glance. Echo resolves the more specific path first, so
	// /email/change/confirm is unaffected by the authorize on this one.
	emailChangeAuth := api.Group("/email", r.authorize)
	emailChangeAuth.PATCH("/change", h.EmailChange.Edit).
		Summary("Begin changing the viewer's email address").
		Describe("Mails a code to the address the account has now: that inbox approves the move, so a stolen session alone cannot.").
		Viewer(apidoc.SignedIn).
		StepUp().
		Takes(&requests.EmailChangeEdit{}).
		SendsNothing(http.StatusNoContent, "the code is sent, or the address is the one the account has").
		Sends(http.StatusForbidden, refusal, stepUp).
		Refuses(http.StatusConflict, "another account has that address")
	// Step two: the code that was mailed to the current address, typed back
	// into the app. Signed in, like step one. The code proves the inbox, the
	// session proves whose change it is.
	emailChangeAuth.POST("/change/confirm", h.EmailChange.Confirm).
		Summary("Change the viewer's email address").
		Describe("With the code mailed to the old address. Every session is signed out, this one too.").
		Viewer(apidoc.SignedIn).
		Takes(&requests.EmailChangeConfirm{}).
		SendsNothing(http.StatusNoContent, "changed").
		Refuses(http.StatusBadRequest, "it is not six digits; nothing was counted").
		Refuses(http.StatusConflict, "another account has taken the address since").
		Refuses(http.StatusGone, "no change is waiting, or its code ran out, was replaced or had too many wrong tries: start again").
		Sends(http.StatusUnprocessableEntity, handlers.CodeMismatch{}, "the code is wrong; it says how many tries are left")

	// Username change. Authenticated and step-up gated, and unlike the
	// email change it is one step: a handle move does not relocate any
	// recovery path, so there is nothing for the mailbox to approve
	// first. See the note on usernameChangeHandler.
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
	reauth := api.Group("/reauth", r.authorize)
	reauth.POST("/password", h.Reauth.AddPassword).
		Summary("Prove who the viewer is, with their password").
		Describe("And their passcode, when they have two-factor authentication. Good for five minutes, and for one change.").
		Viewer(apidoc.SignedIn).
		Takes(&requests.ReauthPassword{}).
		SendsNothing(http.StatusNoContent, "proved").
		Refuses(http.StatusUnauthorized, "the password is wrong").
		Refuses(http.StatusForbidden, "the passcode is wrong, or missing").
		Refuses(http.StatusTooManyRequests, "too many tries; wait")
	reauth.POST("/passkey/begin", h.Reauth.AddPasskeyBegin).
		Summary("Begin proving who the viewer is, with a passkey").
		Describe("The options to hand to `navigator.credentials.get()`: one of the viewer's own passkeys.").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, json.RawMessage{}, "the options (WebAuthn)").
		Refuses(http.StatusConflict, "the viewer has no passkeys: use the password")
	reauth.POST("/passkey/finish", h.Reauth.AddPasskeyFinish).
		Summary("Prove who the viewer is, with a passkey").
		Describe("What `navigator.credentials.get()` answered, as the browser gives it.").
		Viewer(apidoc.SignedIn).
		TakesBody(json.RawMessage{}).
		SendsNothing(http.StatusNoContent, "proved").
		Sends(http.StatusBadRequest, refusal, "`ceremony_expired`: it was begun too long ago, or not at all").
		Sends(http.StatusUnauthorized, refusal, "`assertion_failed`: the passkey was not accepted; or, without a body, no token")

	authLogs := api.Group("/auth-logs", r.authorize)
	authLogs.GET("/:id", h.AuthLog.Browse).
		Summary("Where somebody is signed in").
		Describe("Each session: the device, roughly where, when it began, whether it is still live, and which is the viewer's own. Their own, or anybody's, for an administrator.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountRead).
		Takes(requests.NewAuthLogRequest().Browse).
		Sends(http.StatusOK, []responses.AuthLogItem{}).
		Refuses(http.StatusForbidden, "it is somebody else, and the viewer is not an administrator").
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
		Refuses(http.StatusForbidden, "it is somebody else's, and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such person, or no such session of theirs")

	// Invite admin operations — same /invites path as the public
	// accept routes above, but Browse, Add, and Delete are gated by
	// authorize. Same split-into-two-groups pattern as /password.
	invitesAuth := api.Group("/invites", r.authorize)
	invitesAuth.GET("", h.Invite.Browse).
		Summary("The invitations the viewer has sent").
		Describe("Their own only, administrators' too: an invitation carries somebody's address. With how many are waiting; how many may is the viewer's entitlement, in userinfo.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeActorsRead).
		Takes(libs_requests.NewBrowse()).
		Sends(http.StatusOK, responses.InviteList{}).
		Refuses(http.StatusForbidden, "the viewer may not invite")
	invitesAuth.POST("", h.Invite.Add).
		Summary("Invite somebody").
		Describe("Mails them a link. Who may invite is the installation's choice (`INVITES_FROM`), and so is how many may wait at once (`INVITE_PENDING_MAX`).").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeActorsWrite).
		Takes(&requests.InviteAdd{}).
		Sends(http.StatusOK, responses.InviteItem{}).
		Sends(http.StatusForbidden, refusal, "`invitations_closed`: the site takes no one new; or, without a body, the viewer may not invite").
		Sends(http.StatusConflict, refusal, "`too_many_invitations`: the viewer has as many waiting as they may, and it says how one frees; or, without a body, the address has an account").
		Refuses(http.StatusTooManyRequests, "too many invitations, from the viewer or to the address, for now")
	invitesAuth.DELETE("/:id", h.Invite.Delete).
		Summary("Take an invitation back").
		Describe("Its link stops working, and the person invited is told.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeActorsWrite).
		Takes(&requests.InviteDelete{}).
		Sends(http.StatusOK, responses.InviteItem{}, "the invitation as it was").
		Refuses(http.StatusForbidden, "it is somebody else's, and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such invitation")

	// The approval queue. Administrator-only, JSON — the screen that consumes
	// this belongs with the other settings work.
	//
	// approve and reject are named in the path rather than being a bare POST
	// and DELETE the way follow requests are answered. Ignoring a follow
	// request is a silent dismissal; rejecting a signup request records a
	// decision note, stamps the deciding administrator and sends mail, and the
	// route says so.
	undecidable := "`already_decided`: it has been answered; `not_verified`: they have not confirmed their address"
	signupRequests := api.Group("/signup-requests", r.authorize)
	signupRequests.GET("", h.SignupRequests.Browse).
		Summary("The requests to join waiting for an answer").
		Describe("Where registration is by approval: oldest first, each with whether the username asked for is still free.").
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
		Sends(http.StatusConflict, refusal, undecidable+"; `username_taken`: somebody has the username now")
	signupRequests.POST("/:id/reject", h.SignupRequests.Reject).
		Summary("Turn down a request to join").
		Describe("They are told by mail; the note is not sent.").
		Viewer(apidoc.Admin).
		Scope(libs_constants.OAuthScopeActorsWrite).
		Takes(&requests.SignupRequestDecision{}).
		Sends(http.StatusOK, handlers.SignupDecided{}).
		Refuses(http.StatusNotFound, "there is no such request").
		Sends(http.StatusConflict, refusal, undecidable)

	// The administration list of people and groups, for site cleanup. Super
	// administrators only — enforced in the handler. Removing what it lists
	// is person-service's and group-service's purge.
	accounts := api.Group("/accounts", r.authorize)
	accounts.GET("", h.Accounts.Browse).
		Summary("People or groups, for cleaning up a site").
		Describe("Least recently active first, by default; purged ones are not listed. Removing them is the purge (`DELETE /people`, `DELETE /groups`, account-lifecycle.md).").
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
		Describe("Every public profile, group and post, for an installation that has set `MIN_CONTENT_ACCESS=registered`. It cannot be undone: the number is typed to confirm it.").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeActorsWrite).
		StepUp().
		Takes(&handlers.MakeRegisteredRequest{}).
		Sends(http.StatusOK, handlers.PublicContent{}, "what was changed").
		Sends(http.StatusBadRequest, handlers.ConfirmationMismatch{}, "`confirmation_mismatch`: the number is not how much is public now, which it says; or, without a body, the request is not well formed").
		Sends(http.StatusForbidden, refusal, stepUp+"; or, without a body, the viewer may not").
		Sends(http.StatusConflict, refusal, "`public_content_allowed`: new content may still be public here")

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
		Refuses(http.StatusForbidden, "it is wrong, or the viewer has no two-factor authentication").
		Refuses(http.StatusNotFound, "the viewer has no account")
	totps.POST("", h.Totp.Add).
		Summary("Begin setting up two-factor authentication").
		Describe("Answers the QR code to pair an authenticator with. It is not on until `/totp/recovery-codes` and `PATCH /totp`.").
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
		Describe("The viewer's own; or, for an administrator, somebody's who has lost their authenticator. They are told by mail.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeActorsWrite).
		StepUp().
		Takes(requests.NewTotpRequest().Delete).
		SendsNothing(http.StatusNoContent, "off, or it was").
		Sends(http.StatusForbidden, refusal, stepUp+"; `forbidden`: the viewer may not act on this account").
		Refuses(http.StatusNotFound, "there is no such person")
	totps.POST("/recovery-codes", h.Totp.AddRecoveryCodes).
		Summary("Get recovery codes").
		Describe("With a passcode from the authenticator just paired, which proves it works. Each code signs in once in place of a passcode. A new set replaces the old, and the owner is told by mail.").
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
		Refuses(http.StatusForbidden, "it is somebody else, and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such person")

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
		Describe("What `navigator.credentials.create()` answered, as the browser gives it. It is named after the device; the owner is told by mail.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountWrite).
		TakesBody(json.RawMessage{}).
		Sends(http.StatusCreated, libs_responses.Item[*responses.WebAuthnCredentialItem]{}).
		Sends(http.StatusBadRequest, refusal, "`ceremony_expired`: it was begun too long ago, or not at all; `registration_failed`: the passkey could not be checked")
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
		Describe("A number only: which devices they hold is theirs. Their own, or anybody's, for an administrator.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountWrite).
		Takes(webAuthnReq.User).
		Sends(http.StatusOK, libs_responses.Item[handlers.PasskeyCount]{}).
		Refuses(http.StatusForbidden, "it is somebody else, and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such person")
	webAuthn.DELETE("/credentials/user/:user_id", h.WebAuthn.DeleteAll).
		Summary("Remove all of somebody's passkeys").
		Describe("Their own, or anybody's, for an administrator, who must have just proved who they are; the owner is told by mail. They still sign in with their password.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeAccountWrite).
		StepUp().
		Takes(webAuthnReq.User).
		Sends(http.StatusOK, libs_responses.Item[handlers.PasskeysRevoked]{}).
		Sends(http.StatusForbidden, refusal, stepUp+"; or, without a body, it is somebody else, and the viewer is not an administrator").
		Refuses(http.StatusNotFound, "there is no such person")

	return doc
}
