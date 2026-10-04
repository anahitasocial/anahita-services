package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"auth/constants"
	"auth/data/entities"
	"auth/requests"

	"github.com/labstack/echo/v4"
)

// Signing in with an emailed code instead of a password.
//
// Three steps, each a POST answered with a redirect, so a refresh never
// repeats a submission:
//
//	/oauth/login/code          who is asking: an email address or a username
//	/oauth/login/code/method   where to send it (skipped while there is one way)
//	/oauth/login/code/verify   the six digits
//
// The code replaces the PASSWORD, not the second factor. An account with
// two-factor turned on goes on to /oauth/login/totp exactly as it does after a
// password.
//
// Nothing here says whether an account exists. An address nobody has gets the
// same pages, the same wording and the same five attempts as a real one; it
// is simply never sent anything.

const (
	sessionKeyPendingCode = "oauth_pending_code"

	// sessionKeyLoginCSRF holds one token for every form in the sign-in flow.
	//
	// One per browser session, not one per page shown. A token replaced on
	// every render breaks the second tab somebody opened, and the back
	// button, for no gain: what it protects against is another site
	// submitting the form, and that site cannot read either token.
	sessionKeyLoginCSRF = "oauth_login_csrf"

	// pendingCodeTTL is the code's own lifetime. State that outlived the
	// code would leave somebody typing into a page that can only refuse.
	pendingCodeTTL = constants.TokenExpiryTimeOTP * time.Second

	maxIdentifierLength = 255
)

// pendingCodeState is what the browser's session remembers between steps.
// It is held server-side; the browser has only a cookie naming it.
type pendingCodeState struct {
	// Identifier is what the person typed, shown back to them. Never the
	// address the code went to: typing a username must not reveal its email.
	Identifier string `json:"identifier"`
	Channel    string `json:"channel,omitempty"`

	// Sent is true once the send step has run, whether or not anything was
	// sent. It is what the verify page checks, so the page looks the same
	// for an account that exists and one that does not.
	Sent bool `json:"sent"`

	// TokenID is the code's row. Zero when nothing was sent: no such
	// account, a disabled one, or a rate limit.
	TokenID int64 `json:"token_id"`

	ExpiresAt time.Time `json:"expires_at"`

	// Attempts counts guesses when there is no token to count them on, so
	// an address with no account runs out after the same five.
	Attempts int `json:"attempts"`
}

// loginCodeChannels lists the ways a code can be sent. One, today. The method
// step appears by itself when there are two.
func loginCodeChannels() []string {
	return []string{constants.CodeChannelEmail}
}

// parseLoginCodeChannel reads the chosen channel. Anything unknown is email,
// so a tampered form cannot choose a channel that does not exist.
func parseLoginCodeChannel(raw string) string {
	wanted := strings.ToLower(strings.TrimSpace(raw))
	for _, channel := range loginCodeChannels() {
		if channel == wanted {
			return channel
		}
	}
	return constants.CodeChannelEmail
}

// ---------------------------------------------------------------------
// Step 1: who is asking
// ---------------------------------------------------------------------

// ReadLoginCode shows the form asking for an email address or username.
func (h *oauthHandler) ReadLoginCode(c echo.Context) error {
	if h.hasIdPSession(c) {
		return h.redirectToReturnTo(c, "ReadLoginCode/existing-session")
	}

	// Starting over: whatever was pending belongs to an earlier try.
	_ = h.Session.Delete(c, sessionKeyPendingCode)

	return h.renderTemplate(c, "login_code.html", h.codeContext(c, nil, ""))
}

// AddLoginCode takes the identifier and moves on. With one channel it sends
// the code here; with more it goes to the method step.
func (h *oauthHandler) AddLoginCode(c echo.Context) error {
	req := &requests.OAuthLoginCode{}
	if err := c.Bind(req); err != nil {
		log.Println("login code: form bind failed:", err)
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=identifier_required")
	}

	if !h.loginCSRFValid(c, req.CSRFToken) {
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=session_expired")
	}

	identifier := strings.ToLower(strings.TrimSpace(req.Identifier))
	if identifier == "" || len(identifier) > maxIdentifierLength {
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=identifier_required")
	}

	state := &pendingCodeState{
		Identifier: identifier,
		ExpiresAt:  time.Now().Add(pendingCodeTTL),
	}

	channels := loginCodeChannels()
	if len(channels) > 1 {
		if err := h.writePendingCode(c, *state); err != nil {
			log.Println("login code: pending state write failed:", err)
			return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=session_expired")
		}
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code/method")
	}

	return h.startLoginCode(c, state, channels[0])
}

// ---------------------------------------------------------------------
// Step 2: where to send it
// ---------------------------------------------------------------------

