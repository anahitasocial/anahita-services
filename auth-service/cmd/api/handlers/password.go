package handlers

import (
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
// only if a matching person is found, and the email send is fire-and-
// forget.
//
//	POST /password/forgot
//
// TODO: add rate limiting (per IP and per email) to prevent abuse.
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

	// Best-effort lookup. Any error (including not-found) is swallowed
	// so the caller cannot distinguish registered emails from
	// unregistered ones.
	user, _ := h.Repos.Users.ReadByEmail(req.Email)

	if user != nil {
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
// person's passkeys and active sessions are revoked, a notification
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
// those after a password reset would be a
// security downgrade, especially for users with TOTP enabled. Sending
// the user through /oauth/login confirms the new password works and
// exercises the second factor where applicable.
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
// JSON-only, called by the Anahita web app from the settings page.
// Auth is enforced upstream by the standard Authorize middleware
// (JWT bearer or X-OAuth-Token from the BFF). The viewer is read
// from the echo context the same way every other authenticated
// handler does it.
//
// The current password is required as a re-authentication step
// even though the session is already valid — the IdP session alone
// is not sufficient proof for a credential mutation. A stolen
// laptop with an unlocked browser would otherwise let anyone change
// the password.
//
//	PATCH /password/change
//
// Status codes:
//
//	204 — success
//	400 — bad request body or validation failure
//	401 — current password is incorrect
//	500 — gRPC or hashing failure
//
// TODO: after persisting the new password, invalidate any other
// active sessions belonging to this person. Same caveat as Reset
// — requires a person→session-ids index which doesn't currently
// exist.
func (h *passwordHandler) Edit(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || viewer.ID == 0 {
		return c.NoContent(http.StatusUnauthorized)
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

	// Re-read the person from gRPC rather than trusting the cached
	// viewer — matches the posture the login handler takes when
	// re-fetching before TOTP verification.
	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || user == nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if !user.CheckPassword(req.CurrentPassword) {
		return c.NoContent(http.StatusUnauthorized)
	}

	if err := user.SetPassword(req.Password); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.Repos.Users.EditPassword(user.Password, user.ID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Fire-and-forget update-confirmation email. Different threat
	// model than Reset — an unexpected update means session access,
	// not email access, so the recovery action in the email is
	// "sign in" rather than "request a fresh reset."
	go func() {
		if err := h.Notif.Password.EmailUpdateSuccess(user); err != nil {
			log.Println(err)
		}
	}()

	return c.NoContent(http.StatusNoContent)
}
