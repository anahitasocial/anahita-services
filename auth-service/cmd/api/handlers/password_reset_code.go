package handlers

import (
	"database/sql"
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

// Resetting a forgotten password with an emailed code.
//
//	GET  /password/forgot         ask for the address
//	POST /password/forgot         send the code (also the "send a new code" button)
//	GET  /password/reset/verify   the page waiting for the code
//	POST /password/reset/verify   the code
//	GET  /password/reset          choose the new password
//	POST /password/reset          set it
//
// The code is typed into the page that asked for it, so the reset finishes in
// the browser, app or device it started in, wherever the mail is read.
//
// A correct code does not use the code up. It marks this browser's session as
// allowed to choose a password, and the code is used up when the password is
// set. Until then the code's own clock keeps running: fifteen minutes from
// when it was sent, for both steps.
//
// /password/reset?token=… still works, for one caller: the first super
// administrator's set-password link, which is read out of a log.

const (
	sessionKeyPasswordReset = "password_reset_pending"

	passwordResetCodeTTL = constants.TokenExpiryTimeCode * time.Second
)

// ForgotForm renders the "forgot password" page.
//
//	GET /password/forgot
func (h *passwordHandler) ForgotForm(c echo.Context) error {
	data := map[string]any{"CSRFToken": h.loginCSRFToken(c)}
	if c.QueryParam("error") == "expired" {
		data["Error"] = "That code is no longer valid. Enter your address and we will send a new one."
	}

	return h.renderTemplate(c, "password_forgot.html", data)
}

// Forgot sends a reset code and goes to the page waiting for it.
//
//	POST /password/forgot
//
// The answer is the same whether or not the address has an account, and
// whether or not a limit was reached: a redirect to the code page. An address
// with no account is sent nothing, and its page runs out after the same five
// tries.
func (h *passwordHandler) Forgot(c echo.Context) error {
	req := &requests.PasswordForgot{}
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return h.renderTemplate(c, "password_forgot.html", map[string]any{
			"Error":     "Please enter a valid email address.",
			"CSRFToken": h.loginCSRFToken(c),
		})
	}

	if !h.loginCSRFValid(c, req.CSRFToken) {
		return h.renderTemplate(c, "password_forgot.html", map[string]any{
			"Error":     "This page expired. Please try again.",
			"Email":     req.Email,
			"CSRFToken": h.loginCSRFToken(c),
		})
	}

	// The "send a new code" button sends no address: the one this browser
	// is already waiting on is used.
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		if step, err := h.readCodeStep(c, sessionKeyPasswordReset); err == nil && step != nil {
			req.Email = step.Subject
		}
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return h.renderTemplate(c, "password_forgot.html", map[string]any{
			"Error":     "Please enter a valid email address.",
			"Email":     req.Email,
			"CSRFToken": h.loginCSRFToken(c),
		})
	}

	// Both counted before the account is looked up, and both every time,
	// so reaching a limit says nothing about whether the account exists.
	// Each code is an email, so this is also what stops the form being
	// used to flood somebody's inbox.
	limitedByAddress := h.rateLimitExceeded(rateLimitScopePasswordEmail, req.Email)
	limitedByIP := h.rateLimitExceeded(rateLimitScopePasswordIP, getClientIP(c))

	var tokenID int64
	if !limitedByAddress && !limitedByIP {
		tokenID = h.sendResetCode(req.Email)
	}

	step := codeStep{
		Subject:   req.Email,
		TokenID:   tokenID,
		ExpiresAt: time.Now().Add(passwordResetCodeTTL),
	}
	if err := h.writeCodeStep(c, sessionKeyPasswordReset, step, passwordResetCodeTTL); err != nil {
		log.Println("password forgot: could not remember the pending code:", err)
		return h.renderTemplate(c, "password_forgot.html", map[string]any{
			"Error":     "Something went wrong. Please try again.",
			"Email":     req.Email,
			"CSRFToken": h.loginCSRFToken(c),
		})
	}

	return c.Redirect(http.StatusSeeOther, "/password/reset/verify")
}

// sendResetCode issues a reset code for an address that has an account, and
// returns its token, or zero when nothing was sent.
//
// A deleted account still gets one: resetting the password is how somebody
// who forgot it gets back in to restore the account. The mail goes out after
// the response, so how long the mail server takes does not show.
func (h *passwordHandler) sendResetCode(email string) int64 {
	user, err := h.Repos.Users.ReadByEmail(email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Println("password forgot: user lookup failed:", err)
	}
	if user == nil {
		return 0
	}

	token, err := h.issueCode(codeRequest{
		Email:    user.Email,
		PersonID: user.ID,
		Type:     constants.TokenTypePasswordResetCode,
		Channel:  constants.CodeChannelEmail,
		Lifetime: constants.TokenExpiryTimeCode,
	})
	if err != nil {
		log.Println("password forgot:", err)
		return 0
	}

	recipient, code := *user, token.Code
	go func() {
		if err := h.Notif.Password.EmailResetCode(&recipient, code); err != nil {
			log.Println("password forgot: email failed:", err)
		}
	}()

	return token.ID
}

