package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"

	"auth/constants"
	"auth/data/entities"
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

	token := entities.NewToken()
	token.Status = constants.TokenStatusPending
	token.ExpiredAfter = constants.TokenExpiryTimeEmailChange
	token.Type = constants.TokenTypeEmailChange
	// Recipient is where the mail goes: the CURRENT address. NewEmail is
	// the destination. Swapping these two is the one mistake that turns
	// this flow from a control into a liability.
	token.RecipientEmail = user.Email
	token.NewEmail = newEmail
	token.IsUsed = false
	token.CreatedAt = time.Now()
	token.CreatedBy = user.ID
	token.UpdatedAt = time.Now()
	token.UpdatedBy = user.ID

	if err := token.GenerateToken(); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	stored, err := h.Repos.Tokens.Add(token)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// One step-up authorises one credential change. Without this, a
	// single passcode would license changing the email, then the phone,
	// then the password for the rest of the window.
	h.clearStepUp(user.ID)

	go func() {
		if err := h.Notif.EmailChange.EmailConfirm(user, stored); err != nil {
			log.Println(err)
		}
	}()

	return c.NoContent(http.StatusNoContent)
}

// ConfirmForm validates the token and renders the confirmation page.
//
//	GET /email/change/confirm?token=...
//
// The page shows the new address and asks the person to re-type it.
// That is the typo guard: confirming from the old inbox is what makes
// the change safe, but it also means a mistyped destination would move
// the account somewhere the person cannot read. Re-typing while looking
// at the address costs one field and removes the lockout.
func (h *emailChangeHandler) ConfirmForm(c echo.Context) error {
	tokenBody := c.QueryParam("token")
	if tokenBody == "" {
		return h.renderError(c, "Invalid Link",
			"This email change link is not valid. Please start the change again from your account settings.")
	}

	token, err := h.Repos.Tokens.ReadByBody(tokenBody)
	if err != nil {
		log.Println(err)
		return h.renderError(c, "Invalid Link",
			"This email change link is not valid. Please start the change again from your account settings.")
	}

	if token.Type != constants.TokenTypeEmailChange {
		return h.renderError(c, "Invalid Link",
			"This link is not valid for changing your email address.")
	}

	switch token.Status {
	case constants.TokenStatusUsed:
		return h.renderError(c, "Link Already Used",
			"This email change has already been confirmed. If you still need to change your address, start again from your account settings.")
	case constants.TokenStatusExpired:
		return h.renderError(c, "Link Expired",
			"This email change link has expired. Please start the change again from your account settings.")
	}

	return h.renderTemplate(c, "email_change_confirm.html", map[string]any{
		"Token":    token.Body,
		"NewEmail": token.NewEmail,
	})
}

// Confirm applies the email change.
//
//	POST /email/change/confirm
//
// On success the address is updated, the token is consumed, and the
// person's passkeys and every active session are revoked — the same
// revocations the password reset flow performs, for the same reason.
// Changing the address changes who can recover the account, so anything
// that outlives the change is a way back in for whoever held it. Both
// the old and new addresses are notified.
func (h *emailChangeHandler) Confirm(c echo.Context) error {
	req := &requests.EmailChangeConfirm{}
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return h.renderError(c, "Invalid Link",
			"This email change link is not valid. Please start the change again from your account settings.")
	}

	token, err := h.Repos.Tokens.ReadByBody(req.Token)
	if err != nil {
		log.Println(err)
		return h.renderError(c, "Invalid Link",
			"This email change link is not valid. Please start the change again from your account settings.")
	}

	if token.Type != constants.TokenTypeEmailChange {
		return h.renderError(c, "Invalid Link",
			"This link is not valid for changing your email address.")
	}

	if token.Status != constants.TokenStatusPending {
		return h.renderError(c, "Link No Longer Valid",
			"This email change link has been used or has expired. Please start the change again from your account settings.")
	}

	// The typed address must match the one the token was issued for.
	//
	// This is a typo check, not an authorisation check — the token
	// already carries the destination, and the person cannot redirect
	// the change by typing something else. Re-rendering the form rather
	// than erroring out lets them correct a slip without starting over.
	if !strings.EqualFold(strings.TrimSpace(req.Email), token.NewEmail) {
		return h.renderTemplate(c, "email_change_confirm.html", map[string]any{
			"Token":    token.Body,
			"NewEmail": token.NewEmail,
			"Error":    "That does not match the new address. Please type it exactly as shown above.",
		})
	}

	user, err := h.Repos.Users.ReadByEmail(token.RecipientEmail)
	if err != nil || user == nil {
		log.Println(err)
		return h.renderError(c, "Account Not Found",
			"We could not find an account associated with this link.")
	}

	// Re-check between issuing and confirming. The address may have been
	// claimed by somebody else in the window, and the DB constraint
	// would otherwise surface as an opaque 500 on a page the person
	// cannot act on.
	isTaken, err := h.Repos.Users.IsEmail(token.NewEmail)
	if err != nil {
		log.Println(err)
		return h.renderError(c, "Something Went Wrong",
			"We could not change your email address. Please try again.")
	}
	if isTaken {
		return h.renderError(c, "Address Unavailable",
			"That email address is now in use by another account. Please start the change again with a different address.")
	}

	oldEmail := user.Email

	if err := h.Repos.Users.EditEmail(token.NewEmail, user.ID); err != nil {
		log.Println(err)
		return h.renderError(c, "Something Went Wrong",
			"We could not change your email address. Please try again.")
	}

	// Reflect the change locally so the notifiers below describe what
	// actually happened rather than the pre-change state.
	user.Email = token.NewEmail

	// Consume the token. If this fails the address is already changed,
	// so log and continue — the status gate catches a replay on the
	// next read.
	token.IsUsed = true
	token.UpdatedAt = time.Now()
	token.UpdatedBy = user.ID
	if _, err := h.Repos.Tokens.Edit(token); err != nil {
		log.Println(err)
	}

	// --- Revoke what an address change does not touch ---------------
	//
	// Same three revocations as a password reset, and for the same
	// reason: none of them are derived from the email address, so all
	// three survive the change untouched. If the change was made by
	// somebody who should not have, leaving them in place means the
	// account reads as recovered while still being open.
	//
	// All best-effort. The address change has already succeeded, and
	// failing here would leave the person unable to recover at all.

	if revokedKeys, err := h.Repos.WebAuthnCredentials.DeleteByPersonID(user.ID); err != nil {
		log.Println("email change: passkey revocation failed:", err)
	} else if revokedKeys > 0 {
		log.Printf("email change: revoked %d passkey(s) (person=%d)", revokedKeys, user.ID)
	}

	authlogHandler := newAuthLogHandler(h.Env)
	if revokedSessions, err := authlogHandler.DeleteAllSessions(user); err != nil {
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

	// Both addresses, old first. The old address is the one that needs
	// to know, because it is losing the ability to recover the account.
	go func() {
		if err := h.Notif.EmailChange.EmailChangedOld(user, oldEmail); err != nil {
			log.Println(err)
		}
		if err := h.Notif.EmailChange.EmailChangedNew(user); err != nil {
			log.Println(err)
		}
	}()

	return h.renderTemplate(c, "email_change_success.html", map[string]any{
		"NewEmail":    user.Email,
		"FrontendURL": h.Config.GetFrontendURL(),
	})
}
