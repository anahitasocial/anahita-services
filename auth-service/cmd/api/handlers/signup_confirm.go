package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"auth/constants"
	"auth/data/entities"
	"auth/requests"

	"github.com/labstack/echo/v4"
)

// Confirming the address an account, or an application to join, was made
// with.
//
// By a six-digit code typed into the page that signup leads to, not by a
// link. A link opens wherever the mail is read, which is often not the
// browser, the app or even the device the signup started on, and a link that
// confirms on a GET is confirmed by any mail scanner that follows it.
//
//	GET  /signup/confirm          the page waiting for the code
//	POST /signup/confirm          the code
//	GET  /signup/confirm/resend   ask for a code by address
//	POST /signup/confirm/resend   send one, from either page
//
// Nothing here says whether an address has an account. Asking for a code for
// an address that has none, or whose address is already confirmed, leads to
// the same page, which runs out after the same five tries.

const (
	sessionKeySignupConfirm = "signup_pending_confirm"

	signupConfirmTTL = constants.TokenExpiryTimeCode * time.Second
)

// sendConfirmationCode issues a confirmation code and mails it. personID is
// zero for an application, which has no account yet.
//
// The mail is sent before this returns, unlike a sign-in code: the caller has
// just created the account and can afford to say so if the mail fails, and
// there is no existence to hide from somebody who has just filled in the
// signup form.
func (h *signupHandler) sendConfirmationCode(email string, personID int64, alias string) (int64, error) {
	token, err := h.issueCode(codeRequest{
		Email:    email,
		PersonID: personID,
		Type:     constants.TokenTypeSignupConfirmCode,
		Channel:  constants.CodeChannelEmail,
		Lifetime: constants.TokenExpiryTimeCode,
	})
	if err != nil {
		return 0, err
	}

	recipient := entities.NewUser()
	recipient.Alias = alias
	recipient.Email = email

	if err := h.Notif.Signup.EmailConfirmCode(recipient, token.Code); err != nil {
		return 0, err
	}

	return token.ID, nil
}

// awaitConfirmation remembers which code this browser is waiting for and goes
// to the page for typing it. tokenID is zero when nothing was sent.
func (h *signupHandler) awaitConfirmation(c echo.Context, email string, tokenID int64) error {
	step := codeStep{
		Subject:   email,
		TokenID:   tokenID,
		ExpiresAt: time.Now().Add(signupConfirmTTL),
	}

	if err := h.writeCodeStep(c, sessionKeySignupConfirm, step, signupConfirmTTL); err != nil {
		log.Println("signup confirm: could not remember the pending code:", err)
		return h.renderError(c, "Check Your Email",
			"We sent you a code, but could not open the page for it. "+
				"Please ask for a new code from the sign-in page.")
	}

	return c.Redirect(http.StatusSeeOther, "/signup/confirm")
}

func (h *signupHandler) confirmContext(c echo.Context, step *codeStep, errMsg string) map[string]any {
	return map[string]any{
		"Email":     step.Subject,
		"Error":     errMsg,
		"CSRFToken": h.loginCSRFToken(c),
	}
}

// Confirm shows the page waiting for the code.
//
//	GET /signup/confirm
//
// With nothing pending, including when somebody arrives by an old
// confirmation link, it goes to the page that asks for the address.
func (h *signupHandler) Confirm(c echo.Context) error {
	step, err := h.readCodeStep(c, sessionKeySignupConfirm)
	if err != nil || step == nil {
		return c.Redirect(http.StatusFound, "/signup/confirm/resend")
	}

	return h.renderTemplate(c, "signup_confirm_code.html", h.confirmContext(c, step, ""))
}

// ConfirmCode checks the code and marks the address confirmed.
//
//	POST /signup/confirm
func (h *signupHandler) ConfirmCode(c echo.Context) error {
	step, err := h.readCodeStep(c, sessionKeySignupConfirm)
	if err != nil || step == nil {
		return c.Redirect(http.StatusSeeOther, "/signup/confirm/resend?error=expired")
	}

	req := &requests.CodeForm{}
	if err := c.Bind(req); err != nil {
		log.Println("signup confirm: form bind failed:", err)
	}

	if !h.loginCSRFValid(c, req.CSRFToken) {
		return h.renderTemplate(c, "signup_confirm_code.html",
			h.confirmContext(c, step, "This page expired. Please enter the code again."))
	}

	token, result, remaining := h.submitCode(c, sessionKeySignupConfirm, step,
		constants.TokenTypeSignupConfirmCode, req.Code)

	switch result {
	case codeMalformed:
		return h.renderTemplate(c, "signup_confirm_code.html",
			h.confirmContext(c, step, "Please enter the 6-digit code."))
	case codeWrong:
		return h.renderTemplate(c, "signup_confirm_code.html",
			h.confirmContext(c, step, wrongCodeMessage(remaining)))
	case codeDead:
		return c.Redirect(http.StatusSeeOther, "/signup/confirm/resend?error=expired")
	}

	// A request to join and an account can never share an address, so
	// whichever one this address belongs to is what is being confirmed.
	sr, err := h.Repos.SignupRequests.ReadByEmail(token.RecipientEmail)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Println("signup confirm: signup request lookup failed:", err)
	}
	if err == nil && sr != nil {
		return h.confirmSignupRequest(c, sr, token)
	}

	user, err := h.Repos.Users.ReadByEmail(token.RecipientEmail)
	if err != nil || user == nil {
		log.Println("signup confirm: account lookup failed:", err)
		_ = h.Session.Delete(c, sessionKeySignupConfirm)
		return h.renderError(c, "Account Not Found",
			"We could not find an account for this code.")
	}

	// Used up before the account is touched, so the same code cannot
	// confirm twice or welcome twice.
	if !h.consumeCode(token, user.ID) {
		_ = h.Session.Delete(c, sessionKeySignupConfirm)
		return c.Redirect(http.StatusSeeOther, "/signup/confirm/resend?error=expired")
	}
	_ = h.Session.Delete(c, sessionKeySignupConfirm)

	if !user.IsEmailVerified() {
		if err := h.Repos.Users.MarkEmailVerified(user.ID); err != nil {
			log.Println("signup confirm: could not mark verified:", err)
			return h.renderError(c, "Something Went Wrong",
				"We could not confirm your address. Please ask for a new code.")
		}

		// The welcome, now that there is an address known to reach them.
		// Off the request path: a slow mail server must not hold the page.
		welcomeUser := *user
		go func() {
			if err := h.Notif.Signup.EmailSignup(&welcomeUser); err != nil {
				log.Println("signup confirm: welcome email failed:", err)
			}
		}()
	}

	return h.renderTemplate(c, "signup_confirm_success.html", map[string]any{
		"Email": user.Email,
	})
}

