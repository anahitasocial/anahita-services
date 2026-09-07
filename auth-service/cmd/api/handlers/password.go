package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"auth/constants"
	"auth/data/entities"
	"auth/requests"

	"github.com/labstack/echo/v4"
)

type passwordHandler struct {
	*Env
}

func newPasswordHandler(env *Env) *passwordHandler {
	return &passwordHandler{env}
}

// ---------------------------------------------------------------------
// Forgot-password flow (unauthenticated, server-rendered)
// ---------------------------------------------------------------------

// ForgotForm renders the "enter your email" form.
//
//	GET /password/forgot
func (h *passwordHandler) ForgotForm(c echo.Context) error {
	return h.renderTemplate(c, "password_forgot.html", map[string]any{})
}

// Forgot processes the forgot-password submission. Always renders the
// same confirmation page regardless of whether the email exists, to
// prevent user enumeration. Token creation and email sending happen
// only if a matching user is found, and the email send is fire-and-
// forget.
//
//	POST /password/forgot
//
// Throttled per target email and per source IP. Unauthenticated and it
// sends mail, so without a limit it is a way to flood someone's inbox
// and burn our sending reputation. Both counters are checked before the
// lookup, and a throttled request renders the identical page — a
// visibly different response would turn the limiter into the
// enumeration oracle the rest of this handler avoids.
func (h *passwordHandler) Forgot(c echo.Context) error {
	req := &requests.PasswordForgot{}
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return h.renderTemplate(c, "password_forgot.html", map[string]any{
			"Error": "Please enter a valid email address.",
		})
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return h.renderTemplate(c, "password_forgot.html", map[string]any{
			"Error": "Please enter a valid email address.",
			"Email": req.Email,
		})
	}

	// Throttle before the lookup. Both counters advance even when the
	// address matches nothing, so the cost of probing is the same
	// either way.
	//
	// The || short-circuits, so the IP counter stops advancing once the
	// per-address limit trips. That is intended, not a bug to tidy: the
	// two limits catch different abuse. Hammering ONE address is caught
	// by the email counter, and the IP counter has nothing to add.
	// Spraying MANY addresses passes each fresh email check, so the IP
	// counter is reached and increments every time — which is the case
	// it exists for.
	throttled := h.rateLimitExceeded(rateLimitScopePasswordEmail, req.Email) ||
		h.rateLimitExceeded(rateLimitScopePasswordIP, getClientIP(c))

	// Best-effort lookup. The RESPONSE is the same either way, so the
	// caller cannot distinguish registered emails from unregistered ones —
	// but the error is logged rather than dropped.
	//
	// Dropping it hid a real outage: a NULL in any of several nullable
	// columns failed the scan, this read returned an error, and the person
	// got "check your email" for a message that was never going to arrive.
	// Nothing anywhere recorded that it had happened. sql.ErrNoRows is the
	// ordinary case and stays quiet; anything else is a fault worth seeing.
	//
	// The address is not logged. This is an unauthenticated endpoint and
	// anyone can name any address at it, so a log of them is a log of
	// guesses, not of members.
	user, err := h.Repos.Users.ReadByEmail(req.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Println("password forgot: user lookup failed:", err)
	}

	if user != nil && !throttled {
		token := entities.NewToken()
		token.Status = constants.TokenStatusPending
		token.ExpiredAfter = 15 * 60
		token.Type = constants.TokenTypePasswordReset
		token.RecipientEmail = user.Email
		token.IsUsed = false
		token.CreatedAt = time.Now()
		token.CreatedBy = user.ID
		token.UpdatedAt = time.Now()
		token.UpdatedBy = user.ID

		if err := token.GenerateToken(); err != nil {
			log.Println(err)
		} else if _, err := h.Repos.Tokens.Add(token); err != nil {
			log.Println(err)
		} else {
			go func() {
				if err := h.Notif.Password.EmailToken(user, token); err != nil {
					log.Println(err)
				}
			}()
		}
	}

	return h.renderTemplate(c, "password_forgot_sent.html", map[string]any{
		"Email": req.Email,
	})
}

// ---------------------------------------------------------------------
// Reset-password flow (unauthenticated, token-gated, server-rendered)
// ---------------------------------------------------------------------

