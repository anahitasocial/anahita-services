package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"auth/data/entities"
	"auth/requests"

	libs_constants "anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

// Session key + TTL for the bridge between credentials (step 1) and
// TOTP (step 2). The state is short-lived so a stale form submission
// can't bypass re-entering credentials, and bounded by an attempt
// counter so a stolen step-1 cookie can't brute-force the code.
const (
	sessionKeyPendingTOTP = "oauth_pending_totp"

	// 2m is long enough for the person to open their authenticator
	// app, short enough that a stolen step-1 session cookie has
	// limited brute-force time when paired with the attempt cap.
	pendingTOTPTTL = 2 * time.Minute

	// Hard cap on failed TOTP submissions before the flow is
	// restarted. Only attempts with the correct shape (6 digits)
	// count — malformed submissions yield no information and so
	// don't deplete the budget.
	maxTOTPAttempts = 5
)

// pendingTOTPState lives in the IdP session cookie between
// AddCredentials (which writes it after a successful password check)
// and AddTotp (which consumes it). The Email is re-resolved server-
// side; we do not trust any person identifier coming from the form.
type pendingTOTPState struct {
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
	Attempts  int       `json:"attempts"`
}

// ReadLogin handles GET /oauth/login.
//
// Renders step 1 (email + password). If the viewer already holds a
// valid IdP session, redirects back into the OAuth flow. Any
// in-progress TOTP state is cleared so the page is always clean on
// direct navigation — this is also the target of the "Back to sign
// in" link from step 2.
func (h *oauthHandler) ReadLogin(c echo.Context) error {
	if h.hasIdPSession(c) {
		return h.redirectToReturnTo(c, "ReadLogin/existing-session")
	}

	_ = h.Session.Delete(c, sessionKeyPendingTOTP)

	return h.renderTemplate(c, "login.html", h.loginContext(c, ""))
}

// AddCredentials handles POST /oauth/login (step 1).
//
// Validates email + password and the account's enabled status.
// If the account has TOTP enrolled, persists a short-lived pending
// state and redirects to GET /oauth/login/totp (POST/Redirect/GET).
// Otherwise finalizes the IdP session and bounces back to the OAuth
// flow.
//
// Auth log entries and login-notification emails remain deferred to
// token issuance (oauth_token.go).
func (h *oauthHandler) AddCredentials(c echo.Context) error {
	var (
		req = requests.NewOAuthLoginRequest().Login
		err error
	)

	if err = c.Bind(req); err != nil {
		log.Println("login: form bind failed:", err)
		return h.renderLoginError(c, "Invalid form submission.")
	}

	// Manual presence check rather than c.Validate(req) because the
	// shared LoginRequest struct also carries TotpPasscode (used in
	// step 2), and we don't want validator tags on that field to
	// affect step 1.
	if req.Identifier == "" || req.Password == "" {
		return h.renderLoginError(c, "Please enter your email and password.")
	}

	user, err := h.Repos.Users.ReadByEmailOrUsername(req.Identifier)
	if err != nil || user == nil {
		log.Println("login: user lookup failed:", err)
		return h.renderLoginError(c, "Invalid email or password.")
	}

	// Password check happens before the disabled check so that both
	// "wrong password" and "disabled" paths run the same hash work.
	// Otherwise the response time itself reveals which case applies.
	if !user.CheckPassword(req.Password) {
		log.Printf("login: bad password (user=%d)", user.ID)
		return h.renderLoginError(c, "Invalid email or password.")
	}

	// Scheduled for deletion, and the password was right. Offer it back.
	//
	// This is the whole recovery path: deletion clears Enabled, so without this
	// branch a person who changed their mind would meet the same "Invalid email
	// or password" as everybody else and have no way to reach their own
	// account.
	//
	// AFTER the password check, deliberately. Saying "this account is scheduled
	// for deletion" to anyone who types an address would turn the grace period
	// into a directory of accounts on their way out — and a deleted account is
	// a softer target than a live one, since nobody is reading its mail. Behind
	// the password, the person has already proved it is theirs.
	if user.DeletedAt != nil {
		log.Printf("login: account scheduled for deletion, offering restore (user=%d)", user.ID)
		return h.renderRestoreForm(c, user)
	}

	// Disabled accounts return the same generic message intentionally.
	// Distinguishing "disabled" from "wrong password" enables account
	// enumeration against any leaked credential dump.
	if !user.Enabled {
		log.Printf("login: rejected disabled user (user=%d)", user.ID)
		return h.renderLoginError(c, "Invalid email or password.")
	}

	// Branch on TOTP enrollment.
	if user.TotpEnabled {
		state := pendingTOTPState{
			Email:     user.Email,
			ExpiresAt: time.Now().Add(pendingTOTPTTL),
			Attempts:  0,
		}
		if err := h.writePendingTOTP(c, state); err != nil {
			log.Println("login: pending state write failed:", err)
			return h.renderLoginError(c, "Could not start verification. Please try again.")
		}
		log.Printf("login: credentials OK, pending TOTP (user=%d)", user.ID)
		return c.Redirect(http.StatusFound, "/oauth/login/totp")
	}

	log.Printf("login: credentials OK, no TOTP (user=%d)", user.ID)
	return h.finalizeLogin(c, user)
}