// confirmSignupRequest moves a request to join into the administrators'
// queue. Until its address is confirmed nobody is asked to read it.
func (h *signupHandler) confirmSignupRequest(
	c echo.Context,
	sr *entities.SignupRequest,
	token *entities.Token,
) error {
	if !h.consumeCode(token, 0) {
		_ = h.Session.Delete(c, sessionKeySignupConfirm)
		return c.Redirect(http.StatusSeeOther, "/signup/confirm/resend?error=expired")
	}
	_ = h.Session.Delete(c, sessionKeySignupConfirm)

	if sr.IsDecided() {
		return h.renderError(c, "Already Reviewed",
			"This request has already been reviewed. If you believe that "+
				"is a mistake, please contact the site administrators.")
	}

	if !sr.IsEmailVerified() {
		if err := h.Repos.SignupRequests.MarkEmailVerified(sr.ID); err != nil {
			log.Println("signup confirm: could not verify the signup request:", err)
			return h.renderError(c, "Something Went Wrong",
				"We could not confirm your address. Please ask for a new code.")
		}

		// Now, and not when the request was made: an unconfirmed request
		// is one nobody can even receive mail at.
		notifyRequest := *sr
		go func() {
			if err := h.Notif.Signup.EmailSignupRequestReceived(&notifyRequest); err != nil {
				log.Println("signup confirm: notifying administrators failed:", err)
			}
		}()
	}

	return h.renderTemplate(c, "signup_request_sent.html", map[string]any{
		"Email": sr.Email,
	})
}

// ResendForm asks which address to send a code to.
//
//	GET /signup/confirm/resend
func (h *signupHandler) ResendForm(c echo.Context) error {
	data := map[string]any{"CSRFToken": h.loginCSRFToken(c)}
	if c.QueryParam("error") == "expired" {
		data["Error"] = "That code is no longer valid. Enter your address and we will send a new one."
	}

	return h.renderTemplate(c, "signup_confirm_resend.html", data)
}

// Resend sends a new code and goes to the page waiting for it.
//
//	POST /signup/confirm/resend
//
// From the address form, or from the "send a new code" button on the code
// page, which sends no address: the one the browser is waiting on is used.
//
// The answer is the same whether or not anything was sent.
func (h *signupHandler) Resend(c echo.Context) error {
	req := &requests.ConfirmResend{}
	if err := c.Bind(req); err != nil {
		log.Println("signup resend: form bind failed:", err)
	}

	if !h.loginCSRFValid(c, req.CSRFToken) {
		return h.renderTemplate(c, "signup_confirm_resend.html", map[string]any{
			"Error":     "This page expired. Please try again.",
			"Email":     req.Email,
			"CSRFToken": h.loginCSRFToken(c),
		})
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		if step, err := h.readCodeStep(c, sessionKeySignupConfirm); err == nil && step != nil {
			email = step.Subject
		}
	}

	req.Email = email
	if err := c.Validate(req); err != nil {
		return h.renderTemplate(c, "signup_confirm_resend.html", map[string]any{
			"Error":     "Please enter a valid email address.",
			"Email":     email,
			"CSRFToken": h.loginCSRFToken(c),
		})
	}

	// Both charged before anything is looked up, and both every time.
	limitedByAddress := h.rateLimitExceeded(rateLimitScopeSignupConfirm, email)
	limitedByIP := h.rateLimitExceeded(rateLimitScopePasswordIP, getClientIP(c))

	var tokenID int64
	if !limitedByAddress && !limitedByIP {
		tokenID = h.resendConfirmationCode(email)
	}

	return h.awaitConfirmation(c, email, tokenID)
}

// resendConfirmationCode sends a code to an address that still needs one, and
// returns its token, or zero when nothing was sent.
//
// Nothing goes to an address that is already confirmed, or that nobody signed
// up with. Otherwise this would be a way to have the site mail anybody on
// demand.
func (h *signupHandler) resendConfirmationCode(email string) int64 {
	sr, err := h.Repos.SignupRequests.ReadByEmail(email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Println("signup resend: signup request lookup failed:", err)
	}
	if err == nil && sr != nil {
		if sr.IsDecided() || sr.IsEmailVerified() {
			return 0
		}
		tokenID, err := h.sendConfirmationCode(sr.Email, 0, sr.Username)
		if err != nil {
			log.Println("signup resend:", err)
			return 0
		}
		return tokenID
	}

	user, err := h.Repos.Users.ReadByEmail(email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Println("signup resend: lookup failed:", err)
	}
	if user == nil || user.IsEmailVerified() {
		return 0
	}

	tokenID, err := h.sendConfirmationCode(user.Email, user.ID, user.Alias)
	if err != nil {
		log.Println("signup resend:", err)
		return 0
	}

	return tokenID
}