// ResetForm validates the reset token from the URL and renders the new
// password form. Invalid, expired, or already-used tokens get a
// friendly error page instead of the form.
//
//	GET /password/reset?token=...
func (h *passwordHandler) ResetForm(c echo.Context) error {
	tokenBody := c.QueryParam("token")
	if tokenBody == "" {
		return h.renderError(c, "Invalid Link",
			"This password reset link is not valid. Please request a new one.")
	}

	token, err := h.Repos.Tokens.ReadByBody(tokenBody)
	if err != nil {
		log.Println(err)
		return h.renderError(c, "Invalid Link",
			"This password reset link is not valid. Please request a new one.")
	}

	if token.Type != constants.TokenTypePasswordReset {
		return h.renderError(c, "Invalid Link",
			"This link is not valid for resetting your password.")
	}

	switch token.Status {
	case constants.TokenStatusUsed:
		return h.renderError(c, "Link Already Used",
			"This password reset link has already been used. Please request a new one if you still need to reset your password.")
	case constants.TokenStatusExpired:
		return h.renderError(c, "Link Expired",
			"This password reset link has expired. Please request a new one to continue.")
	}

	return h.renderTemplate(c, "password_reset.html", map[string]any{
		"Token": token.Body,
	})
}

// Reset processes the new password submission.
//
//	POST /password/reset
//
// On success: the password is updated, the token is consumed, the
// user's passkeys and active sessions are revoked, a notification
// email is sent, and a success page is rendered with a link to
// /oauth/login.
//
// The revocations are the part worth explaining. Changing a password
// does nothing to credentials that were never derived from it, so an
// attacker holding either a passkey they enrolled or a refresh token
// from a session they were already in survives the reset untouched —
// the account reads as recovered while still being open. Since this
// flow exists to take an account back, both have to go with it.
//
// Auto-login is intentionally NOT performed here. The login flow
// enforces TOTP and account-disabled checks — silently bypassing
// those after a password reset would be a security downgrade,
// especially for users with TOTP enabled. Sending the user through
// /oauth/login confirms the new password works and exercises the
// second factor where applicable.
func (h *passwordHandler) Reset(c echo.Context) error {
	req := &requests.PasswordReset{}
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return h.renderTemplate(c, "password_reset.html", map[string]any{
			"Token": req.Token,
			"Error": "Password must be at least 15 characters long.",
		})
	}

	token, err := h.Repos.Tokens.ReadByBody(req.Token)
	if err != nil {
		log.Println(err)
		return h.renderError(c, "Invalid Link",
			"This password reset link is not valid. Please request a new one.")
	}

	if token.Type != constants.TokenTypePasswordReset {
		return h.renderError(c, "Invalid Link",
			"This link is not valid for resetting your password.")
	}

	if token.Status != constants.TokenStatusPending {
		return h.renderError(c, "Link No Longer Valid",
			"This password reset link has been used or has expired. Please request a new one.")
	}

	user, err := h.Repos.Users.ReadByEmail(token.RecipientEmail)
	if err != nil || user == nil {
		log.Println(err)
		return h.renderError(c, "Account Not Found",
			"We could not find an account associated with this reset link.")
	}

	// The new password has to actually be new. This flow exists to take
	// an account back, and it acts like it: below, every passkey and
	// every session goes, and the user is mailed a confirmation that
	// their password was reset. Landing on the same value performs that
	// whole ritual while leaving the credential the reset was for
	// exactly where it was — the account reads as recovered while the
	// password an attacker may know still opens it.
	//
	// The token is deliberately NOT consumed here. IsUsed is set only
	// after a successful update, so the person corrects what they typed
	// and resubmits on the same link rather than going back for another
	// email.
	//
	// That is also what makes the throttle part of this check rather
	// than a nicety bolted on beside it. This comparison is the only
	// thing on the unauthenticated side that can answer "is this the
	// current password?", and a rejected attempt leaves the link live —
	// so without a counter, whoever holds the token gets an unmetered
	// password oracle for the token's fifteen minutes, at one bcrypt
	// per guess. They already hold the mailbox at that point and could
	// simply take the account; what is being denied them is walking off
	// with the plaintext to try somewhere else.
	//
	// Keyed on the token, not the email, and that distinction is the
	// whole reason this does not hurt anybody: a fresh link starts with
	// a fresh budget, while issuing fresh links is itself already
	// capped by rateLimitScopePasswordEmail. Keying on the email would
	// instead let a probe lock the owner out of resetting at all.
	//
	// Only this branch increments. An ordinary form error is not a
	// probe and should not spend from the same budget.
	if user.CheckPassword(req.Password) {
		if h.rateLimitExceeded(rateLimitScopePasswordReset, req.Token) {
			return h.renderError(c, "Too Many Attempts",
				"There have been too many attempts on this link. Please request a new password reset email.")
		}

		return h.renderTemplate(c, "password_reset.html", map[string]any{
			"Token": req.Token,
			"Error": "Your new password must be different from your current password.",
		})
	}

	if err := user.SetPassword(req.Password); err != nil {
		log.Println(err)
		return h.renderTemplate(c, "password_reset.html", map[string]any{
			"Token": req.Token,
			"Error": "Could not update your password. Please try again.",
		})
	}

	if err := h.Repos.Users.EditPassword(user.Password, user.ID); err != nil {
		log.Println(err)
		return h.renderTemplate(c, "password_reset.html", map[string]any{
			"Token": req.Token,
			"Error": "Could not update your password. Please try again.",
		})
	}

	// Consume the token. If this fails, the password is already
	// updated — log but don't block the user. Subsequent attempts
	// will be caught by the status gate on the next read.
	token.IsUsed = true
	token.UpdatedAt = time.Now()
	token.UpdatedBy = user.ID
	if _, err := h.Repos.Tokens.Edit(token); err != nil {
		log.Println(err)
	}

	// --- Revoke what a password change does not touch -------------
	//
	// Both of the following exist because changing a password does
	// nothing to credentials that were never derived from it. An
	// attacker who got into this account may hold either, and if they
	// survive the reset then the account reads as recovered while
	// still being open.
	//
	// Both are best-effort. The password change has already
	// succeeded, and failing the reset here would leave the person
	// unable to recover at all — worse than the case being guarded
	// against. Both run before the confirmation email, which tells
	// the person this happened.

	// Passkeys. Enrolled by an attacker, one survives a password
	// change untouched and lets them straight back in.
	if revokedKeys, err := h.Repos.WebAuthnCredentials.DeleteByPersonID(user.ID); err != nil {
		log.Println("password reset: passkey revocation failed:", err)
	} else if revokedKeys > 0 {
		log.Printf("password reset: revoked %d passkey(s) (person=%d)",
			revokedKeys, user.ID)
	}

	// Active sessions. A refresh token is independent of the
	// password, so an attacker who was simply signed in when the
	// reset happened keeps working without having enrolled anything.
	//
	// This clears the BFF session; the IdP cookie is handled next.
	// Both are needed — either alone leaves a way back in.
	authlogHandler := newAuthLogHandler(h.Env)
	if revokedSessions, err := authlogHandler.DeleteAllSessions(user); err != nil {
		log.Println("password reset: session revocation failed:", err)
	} else if revokedSessions > 0 {
		log.Printf("password reset: revoked %d session(s) (person=%d)",
			revokedSessions, user.ID)
	}

	// Revoke IdP sessions. Refresh-token revocation above clears the
	// BFF session, but the person_session cookie on the auth host
	// survives it — /oauth/authorize would honour that cookie and
	// mint a fresh token pair without prompting, putting an attacker
	// straight back in.
	oauthHandler := newOAuthHandler(h.Env)
	if revokedIdP, err := oauthHandler.revokeAllIdPSessions(user); err != nil {
		log.Println("password reset: IdP session revocation failed:", err)
	} else if revokedIdP > 0 {
		log.Printf("password reset: revoked %d IdP session(s) (person=%d)",
			revokedIdP, user.ID)
	}

	// Fire-and-forget reset-confirmation email. Last, so it describes
	// work that has already happened rather than work that was
	// intended.
	go func() {
		if err := h.Notif.Password.EmailResetSuccess(user); err != nil {
			log.Println(err)
		}
	}()

	return h.renderTemplate(c, "password_reset_success.html", map[string]any{})
}