// ReadTotp handles GET /oauth/login/totp (step 2).
//
// Renders the verification code form when a pending state exists and
// hasn't expired. If the state is missing or expired, falls back to
// step 1 with a friendly error.
func (h *oauthHandler) ReadTotp(c echo.Context) error {
	if h.hasIdPSession(c) {
		return h.redirectToReturnTo(c, "ReadTotp/existing-session")
	}

	state, err := h.readPendingTOTP(c)
	if err != nil || state == nil {
		return c.Redirect(http.StatusFound, "/oauth/login?error=session_expired")
	}

	return h.renderTemplate(c, "login_totp.html", h.totpContext(state, ""))
}

// AddTotp handles POST /oauth/login/totp (step 2).
//
// Re-resolves the person from the email stored in pending state (so
// any change between step 1 and step 2 — disabled, deleted — is
// caught), validates the code, and finalizes the IdP session.
//
// Failed attempts on well-formed codes are counted; exceeding
// maxTOTPAttempts terminates the pending state and forces the person
// back to step 1.
func (h *oauthHandler) AddTotp(c echo.Context) error {
	state, err := h.readPendingTOTP(c)
	if err != nil || state == nil {
		return c.Redirect(http.StatusFound, "/oauth/login?error=session_expired")
	}

	req := requests.NewOAuthLoginRequest().Login
	if err := c.Bind(req); err != nil {
		log.Println("totp: form bind failed:", err)
		return h.renderTotpError(c, state, "Invalid form submission.")
	}

	// Shape errors don't count toward the attempt budget — they
	// reveal nothing about the correct code, so brute force has
	// to go through the validation path below.
	if len(req.TotpPasscode) != 6 || !isAllDigits(req.TotpPasscode) {
		return h.renderTotpError(c, state, "Enter the 6-digit code from your authenticator app.")
	}

	// Re-fetch fresh data — the person could have been disabled
	// between step 1 and step 2.
	user, err := h.Repos.Users.ReadByEmail(state.Email)
	if err != nil || user == nil {
		log.Println("totp: user re-lookup failed:", err)
		_ = h.Session.Delete(c, sessionKeyPendingTOTP)
		return c.Redirect(http.StatusFound, "/oauth/login?error=session_expired")
	}

	if user.Enabled == false {
		log.Printf("totp: person disabled mid-flow (person=%d)", user.ID)
		_ = h.Session.Delete(c, sessionKeyPendingTOTP)
		return h.renderLoginError(c, "Invalid email or password.")
	}

	// Defensive: the only way pending state was created was via the
	// TotpEnabled branch in AddCredentials. If that's no longer true,
	// something is off — restart the flow.
	if !user.TotpEnabled {
		log.Printf("totp: pending state for non-TOTP person, restarting (person=%d)", user.ID)
		_ = h.Session.Delete(c, sessionKeyPendingTOTP)
		return c.Redirect(http.StatusFound, "/oauth/login")
	}

	totpValidator := newTotpValidatorHandler(h.Env)
	if ok, err := totpValidator.ValidateTotp(c, req.TotpPasscode, user); !ok {
		state.Attempts++
		log.Printf("totp: validation failed (person=%d attempts=%d/%d err=%v)",
			user.ID, state.Attempts, maxTOTPAttempts, err)

		if state.Attempts >= maxTOTPAttempts {
			_ = h.Session.Delete(c, sessionKeyPendingTOTP)
			return c.Redirect(http.StatusFound, "/oauth/login?error=too_many_attempts")
		}

		// Best-effort persist the incremented counter. Failing closed
		// here would hand attackers a way to reset the counter by
		// inducing write failures, so we log and continue.
		if err := h.writePendingTOTP(c, *state); err != nil {
			log.Println("totp: failed to persist attempt counter:", err)
		}

		remaining := maxTOTPAttempts - state.Attempts
		msg := "Invalid verification code."
		if remaining <= 2 {
			msg = fmt.Sprintf("Invalid verification code. %d %s remaining before you'll need to sign in again.",
				remaining, pluralize(remaining, "attempt", "attempts"))
		}
		return h.renderTotpError(c, state, msg)
	}

	user.TotpLastVerifiedAt = time.Now()
	if _, err := h.Repos.Users.Edit(user); err != nil {
		log.Println("totp: failed to record TOTP verification time:", err)
		// Non-fatal — the person authenticated successfully; we just
		// failed to update an audit field.
	}

	_ = h.Session.Delete(c, sessionKeyPendingTOTP)

	log.Printf("totp: validated (user=%d)", user.ID)
	return h.finalizeLogin(c, user)
}

