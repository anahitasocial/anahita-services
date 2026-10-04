package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"auth/constants"
	"auth/requests"

	"github.com/labstack/echo/v4"
)

type emailChangeHandler struct {
	*Env
}

func newEmailChangeHandler(env *Env) *emailChangeHandler {
	return &emailChangeHandler{env}
}

// ---------------------------------------------------------------------
// Email change — the only way an account's email address moves.
//
// The ordinary person edit endpoint in organization-service rejects an
// `email` field outright, from admins and from the person themselves.
// Email is what password reset delivers to, so whoever can set it can
// take the account; there is no version of "just let admins fix a typo"
// that is not also "let admins become their colleagues".
//
// Two steps:
//
//  1. PATCH /email/change — authenticated, re-authenticates with the
//     current password, and mails a confirmation link to the CURRENT
//     address.
//  2. GET/POST /email/change/confirm — public, token-gated, applies the
//     change and revokes everything.
//
// The confirmation goes to the old address, not the new one. An
// attacker inside a live session has the session and the password only
// if they also phished it; what they reliably do NOT have is the
// person's inbox. Requiring the old inbox to approve is what keeps a
// session hijack from becoming a permanent takeover.
// ---------------------------------------------------------------------

// Edit starts an email change.
//
//	PATCH /email/change
//
// Status codes:
//
//	204 — confirmation sent to the current address
//	400 — bad body or validation failure
//	403 — no recent step-up; the client re-authenticates and retries
//	409 — the requested address is already in use
//	500 — gRPC, token, or hashing failure
//
// 409 on a taken address is a deliberate exception to the enumeration
// caution applied elsewhere. This endpoint requires an authenticated
// session AND the account password, so the caller has already proved
// who they are; refusing to say why the change failed would only
// confuse a legitimate person. The unauthenticated surfaces
// (/password/forgot, and the confirm handler below) still swallow it.
func (h *emailChangeHandler) Edit(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || viewer.ID == 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	req := &requests.EmailChangeEdit{}
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	newEmail := strings.ToLower(strings.TrimSpace(req.Email))

	// Re-read rather than trusting the cached viewer — the access token
	// does not carry the email, and the password check below needs the
	// stored hash. Matches what the password change handler does.
	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || user == nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.requireStepUp(user.ID); err != nil {
		log.Println("email change:", err)
		return c.NoContent(reauthStatus(err))
	}

	// Asking to change to the address you already have is a no-op, not
	// an error worth a token and an email.
	if newEmail == strings.ToLower(user.Email) {
		return c.NoContent(http.StatusNoContent)
	}

	isTaken, err := h.Repos.Users.IsEmail(newEmail)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if isTaken {
		return c.NoContent(http.StatusConflict)
	}

	// A code, sent to the CURRENT address. The current inbox is what has to
	// approve the move, so a stolen session alone cannot repoint the
	// account. The token remembers where the account is moving to; the
	// person types the code back into the settings page they are on.
	//
	// Issuing a new one retires any earlier request, so only the latest
	// address asked for can be confirmed.
	token, err := h.issueCode(codeRequest{
		Email:    user.Email,
		PersonID: user.ID,
		Type:     constants.TokenTypeEmailChangeCode,
		Channel:  constants.CodeChannelEmail,
		Lifetime: constants.TokenExpiryTimeCode,
		NewEmail: newEmail,
	})
	if err != nil {
		log.Println("email change:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// One step-up buys one request. Asking again, for another address or
	// another code, means proving it is you again, which is the limit on
	// how often this can mail the account.
	h.clearStepUp(user.ID)

	recipient, code := *user, token.Code
	go func() {
		if err := h.Notif.EmailChange.EmailCode(&recipient, newEmail, code); err != nil {
			log.Println("email change: email failed:", err)
		}
	}()

	return c.NoContent(http.StatusNoContent)
}

// Confirm applies a pending email change, given the code that was sent to
// the current address.
//
//	POST /email/change/confirm   { "code": "123456" }
//
// Signed in, and called by the web and mobile apps from the screen where the
// change was started. Answers:
//
//	204  changed. Every session, this one included, is signed out
//	400  not six digits. Nothing was counted
//	409  the new address has been taken in the meantime
//	410  no change is pending, or its code expired, was replaced, or ran
//	     out of tries. Start again
//	422  wrong code. The body says how many tries are left
func (h *emailChangeHandler) Confirm(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || viewer.ID == 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	req := &requests.EmailChangeConfirm{}
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	code := strings.TrimSpace(req.Code)
	if len(code) != constants.OTPCodeLength || !isAllDigits(code) {
		return c.NoContent(http.StatusBadRequest)
	}

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || user == nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Found by whose it is, never by the code: this person's one live
	// email-change code.
	pending, err := h.Repos.Tokens.ReadLiveCode(constants.TokenTypeEmailChangeCode, user.Email)
	if err != nil || pending == nil {
		return c.NoContent(http.StatusGone)
	}

	token, remaining, err := h.verifyCode(pending.ID, constants.TokenTypeEmailChangeCode, code)
	if err != nil {
		if errors.Is(err, errCodeMismatch) {
			return c.JSON(http.StatusUnprocessableEntity, map[string]int{"remaining": remaining})
		}
		return c.NoContent(http.StatusGone)
	}

	// Somebody else may have taken the address in the last fifteen minutes.
	// The code is left usable: nothing was changed, and the answer is the
	// same however often it is asked.
	isTaken, err := h.Repos.Users.IsEmail(token.NewEmail)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if isTaken {
		return c.NoContent(http.StatusConflict)
	}

	// Used up before the address moves, so two submissions cannot both
	// run the change.
	if !h.consumeCode(token, user.ID) {
		return c.NoContent(http.StatusGone)
	}

	oldEmail := user.Email

	if err := h.Repos.Users.EditEmail(token.NewEmail, user.ID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	user.Email = token.NewEmail

	// The address is what recovery delivers to, so changing it is treated
	// like taking the account back: every passkey and every session goes,
	// and the person signs in again with the new address.
	if revokedKeys, err := h.Repos.WebAuthnCredentials.DeleteByPersonID(user.ID); err != nil {
		log.Println("email change: passkey revocation failed:", err)
	} else if revokedKeys > 0 {
		log.Printf("email change: revoked %d passkey(s) (person=%d)", revokedKeys, user.ID)
	}

	authLogHandler := newAuthLogHandler(h.Env)
	if revokedSessions, err := authLogHandler.DeleteAllSessions(user); err != nil {
		log.Println("email change: session revocation failed:", err)
	} else if revokedSessions > 0 {
		log.Printf("email change: revoked %d session(s) (person=%d)", revokedSessions, user.ID)
	}

	oauthHandler := newOAuthHandler(h.Env)
	if revokedIdP, err := oauthHandler.revokeAllIdPSessions(user); err != nil {
		log.Println("email change: IdP session revocation failed:", err)
	} else if revokedIdP > 0 {
		log.Printf("email change: revoked %d IdP session(s) (person=%d)", revokedIdP, user.ID)
	}

	// Both addresses are told. The old one is how an owner who did not do
	// this finds out.
	notifyUser := *user
	go func() {
		if err := h.Notif.EmailChange.EmailChangedOld(&notifyUser, oldEmail); err != nil {
			log.Println(err)
		}
		if err := h.Notif.EmailChange.EmailChangedNew(&notifyUser); err != nil {
			log.Println(err)
		}
	}()

	return c.NoContent(http.StatusNoContent)
}