// ---------------------------------------------------------------------
// Change-password (authenticated, JSON API)
// ---------------------------------------------------------------------

// Edit processes a password change for an authenticated user.
//
// JSON-only, called by anahita-web from the settings page.
// Auth is enforced upstream by the standard Authorize middleware
// (JWT bearer or X-OAuth-Token from the BFF). The viewer is read
// from the echo context the same way every other authenticated
// handler does it.
//
// Gated on a recent step-up rather than a current-password field. The
// session alone is still not sufficient proof for a credential
// mutation — a stolen laptop with an unlocked browser is a session —
// but the proof now comes from /reauth/*, which accepts a passkey,
// or a password plus a passcode, or a password.
//
// That is what lets a passkey user change their password. Requiring
// the old one kept exactly those people out: someone who signs in by
// passkey daily may not recall it, and their only route was
// forgot-password, which deletes every passkey on the account.
//
//	PATCH /password/change
//
// Status codes:
//
//	200 — success, with the count of other sessions signed out
//	400 — bad request body or validation failure
//	403 — no recent step-up; the client re-authenticates and retries
//	409 — the new password is the current one
//	429 — too many attempts
//	500 — store, database, or hashing failure
func (h *passwordHandler) Edit(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || viewer.ID == 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	subject := fmt.Sprintf("%d", viewer.ID)
	if h.rateLimitExceeded(rateLimitScopePasswordEdit, subject) {
		return c.NoContent(http.StatusTooManyRequests)
	}

	req := &requests.PasswordEdit{}
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	// Re-read the user rather than trusting the cached viewer —
	// matches the posture the login handler takes when re-fetching
	// before TOTP verification. The viewer carries no password hash
	// anyway, so this read is also what makes the comparison below
	// possible at all.
	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || user == nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.requireStepUp(user.ID); err != nil {
		log.Println("password change:", err)
		return c.NoContent(reauthStatus(err))
	}

	// The new password has to actually be new. Below, this endpoint
	// revokes every other session and mails "your password has been
	// updated" — someone rotating a password they believe is
	// compromised gets that entire reassurance while the compromised
	// value stays exactly where it was.
	//
	// AFTER the step-up gate, and that ordering is load-bearing rather
	// than tidiness. This is the only comparison against the stored
	// hash anywhere in this flow, so ahead of the gate it turns
	// /password/change into a password-confirmation oracle for anyone
	// merely holding a session — and a stolen laptop with an unlocked
	// browser is a session. Behind it, the caller has already proven a
	// passkey, or the password itself, at /reauth/*.
	//
	// A rejected attempt still costs one of the five in the window:
	// rateLimitReset and clearStepUp run only on success, further down.
	// Moving this guard below either of them would quietly hand back
	// the probe budget it is meant to spend.
	//
	// 409 rather than 400: the payload is well-formed, it conflicts
	// with stored state.
	if user.CheckPassword(req.Password) {
		log.Println("password change: new password matches current")
		return c.NoContent(http.StatusConflict)
	}

	if err := user.SetPassword(req.Password); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.Repos.Users.EditPassword(user.Password, user.ID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// One step-up authorises one credential change.
	h.clearStepUp(user.ID)
	h.rateLimitReset(rateLimitScopePasswordEdit, subject)

	// --- Revoke every OTHER session ---------------------------------
	//
	// Reset does this too, but Edit is the endpoint someone reaches for
	// when they think their account is compromised — leaving it as the
	// one that revokes nothing had the asymmetry exactly backwards.
	//
	// The acting session is preserved: this is an authenticated request
	// and signing the person out of the tab they are working in is
	// gratuitous. Both helpers take the identifiers that pin it.
	//
	// PASSKEYS ARE DELIBERATELY NOT REVOKED, unlike Reset. The person
	// has a per-passkey delete control on this same page, and wiping
	// them here would recreate the very trap this flow exists to close:
	// a passkey holder changing their password would lose the
	// credential they use to sign in.
	//
	// Best-effort throughout: the password has already changed, and
	// failing now would tell the person it did not.
	revoked := 0

	authlogHandler := newAuthLogHandler(h.Env)
	if count, err := authlogHandler.DeleteAllSessionsExcept(user, viewer.OAuthRefreshTokenID); err != nil {
		log.Println("password change: session revocation failed:", err)
	} else {
		revoked = count
	}

	oauthHandler := newOAuthHandler(h.Env)
	if _, err := oauthHandler.revokeIdPSessionsExcept(c, user); err != nil {
		log.Println("password change: IdP session revocation failed:", err)
	}

	// Fire-and-forget update-confirmation email. Different threat
	// model than Reset — an unexpected update means session access,
	// not email access, so the recovery action in the email is
	// "sign in" rather than "request a fresh reset."
	go func() {
		if err := h.Notif.Password.EmailUpdateSuccess(user, revoked); err != nil {
			log.Println(err)
		}
	}()

	// A body rather than 204 so the UI can say how many sessions ended.
	// "You have been signed out everywhere else" is only reassuring if
	// the person can see it happened.
	return c.JSON(http.StatusOK, map[string]int{
		"revoked_sessions": revoked,
	})
}
