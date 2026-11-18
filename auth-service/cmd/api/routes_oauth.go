package main

import (
	"net/http"

	"auth/requests"
	"auth/responses"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
	libs_responses "anahita-libs/responses"
)

// oauthProtocolRoutes registers the OAuth 2.0 endpoints an app calls with its own
// credentials, and OpenID Connect's userinfo.
func (r *Routes) oauthProtocolRoutes(api *apidoc.Router) {
	h := r.handler

	// --- OAuth 2.0 endpoints ---
	// An app proves which it is with its own credentials, not a person's. Public:
	// no middleware, so the caller is never known.

	// invalidClient is the refusal of an app that did not prove which it is, the
	// same on each endpoint.
	invalidClient := "`invalid_client`: the app did not prove which it is"
	oauthProto := api.Group("/oauth")

	oauthProto.POST("/token", h.OAuth.AddToken).
		Summary("Get tokens (OAuth 2.0)").
		Describe("The token endpoint (RFC 6749, 3.2): an authorization code, a refresh token or "+
			"the app's own credentials for an access token. A refresh token is used once, and the "+
			"answer has the next. A refusal of a refresh token says which in `error_code`: "+
			"`refresh_raced` alone is not the end of the session, but another request refreshing "+
			"first.").
		Viewer(apidoc.Anyone).
		Takes(requests.NewOAuthTokenRequest().Token).
		Sends(http.StatusOK, responses.OAuthTokenResponse{}).
		Sends(http.StatusBadRequest, oauthError, "`invalid_grant`: the code or refresh token is "+
			"wrong, used or expired; `unsupported_grant_type`: the app may not use this grant; "+
			"`invalid_scope`: it asked for a scope it does not have; or, without a body, the "+
			"request is not well formed").
		Sends(http.StatusUnauthorized, oauthError, invalidClient).
		Sends(http.StatusForbidden, oauthError,
			"`unauthorized_client`: the app may not act for itself")

	oauthProto.POST("/revoke", h.OAuth.AddRevoke).
		Summary("Revoke a token (OAuth 2.0)").
		Describe("RFC 7009. A refresh token is revoked; an access token is short-lived and runs "+
			"out by itself. The answer is the same whether or not the token was known.").
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

	// --- OpenID Connect userinfo ---
	// Who an access token is for. Every route needs a token (r.authorize). It is
	// on /oauth like the endpoints above, in a group of its own, because it is the
	// only one there that needs a token.
	oauthUser := api.Group("/oauth", r.authorize)

	oauthUser.GET("/userinfo", h.OAuth.ReadUserInfo).
		Summary("Who the token is for (OpenID Connect)").
		Describe("The claims its scopes allow: `sub` always; with `profile`, `name`, "+
			"`preferred_username`, `alias`, `avatar_urls`, `id`, `type`, `enabled`, "+
			"`person_type`, `totp_enabled`, `last_visited_at` and the rest of what the web app "+
			"reads of the viewer; with `email`, `email` and `email_verified`. With `profile`, "+
			"also `entitlements`: what the person may do, and how much, where the installation "+
			"sets a limit; `entitlements.invites.max_pending` is how many invitations may wait at "+
			"once, and `invites` is null for somebody who may not invite.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeOpenID).
		Sends(http.StatusOK, map[string]any{}).
		Sends(http.StatusUnauthorized, oauthError,
			"`invalid_token`: no token, or one that has expired").
		Sends(http.StatusForbidden, oauthError, "`insufficient_scope`: the token has no `openid`")
}

// oauthAdminRoutes registers the apps that may sign people in, and the keys tokens
// are signed with.
func (r *Routes) oauthAdminRoutes(api *apidoc.Router) {
	h := r.handler

	// --- OAuth apps ---
	// Every route needs a token (r.authorize). Only a super administrator may use
	// them, which the handlers check.

	// clientsReq holds what the routes on one app take.
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
		Sends(http.StatusCreated, responses.OAuthClientCreatedItem{},
			"with `client_secret` when it is confidential").
		Sends(http.StatusBadRequest, refusal, "`invalid_scope`: a scope that does not exist; or, "+
			"without a body, the request is not well formed").
		Sends(http.StatusForbidden, refusal, stepUp)

	clients.PATCH("/:id", h.OAuth.EditClient).
		Summary("Change an app").
		Describe("Everything but its id and secret: a new secret is `rotate-secret`.").
		Viewer(apidoc.SuperAdmin).
		Scope(libs_constants.OAuthScopeOAuthWrite).
		StepUp().
		Takes(clientsReq.Edit).
		Sends(http.StatusOK, responses.OAuthClientItem{}).
		Sends(http.StatusBadRequest, refusal, "`invalid_scope`: a scope that does not exist; or, "+
			"without a body, the request is not well formed").
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
		Sends(http.StatusOK, responses.OAuthClientCreatedItem{},
			"with the new `client_secret`, this once").
		Sends(http.StatusBadRequest, refusal, "`public_client`: the app is public and has no "+
			"secret; or, without a body, the request is not well formed").
		Sends(http.StatusForbidden, refusal, stepUp).
		Refuses(http.StatusNotFound, "there is no such app")

	// --- Signing keys ---
	// Every route needs a token (r.authorize). Only a super administrator may use
	// them, which the handlers check.

	// keyForbidden is the refusal the two routes that read keys share.
	keyForbidden := "`insufficient_scope`: the token has no `oauth:read`; `forbidden`: the viewer " +
		"is not a super administrator"
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
		Sends(http.StatusForbidden, refusal, stepUp+"; `insufficient_scope`: the token has no "+
			"`oauth:write`; `forbidden`: the viewer is not a super administrator").
		Refuses(http.StatusNotFound, "there is no such key").
		Sends(http.StatusConflict, refusal, "`last_active_key`: it is the only key signed with, "+
			"and nothing could be signed without it")
}