// writeViewerSession builds the Viewer, persists it to the IdP
// session, and indexes the session against its owner.
//
// Shared by both login paths deliberately. The three steps have to
// happen together — a session created without being indexed cannot be
// revoked remotely, which silently defeats password reset — and a
// second copy of this sequence is a second place to forget the third
// step.
func (h *oauthHandler) writeViewerSession(c echo.Context, user *entities.User) error {
	viewer := libs_entities.NewViewer()
	viewer.ID = user.ID
	viewer.Email = user.Email
	viewer.Authenticated = true

	viewerJSON, err := json.Marshal(viewer)
	if err != nil {
		return fmt.Errorf("viewer marshal: %w", err)
	}

	sessionID, err := h.Session.Add(c, libs_constants.UserSessionKey, string(viewerJSON))
	if err != nil {
		return fmt.Errorf("session write: %w", err)
	}

	h.indexIdPSession(user.ID, sessionID)

	return nil
}

// finalizeLogin builds the Viewer, persists it to the IdP session,
// and bounces back to the stored OAuth flow URL (or the frontend
// fallback — see redirectToReturnTo in oauth_return_to.go).
func (h *oauthHandler) finalizeLogin(c echo.Context, user *entities.User) error {
	if err := h.writeViewerSession(c, user); err != nil {
		log.Println("login:", err)
		return h.renderLoginError(c, "Could not create session. Please try again.")
	}
	return h.redirectToReturnTo(c, fmt.Sprintf("finalizeLogin/user=%d", user.ID))
}

// readPendingTOTP returns the current pending state, or nil if it's
// missing, malformed, or expired. Expired state is cleared as a side
// effect so the next request starts clean.
func (h *oauthHandler) readPendingTOTP(c echo.Context) (*pendingTOTPState, error) {
	raw, err := h.Session.Read(c, sessionKeyPendingTOTP)
	if err != nil || raw == "" {
		return nil, err
	}
	var state pendingTOTPState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, err
	}
	if time.Now().After(state.ExpiresAt) {
		_ = h.Session.Delete(c, sessionKeyPendingTOTP)
		return nil, nil
	}
	return &state, nil
}

// writePendingTOTP serializes and stores pending state. Extracted so
// the attempt-counter update path uses the same code as initial
// creation.
func (h *oauthHandler) writePendingTOTP(c echo.Context, state pendingTOTPState) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = h.Session.Add(c, sessionKeyPendingTOTP, string(stateJSON))
	return err
}

// loginContext builds the template context for step 1. If errMsg is
// empty, falls back to translating the ?error= query param.
func (h *oauthHandler) loginContext(c echo.Context, errMsg string) map[string]any {
	if errMsg == "" {
		switch c.QueryParam("error") {
		case "session_expired":
			errMsg = "Your sign-in session expired. Please try again."
		case "too_many_attempts":
			errMsg = "Too many incorrect verification codes. Please sign in again."
		}
	}
	return map[string]any{
		"Error":       errMsg,
		"FrontendURL": h.Config.GetFrontendURL(),
		"ReturnParam": h.returnToParam(c),
	}
}

// totpContext builds the template context for step 2.
func (h *oauthHandler) totpContext(state *pendingTOTPState, errMsg string) map[string]any {
	return map[string]any{
		"Error":       errMsg,
		"Email":       state.Email,
		"FrontendURL": h.Config.GetFrontendURL(),
	}
}

// renderLoginError re-renders step 1 with an error message.
func (h *oauthHandler) renderLoginError(c echo.Context, message string) error {
	return h.renderTemplate(c, "login.html", h.loginContext(c, message))
}

// renderTotpError re-renders step 2 with an error message. Falls
// back to step 1 if the pending state has been lost.
func (h *oauthHandler) renderTotpError(c echo.Context, state *pendingTOTPState, message string) error {
	if state == nil {
		return h.renderLoginError(c, message)
	}
	return h.renderTemplate(c, "login_totp.html", h.totpContext(state, message))
}

// hasIdPSession reports whether the request carries a valid IdP
// session cookie containing a Viewer with Authenticated=true.
func (h *oauthHandler) hasIdPSession(c echo.Context) bool {
	viewer, err := h.readIdPSessionViewer(c)
	return err == nil && viewer != nil && viewer.Authenticated
}

// readIdPSessionViewer parses the IdP session cookie and returns the
// stored Viewer. Returns an error if the session is missing or its
// payload can't be unmarshaled.
func (h *oauthHandler) readIdPSessionViewer(c echo.Context) (*libs_entities.Viewer, error) {
	raw, err := h.Session.Read(c, libs_constants.UserSessionKey)
	if err != nil {
		return nil, err
	}
	viewer := libs_entities.NewViewer()
	if err := json.Unmarshal([]byte(raw), viewer); err != nil {
		return nil, err
	}
	return viewer, nil
}

// isAllDigits reports whether s contains only ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// pluralize returns singular if n == 1, plural otherwise.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
