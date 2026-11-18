package main

import (
	"encoding/json"
	"net/http"

	"auth/cmd/api/handlers"
	"auth/requests"

	"anahita-libs/apidoc"
)

// signInRoutes registers signing in and out: the pages a browser is sent to, and
// the session an app's own server keeps.
func (r *Routes) signInRoutes(api *apidoc.Router) {
	h := r.handler

	// --- Signing in ---
	// Pages a browser is sent to (HTML flows). Public: no middleware, so the
	// caller is never known from a token. Each reads the IdP session cookie or the
	// oauth_tokens cookie itself, because it comes before there is a token in the
	// flow (login, authorize, consent), or it is what issues and checks one
	// (session, validate, logout).
	//
	// Signing in with a password is two steps for an account with TOTP: POST
	// /login checks the password and redirects to GET /login/totp for the
	// passcode. An account without TOTP has its IdP session finished by POST
	// /login itself.
	//
	// The passkey routes are another way, sufficient on its own, to the same IdP
	// session. They need no token for the same reason POST /login needs none: they
	// are how a person proves who they are. No TOTP step follows: user
	// verification is Required in the RP config, so the biometric or PIN gesture
	// already supplied the second factor. See data/services/webauthn.go.

	// signedIn is where a browser goes once it is signed in, whichever way it
	// signed in.
	signedIn := "on to where the person was going: back to the authorization that sent them to " +
		"sign in, or to the web app"
	oauth := api.Group("/oauth")

	oauth.GET("/login", h.OAuth.ReadLogin).
		With(form("The sign-in page", "")).
		Takes(&handlers.LoginPageRequest{}).
		SendsNothing(http.StatusFound, "on, when the browser is signed in already")

	oauth.POST("/login", h.OAuth.AddCredentials).
		Summary("Sign in with a password").
		Describe("For an account with two-factor authentication, on to the page for the passcode. "+
			"For one being deleted, the page that offers to restore it.").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Takes(requests.NewOAuthLoginRequest().Login).
		Unbound().
		Sends(http.StatusOK, page, "the page again, saying what was wrong, or the page that "+
			"offers to restore the account").
		SendsNothing(http.StatusFound, signedIn+"; or on to the page for the passcode")

	// Signing in with an emailed code in place of the password. Public for
	// the same reason the password form is. See oauth_login_code.go.
	oauth.GET("/login/code", h.OAuth.ReadLoginCode).
		With(form("The page that asks who is signing in with a code", "")).
		Takes(&handlers.LoginCodePageRequest{})

	oauth.POST("/login/code", h.OAuth.AddLoginCode).
		With(submit("Say who is signing in with a code", "An email address or a username.",
			&requests.OAuthLoginCode{},
			"on to choose where the code goes, or back when it was not given"))

	oauth.GET("/login/code/method", h.OAuth.ReadLoginCodeMethod).
		Summary("The page that asks where the code should go").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Sends(http.StatusOK, page, "the page").
		SendsNothing(http.StatusFound,
			"back to the start, or on, when there is only one place to send it")

	oauth.POST("/login/code/method", h.OAuth.AddLoginCodeMethod).
		With(submit("Send the code",
			"The answer is the same whether or not there is such an account.",
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
		Sends(http.StatusOK, page,
			"the page again, saying what was wrong and how many tries are left").
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
		SendsNothing(http.StatusFound, signedIn+"; or back to the sign-in page, when the first "+
			"step ran out or there were too many wrong passcodes")

	oauth.POST("/login/passkey/begin", h.OAuth.AddPasskeyLoginBegin).
		Summary("Begin signing in with a passkey").
		Describe("The options to hand to `navigator.credentials.get()`. The browser offers "+
			"whichever of its passkeys are for this site.").
		Viewer(apidoc.Anyone).
		Sends(http.StatusOK, json.RawMessage{}, "the options (WebAuthn)")

	oauth.POST("/login/passkey/finish", h.OAuth.AddPasskeyLoginFinish).
		Summary("Sign in with a passkey").
		Describe("What `navigator.credentials.get()` answered, as the browser gives it. No "+
			"passcode follows: the gesture that unlocked the passkey is the second factor.").
		Viewer(apidoc.Anyone).
		TakesBody(json.RawMessage{}).
		Sends(http.StatusOK, handlers.PasskeySignedIn{}).
		Sends(http.StatusBadRequest, refusal,
			"`ceremony_expired`: it was begun too long ago, or not at all").
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
		SendsNothing(http.StatusFound,
			"back to the sign-in page, when no password was just given for such an account")

	oauth.POST("/login/restore", h.OAuth.AddRestore).
		Summary("Restore an account being deleted").
		Describe("Then the person signs in again.").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Sends(http.StatusOK, page, "a page that says it was restored, or why not").
		SendsNothing(http.StatusFound,
			"back to the sign-in page, when no password was just given for such an account")

	// --- Authorizing an app ---
	// The OAuth 2.0 authorization endpoint, and the page that asks whether to
	// allow the app. Public: no middleware, so the caller is never known. Who is
	// asking is the IdP session cookie.
	oauth.GET("/authorize", h.OAuth.ReadAuthorize).
		Summary("Ask to act for somebody (OAuth 2.0)").
		Describe("The authorization endpoint (RFC 6749, 4.1.1), with PKCE (RFC 7636). The person "+
			"signs in first if they have to, and is asked whether to allow it unless the app is "+
			"the installation's own.").
		Viewer(apidoc.Anyone).
		Media("text/html").
		Takes(requests.NewOAuthAuthorizeRequest().Authorize).
		Sends(http.StatusOK, page,
			"a page that says why it cannot go on: no such app, or a redirect address it has not "+
				"registered; or, for an app on a device, the page that hands it the code").
		SendsNothing(http.StatusFound,
			"back to the app's redirect address with `code` and `state`, or with `error` and "+
				"`error_description`; or on to sign in, or to the consent page")

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
		Sends(http.StatusOK, page, "a page that says why it cannot go on; or, for an app on a "+
			"device, the page that hands it the code").
		SendsNothing(http.StatusFound, "back to the app's redirect address with `code` and "+
			"`state`, or with `error=access_denied`; or on to sign in")

	// --- An app's session ---
	// For an app's own server and the gateway, the BFF endpoints (oauth.md): the
	// session that holds an app's tokens, so that they are never sent to the
	// browser. Public: no middleware, so the caller is never known.
	oauth.POST("/session", h.OAuth.AddSession).
		Summary("Start an app's session").
		Describe("For an app's own server (oauth.md): the authorization code is exchanged as "+
			"`/oauth/token` would, and the tokens are kept in the session, not sent.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewOAuthTokenRequest().Token).
		Sends(http.StatusOK, handlers.SessionStarted{}).
		Sends(http.StatusBadRequest, oauthError, "`invalid_grant`: the code is wrong, used or "+
			"expired; or, without a body, the request is not well formed").
		Sends(http.StatusUnauthorized, oauthError,
			"`invalid_client`: the app did not prove which it is")

	oauth.GET("/validate", h.OAuth.ReadValidate).
		Summary("The access token of the session").
		Describe("For the gateway, on every request: the token is in the `X-OAuth-Token` header "+
			"of the answer, refreshed when it had expired, and missing when there is none. Always "+
			"200.").
		Viewer(apidoc.Anyone).
		SendsNothing(http.StatusOK, "the token, in a header, or none")

	oauth.GET("/logout", h.OAuth.ReadLogout).
		Summary("Sign out").
		Describe("Navigated to, not called: it signs out the app's session, then the sign-in "+
			"session, each on its own host, and ends at `return_to`.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.LogoutRequest{}).
		Unbound().
		SendsNothing(http.StatusFound, "on to the next host, or to `return_to`")
}