// ReadLoginCodeMethod shows the choice of channel. With only one there is
// nothing to choose, and the page is never the right place to be.
func (h *oauthHandler) ReadLoginCodeMethod(c echo.Context) error {
	if h.hasIdPSession(c) {
		return h.redirectToReturnTo(c, "ReadLoginCodeMethod/existing-session")
	}

	state, err := h.readPendingCode(c)
	if err != nil || state == nil {
		return c.Redirect(http.StatusFound, "/oauth/login/code?error=session_expired")
	}

	if len(loginCodeChannels()) < 2 {
		// Not sent from here: a GET must never send a message.
		if state.Sent {
			return c.Redirect(http.StatusFound, "/oauth/login/code/verify")
		}
		return c.Redirect(http.StatusFound, "/oauth/login/code")
	}

	return h.renderTemplate(c, "login_code_method.html", h.codeContext(c, state, ""))
}

// AddLoginCodeMethod sends the code by the chosen channel.
func (h *oauthHandler) AddLoginCodeMethod(c echo.Context) error {
	state, err := h.readPendingCode(c)
	if err != nil || state == nil {
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=session_expired")
	}

	req := &requests.OAuthLoginCodeMethod{}
	if err := c.Bind(req); err != nil {
		log.Println("login code: method bind failed:", err)
	}

	if !h.loginCSRFValid(c, req.CSRFToken) {
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=session_expired")
	}

	return h.startLoginCode(c, state, parseLoginCodeChannel(req.Channel))
}

// AddLoginCodeResend sends a fresh code and retires the one before it.
func (h *oauthHandler) AddLoginCodeResend(c echo.Context) error {
	state, err := h.readPendingCode(c)
	if err != nil || state == nil {
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=session_expired")
	}

	req := &requests.OAuthLoginCodeMethod{}
	if err := c.Bind(req); err != nil {
		log.Println("login code: resend bind failed:", err)
	}

	if !h.loginCSRFValid(c, req.CSRFToken) {
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=session_expired")
	}

	return h.startLoginCode(c, state, parseLoginCodeChannel(state.Channel))
}

// startLoginCode runs the send step and goes to the verify page. It always
// goes there, and always records the step as done, whatever happened inside.
func (h *oauthHandler) startLoginCode(c echo.Context, state *pendingCodeState, channel string) error {
	// Both limits are charged before the account is looked up, so being
	// limited says nothing about whether the account exists. Both are
	// charged every time, so neither can be used to probe the other.
	limitedByAddress := h.rateLimitExceeded(rateLimitScopeLoginCodeEmail, state.Identifier)
	limitedByIP := h.rateLimitExceeded(rateLimitScopeLoginCodeIP, c.RealIP())

	var tokenID int64
	if !limitedByAddress && !limitedByIP {
		tokenID = h.sendLoginCode(state.Identifier, channel)
	}

	state.Sent = true
	state.Channel = channel
	state.TokenID = tokenID
	state.Attempts = 0
	state.ExpiresAt = time.Now().Add(pendingCodeTTL)

	if err := h.writePendingCode(c, *state); err != nil {
		log.Println("login code: pending state write failed:", err)
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=session_expired")
	}

	return c.Redirect(http.StatusSeeOther, "/oauth/login/code/verify")
}

// sendLoginCode issues a code and sends it, when there is an account to send
// it to. It returns the token's ID, or zero when nothing was sent.
//
// The message goes out after the response, on its own goroutine: how long the
// mail server takes must not show in how long this page takes.
func (h *oauthHandler) sendLoginCode(identifier, channel string) int64 {
	user, err := h.Repos.Users.ReadByEmailOrUsername(identifier)
	if err != nil || user == nil {
		log.Println("login code: no account for the identifier")
		return 0
	}

	// A disabled account cannot sign in by any means, so it gets no code.
	// A deleted or archived one does: the code proves who is asking, and
	// what happens next is decided when it is checked.
	if !user.Enabled {
		log.Printf("login code: not sent to a disabled account (user=%d)", user.ID)
		return 0
	}

	// A new code retires the ones before it. Without this, asking again
	// would leave two live codes, each with its own five attempts.
	if _, err := h.Repos.Tokens.InvalidateOutstanding(constants.TokenTypeLoginOTP, user.Email); err != nil {
		log.Println("login code: could not retire earlier codes:", err)
		return 0
	}

	token, err := h.issueCode(user, constants.TokenTypeLoginOTP, channel)
	if err != nil {
		log.Println("login code:", err)
		return 0
	}

	recipient, code := *user, token.Code
	go func() {
		// Email is the only channel. When SMS exists it is tried here
		// first, falling back to email when the number is not verified.
		if err := h.Notif.Auth.EmailLoginCode(&recipient, code); err != nil {
			log.Println("login code: email failed:", err)
		}
	}()

	log.Printf("login code: issued (user=%d channel=%s)", user.ID, channel)

	return token.ID
}