func (h *passwordHandler) resetCodeContext(c echo.Context, step *codeStep, errMsg string) map[string]any {
	return map[string]any{
		"Email":     step.Subject,
		"Error":     errMsg,
		"CSRFToken": h.loginCSRFToken(c),
	}
}

// VerifyForm shows the page waiting for the reset code.
//
//	GET /password/reset/verify
func (h *passwordHandler) VerifyForm(c echo.Context) error {
	step, err := h.readCodeStep(c, sessionKeyPasswordReset)
	if err != nil || step == nil {
		return c.Redirect(http.StatusFound, "/password/forgot")
	}

	if step.Verified {
		return c.Redirect(http.StatusFound, "/password/reset")
	}

	return h.renderTemplate(c, "password_reset_code.html", h.resetCodeContext(c, step, ""))
}

// Verify checks the reset code and, when it is right, lets this browser on to
// the new-password form.
//
//	POST /password/reset/verify
func (h *passwordHandler) Verify(c echo.Context) error {
	step, err := h.readCodeStep(c, sessionKeyPasswordReset)
	if err != nil || step == nil {
		return c.Redirect(http.StatusSeeOther, "/password/forgot?error=expired")
	}

	req := &requests.CodeForm{}
	if err := c.Bind(req); err != nil {
		log.Println("password reset verify: form bind failed:", err)
	}

	if !h.loginCSRFValid(c, req.CSRFToken) {
		return h.renderTemplate(c, "password_reset_code.html",
			h.resetCodeContext(c, step, "This page expired. Please enter the code again."))
	}

	_, result, remaining := h.submitCode(c, sessionKeyPasswordReset, step,
		constants.TokenTypePasswordResetCode, req.Code)

	switch result {
	case codeMalformed:
		return h.renderTemplate(c, "password_reset_code.html",
			h.resetCodeContext(c, step, "Please enter the 6-digit code."))
	case codeWrong:
		return h.renderTemplate(c, "password_reset_code.html",
			h.resetCodeContext(c, step, wrongCodeMessage(remaining)))
	case codeDead:
		return c.Redirect(http.StatusSeeOther, "/password/forgot?error=expired")
	}

	step.Verified = true
	if err := h.writeCodeStep(c, sessionKeyPasswordReset, *step, time.Until(step.ExpiresAt)); err != nil {
		log.Println("password reset verify: could not record the verified step:", err)
		return c.Redirect(http.StatusSeeOther, "/password/forgot?error=expired")
	}

	return c.Redirect(http.StatusSeeOther, "/password/reset")
}

// verifiedReset returns the account this browser may reset, and the code that
// says so, or nils when it may not: no code was entered here, or the code has
// since expired, been replaced, or been used.
func (h *passwordHandler) verifiedReset(c echo.Context) (*entities.User, *entities.Token) {
	step, err := h.readCodeStep(c, sessionKeyPasswordReset)
	if err != nil || step == nil || !step.Verified || step.TokenID == 0 {
		return nil, nil
	}

	token, err := h.Repos.Tokens.ReadByID(step.TokenID)
	if err != nil || token == nil ||
		token.Type != constants.TokenTypePasswordResetCode ||
		token.IsUsed || token.IsExpired() {
		_ = h.Session.Delete(c, sessionKeyPasswordReset)
		return nil, nil
	}

	user, err := h.Repos.Users.ReadByEmail(token.RecipientEmail)
	if err != nil || user == nil {
		log.Println("password reset: account lookup failed:", err)
		_ = h.Session.Delete(c, sessionKeyPasswordReset)
		return nil, nil
	}

	return user, token
}

// resetFormAfterCode shows the new-password form to a browser that has
// entered its code.
func (h *passwordHandler) resetFormAfterCode(c echo.Context) error {
	user, _ := h.verifiedReset(c)
	if user == nil {
		return c.Redirect(http.StatusFound, "/password/forgot")
	}

	data := map[string]any{"Token": ""}

	// Somebody resetting the password of an account that is scheduled for
	// deletion is usually trying to get back in to restore it. Say so here,
	// where they will read it.
	if user.DeletedAt != nil {
		data["ScheduledForDeletion"] = true
		data["PurgeDate"] = user.DeletedAt.
			AddDate(0, 0, constants.AccountGraceDays()).
			Format("2 January 2006")
	}

	return h.renderTemplate(c, "password_reset.html", data)
}

// resetAfterCode sets the new password for a browser that has entered its
// code.
func (h *passwordHandler) resetAfterCode(c echo.Context, req *requests.PasswordReset) error {
	user, token := h.verifiedReset(c)
	if user == nil {
		return c.Redirect(http.StatusSeeOther, "/password/forgot?error=expired")
	}

	if len(req.Password) < 15 || len(req.Password) > 150 {
		return h.renderTemplate(c, "password_reset.html", map[string]any{
			"Token": "",
			"Error": "Password must be at least 15 characters long.",
		})
	}

	probeKey := fmt.Sprintf("code:%d", token.ID)

	return h.applyReset(c, user, req.Password, "", probeKey, func() {
		// If this fails the password is already set. The step is removed
		// either way, and the code expires on its own within minutes.
		h.consumeCode(token, user.ID)
		_ = h.Session.Delete(c, sessionKeyPasswordReset)
	})
}
