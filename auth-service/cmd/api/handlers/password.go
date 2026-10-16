package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"auth/constants"
	"auth/data/entities"
	"auth/requests"

	"github.com/labstack/echo/v4"
)

// passwordResetTokenLength is the hex length of a reset token body: 64 random
// bytes, hex-encoded. Mirrors Token.GenerateToken and the `len=128` tag on
// PasswordReset.Token — if any of the three moves, all three must.
const passwordResetTokenLength = 128

type passwordHandler struct {
	*Env
}

func newPasswordHandler(env *Env) *passwordHandler {
	return &passwordHandler{env}
}

// ---------------------------------------------------------------------
// Forgot-password flow (unauthenticated, server-rendered)
// ---------------------------------------------------------------------

// ---------------------------------------------------------------------
// Reset-password flow (unauthenticated, token-gated, server-rendered)
// ---------------------------------------------------------------------

// resetFormError re-renders the reset form with a message.
func (h *passwordHandler) resetFormError(
	c echo.Context, tokenBody string, _ *entities.User, message string,
) error {
	return h.renderTemplate(c, "password_reset.html", map[string]any{
		"Token": tokenBody,
		"Error": message,
	})
}

// ResetForm validates the reset token from the URL and renders the new
// password form. Invalid, expired, or already-used tokens get a
// friendly error page instead of the form.
//
//	GET /password/reset?token=...
func (h *passwordHandler) ResetForm(c echo.Context) error {
	tokenBody := c.QueryParam("token")

	// No token in the address: the person came through the emailed code,
	// and what lets them in is in their session. The link form is kept for
	// the first super administrator's set-password link, which is read from
	// a log or a message with no page open to type a code into.
	if tokenBody == "" {
		return h.resetFormAfterCode(c)
	}
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

	// Whether the account is on a deletion countdown, so the page can say so.
	//
	// Somebody resetting their password to get back in DESERVES to know there
	// is a clock running, and this is the one screen they are certain to see.
	// It is also the moment it is most useful: the restore happens at the next
	// sign-in, which is exactly where this page sends them.
	//
	// No enumeration risk. Reaching here requires a live token from the
	// account's own mailbox, so the reader has already proved they are the
	// person entitled to know.
	//
	// Best-effort: a lookup failure costs the notice, not the reset.
	data := map[string]any{"Token": token.Body}
	if user, err := h.Repos.Users.ReadByEmail(token.RecipientEmail); err == nil && user != nil {
		if user.DeletedAt != nil {
			data["ScheduledForDeletion"] = true
			data["PurgeDate"] = user.DeletedAt.
				AddDate(0, 0, constants.AccountGraceDays).
				Format("2 January 2006")
		}
	}

	return h.renderTemplate(c, "password_reset.html", data)
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

	// No token in the form: the person came through the emailed code.
	if req.Token == "" {
		return h.resetAfterCode(c, req)
	}
	// The token first, and separately.
	//
	// This form validates two fields and rendered one message, so a truncated
	// or mangled link reported "Password must be at least 15 characters long"
	// — sending somebody back to correct the only part of the submission that
	// was already right.
	//
	// Checked here rather than by inspecting the validation error, because the
	// shared RequestValidator wraps failures as
	// echo.NewHTTPError(400, err.Error()) — a STRING. The typed
	// validator.ValidationErrors is gone by the time a handler sees it, so
	// there is nothing to ask which field failed. Fixing that is a change to
	// every service's error responses; this is one handler's problem and this
	// is where it is cheap to solve.
	//
	// A bad token also means the form cannot work, so this sends them for a
	// fresh link rather than re-rendering a page whose hidden field is the
	// fault.
	if len(req.Token) != passwordResetTokenLength {
		log.Printf("password reset: token is %d characters, expected %d",
			len(req.Token), passwordResetTokenLength)
		return h.renderError(c, "Invalid Link",
			"This password reset link is not valid. Please request a new one.")
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		// The account has not been looked up yet, so the SUBMISSION is what
		// says whether the form carried the agreement checkboxes: it sends a
		// version only when it did. Dropping them here would hand back a form
		// missing the fields the next submission needs.
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

	return h.applyReset(c, user, req.Password, req.Token, req.Token, func() {
		// Consume the token. If this fails, the password is already
		// updated — log but don't block the user. Subsequent attempts
		// will be caught by the status gate on the next read.
		token.IsUsed = true
		token.UpdatedAt = time.Now()
		token.UpdatedBy = user.ID
		if _, err := h.Repos.Tokens.Edit(token); err != nil {
			log.Println(err)
		}
	})
}

// applyReset sets the new password and revokes what must not survive it. It
// is the same whether the person proved themselves with a link or a code.
//
// formToken goes back into the form when it is shown again, and is empty on
// the code path. probeKey is what the "is this my current password" guesses
// are counted against. used marks the link or the code spent.
func (h *passwordHandler) applyReset(
	c echo.Context, user *entities.User, password, formToken, probeKey string, used func(),
) error {
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
	if user.CheckPassword(password) {
		if h.rateLimitExceeded(rateLimitScopePasswordReset, probeKey) {
			return h.renderError(c, "Too Many Attempts",
				"There have been too many attempts on this link. Please request a new password reset email.")
		}

		return h.resetFormError(c, formToken, user,
			"Your new password must be different from your current password.")
	}

	if err := user.SetPassword(password); err != nil {
		log.Println(err)
		return h.resetFormError(c, formToken, user,
			"Could not update your password. Please try again.")
	}

	if err := h.Repos.Users.EditPassword(user.Password, user.ID); err != nil {
		log.Println(err)
		return h.resetFormError(c, formToken, user,
			"Could not update your password. Please try again.")
	}

	// The grant, whichever kind it was, is spent.
	used()

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
	authLogHandler := newAuthLogHandler(h.Env)
	if revokedSessions, err := authLogHandler.DeleteAllSessions(user); err != nil {
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

	// Tell them here too. This is the page with the sign-in button on it, and
	// signing in is what restores the account — so the notice and the action
	// it calls for are in the same place.
	successData := map[string]any{}
	if user.DeletedAt != nil {
		successData["ScheduledForDeletion"] = true
		successData["PurgeDate"] = user.DeletedAt.
			AddDate(0, 0, constants.AccountGraceDays).
			Format("2 January 2006")
	}

	return h.renderTemplate(c, "password_reset_success.html", successData)
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
//	403 — `step_up_required`: no recent step-up; the client re-authenticates and retries
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
		return stepUpRequired(c, err)
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

	authLogHandler := newAuthLogHandler(h.Env)
	if count, err := authLogHandler.DeleteAllSessionsExcept(user, viewer.OAuthRefreshTokenID); err != nil {
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
	return c.JSON(http.StatusOK, &PasswordChanged{RevokedSessions: revoked})
}

// PasswordChanged says how many other sessions were signed out.
type PasswordChanged struct {
	// Every one but the one that changed it.
	RevokedSessions int `json:"revoked_sessions"`
}