// ---------------------------------------------------------------------
// Step 3: the six digits
// ---------------------------------------------------------------------

// ReadLoginCodeVerify shows the form for the code.
func (h *oauthHandler) ReadLoginCodeVerify(c echo.Context) error {
	if h.hasIdPSession(c) {
		return h.redirectToReturnTo(c, "ReadLoginCodeVerify/existing-session")
	}

	state, err := h.readPendingCode(c)
	if err != nil || state == nil {
		return c.Redirect(http.StatusFound, "/oauth/login/code?error=session_expired")
	}

	if !state.Sent {
		return c.Redirect(http.StatusFound, "/oauth/login/code")
	}

	return h.renderTemplate(c, "login_code_verify.html", h.codeContext(c, state, ""))
}

// AddLoginCodeVerify checks the code and, when it is right, signs the person
// in the same way a correct password would.
func (h *oauthHandler) AddLoginCodeVerify(c echo.Context) error {
	state, err := h.readPendingCode(c)
	if err != nil || state == nil {
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=session_expired")
	}

	if !state.Sent {
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code")
	}

	req := &requests.OAuthLoginCodeVerify{}
	if err := c.Bind(req); err != nil {
		log.Println("login code verify: form bind failed:", err)
		return h.renderCodeError(c, state, "Please enter the 6-digit code.")
	}

	if !h.loginCSRFValid(c, req.CSRFToken) {
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=session_expired")
	}

	// Something that is not six digits cannot be a code, so it costs
	// nothing. A slip of the finger should not use up an attempt.
	code := strings.TrimSpace(req.Code)
	if len(code) != constants.OTPCodeLength || !isAllDigits(code) {
		return h.renderCodeError(c, state, "Please enter the 6-digit code.")
	}

	// Nothing was sent. Count the guess here, so the page runs out after
	// the same five tries it would for a real account.
	if state.TokenID == 0 {
		state.Attempts++
		if state.Attempts >= constants.OTPMaxAttempts {
			_ = h.Session.Delete(c, sessionKeyPendingCode)
			return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=code_expired")
		}
		if err := h.writePendingCode(c, *state); err != nil {
			log.Println("login code verify: could not record the attempt:", err)
		}
		return h.renderCodeError(c, state,
			wrongCodeMessage(constants.OTPMaxAttempts-state.Attempts))
	}

	token, remaining, err := h.verifyCode(state.TokenID, constants.TokenTypeLoginOTP, code)
	if err != nil {
		if errors.Is(err, errCodeMismatch) {
			return h.renderCodeError(c, state, wrongCodeMessage(remaining))
		}
		_ = h.Session.Delete(c, sessionKeyPendingCode)
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=code_expired")
	}

	// The account as it is NOW, not as it was when the code was sent. Ten
	// minutes is long enough for it to have been disabled in between.
	//
	// By the address the code went to, which is the token's, not by what
	// was typed: a username can be changed between the two steps.
	user, err := h.Repos.Users.ReadByEmail(token.RecipientEmail)
	if err != nil || user == nil {
		log.Println("login code verify: account lookup failed:", err)
		_ = h.Session.Delete(c, sessionKeyPendingCode)
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=code_expired")
	}

	// The code is used up before anything else is decided, so no outcome
	// below can be tried twice with it.
	if !h.consumeCode(token, user.ID) {
		_ = h.Session.Delete(c, sessionKeyPendingCode)
		return c.Redirect(http.StatusSeeOther, "/oauth/login/code?error=code_expired")
	}

	_ = h.Session.Delete(c, sessionKeyPendingCode)
	h.rateLimitReset(rateLimitScopeLoginCodeEmail, state.Identifier)

	return h.admitAfterCode(c, user)
}

// admitAfterCode applies the same gates, in the same order, as a correct
// password does in AddCredentials.
func (h *oauthHandler) admitAfterCode(c echo.Context, user *entities.User) error {
	if !user.Enabled {
		log.Printf("login code verify: rejected disabled account (user=%d)", user.ID)
		return h.renderLoginError(c,
			"We could not sign you in. Contact support if this continues.")
	}

	if user.DeletedAt != nil {
		log.Printf("login code verify: account scheduled for deletion, offering restore (user=%d)", user.ID)
		return h.renderRestoreForm(c, user)
	}

	if user.ArchivedAt != nil {
		log.Printf("login code verify: rejected archived account (user=%d)", user.ID)
		return h.renderLoginError(c,
			"This account has been archived. Its posts remain readable, but it "+
				"can no longer be signed in to. Contact support if you believe "+
				"this is a mistake.")
	}

	// The code arrived at the account's address and came back, which is all
	// the confirmation link proves. There is nothing left to ask for.
	if !user.IsEmailVerified() {
		if err := h.Repos.Users.MarkEmailVerified(user.ID); err != nil {
			log.Printf("login code verify: could not mark the address verified (user=%d): %v", user.ID, err)
			return h.renderLoginError(c, "Could not sign you in. Please try again.")
		}
		log.Printf("login code verify: address verified by code (user=%d)", user.ID)
	}

	if user.TotpEnabled {
		if err := h.writePendingTOTP(c, pendingTOTPState{
			Email:     user.Email,
			ExpiresAt: time.Now().Add(pendingTOTPTTL),
		}); err != nil {
			log.Println("login code verify: pending TOTP write failed:", err)
			return h.renderLoginError(c, "Could not start verification. Please try again.")
		}
		log.Printf("login code verify: code OK, pending TOTP (user=%d)", user.ID)
		return c.Redirect(http.StatusFound, "/oauth/login/totp")
	}

	log.Printf("login code verify: code OK, no TOTP (user=%d)", user.ID)
	return h.finalizeLogin(c, user)
}

// wrongCodeMessage says the code was wrong, and how many tries are left once
// that is worth knowing.
func wrongCodeMessage(remaining int) string {
	if remaining <= 2 {
		return fmt.Sprintf("That code is not correct. %d %s left.",
			remaining, pluralize(remaining, "try", "tries"))
	}
	return "That code is not correct."
}

// ---------------------------------------------------------------------
// Pages and state
// ---------------------------------------------------------------------

func (h *oauthHandler) codeContext(c echo.Context, state *pendingCodeState, errMsg string) map[string]any {
	hint := ""
	if errMsg == "" {
		switch c.QueryParam("error") {
		case "identifier_required":
			hint = "Enter the email address or username for your account."
		case "session_expired":
			errMsg = "That took too long. Please start again."
		case "code_expired":
			errMsg = "That code is no longer valid. Please request a new one."
		}
	}

	ctx := map[string]any{
		"Error":       errMsg,
		"Hint":        hint,
		"FrontendURL": h.Config.GetFrontendURL(),
		"CSRFToken":   h.loginCSRFToken(c),
		"Channels":    loginCodeChannels(),
	}
	if state != nil {
		ctx["Identifier"] = state.Identifier
		ctx["Channel"] = state.Channel
	}

	return ctx
}

func (h *oauthHandler) renderCodeError(c echo.Context, state *pendingCodeState, message string) error {
	return h.renderTemplate(c, "login_code_verify.html", h.codeContext(c, state, message))
}

// writePendingCode updates the state in place when the browser already has
// it, and creates it otherwise. Add alone would hand out a new session on
// every step and leave the earlier ones behind.
func (h *oauthHandler) writePendingCode(c echo.Context, state pendingCodeState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}

	if existing, err := h.Session.Read(c, sessionKeyPendingCode); err == nil && existing != "" {
		if err := h.Session.Edit(c, sessionKeyPendingCode, string(raw)); err == nil {
			return nil
		}
	}

	_, err = h.Session.AddWithTTL(c, sessionKeyPendingCode, string(raw), pendingCodeTTL)
	return err
}

func (h *oauthHandler) readPendingCode(c echo.Context) (*pendingCodeState, error) {
	raw, err := h.Session.Read(c, sessionKeyPendingCode)
	if err != nil || raw == "" {
		return nil, err
	}

	var state pendingCodeState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, err
	}

	if time.Now().After(state.ExpiresAt) {
		_ = h.Session.Delete(c, sessionKeyPendingCode)
		return nil, nil
	}

	return &state, nil
}

// ---------------------------------------------------------------------
// The sign-in forms' CSRF token
// ---------------------------------------------------------------------

// loginCSRFToken returns this browser session's token for the sign-in forms,
// making one if there is none. Empty only if the session store is failing, in
// which case the form that carries it will be refused, which is the safe way
// for that to go.
func (h *oauthHandler) loginCSRFToken(c echo.Context) string {
	if existing, err := h.Session.Read(c, sessionKeyLoginCSRF); err == nil && existing != "" {
		return existing
	}

	token, err := generateCSRFToken()
	if err != nil {
		log.Println("login: could not generate a CSRF token:", err)
		return ""
	}

	if _, err := h.Session.Add(c, sessionKeyLoginCSRF, token); err != nil {
		log.Println("login: could not store the CSRF token:", err)
		return ""
	}

	return token
}

// loginCSRFValid reports whether a submitted form carries this session's
// token. A session with no token has no valid form.
func (h *oauthHandler) loginCSRFValid(c echo.Context, submitted string) bool {
	stored, err := h.Session.Read(c, sessionKeyLoginCSRF)
	if err != nil || stored == "" || submitted == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(stored), []byte(submitted)) == 1
}
