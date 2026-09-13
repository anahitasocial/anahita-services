package handlers

import (
	"log"
	"net/http"
	"time"

	"auth/constants"
	"auth/data/entities"
	"auth/data/repositories"

	"github.com/labstack/echo/v4"
)

// ---------------------------------------------------------------------
// Account restore.
//
// Deleting a profile no longer destroys it: it is marked, disabled and swept
// after a grace period. This is the way back, and without it the grace period
// would be a delay rather than a chance — the account would sit recoverable in
// the database with nothing able to recover it.
//
// Restoring happens at LOGIN rather than through a link in the confirmation
// email, and that is the deliberate part. The person has to know the password
// either way, so a link would add a step without adding proof; and it means
// somebody who changed their mind three weeks later does the obvious thing —
// tries to sign in — and is met with the offer rather than a rejection.
// ---------------------------------------------------------------------

// sessionKeyPendingRestore holds the account awaiting a restore decision.
//
// Its own key, so that arriving at the restore form cannot be confused with a
// half-finished TOTP login. Same one-shot discipline: written after the
// password verifies, cleared when the decision is made.
const sessionKeyPendingRestore = "oauth_pending_restore"

// renderRestoreForm offers the account back.
//
// Reached only after the password has been verified — see AddCredentials. The
// email is stashed rather than posted back in a hidden field so the form cannot
// be pointed at a different account than the one whose password was just
// checked.
func (h *oauthHandler) renderRestoreForm(c echo.Context, user *entities.User) error {
	if _, err := h.Session.Add(c, sessionKeyPendingRestore, user.Email); err != nil {
		log.Println("restore: pending state write failed:", err)
		return h.renderLoginError(c, "Could not continue. Please try again.")
	}

	return h.renderTemplate(c, "account_restore.html", map[string]any{
		"PurgeDate": h.purgeDate(user),
	})
}

// purgeDate is when the sweeper will reach this account, formatted for a person
// to read.
//
// Empty when the deletion timestamp is missing, which the template treats as
// "soon" rather than printing a date derived from a zero time — a page telling
// somebody their account will be erased in January 0001 is worse than one that
// declines to be specific.
func (h *oauthHandler) purgeDate(user *entities.User) string {
	if user == nil || user.DeletedAt == nil {
		return ""
	}

	return user.DeletedAt.
		Add(time.Duration(constants.AccountGraceDays) * 24 * time.Hour).
		Format("2 January 2006")
}

// ReadRestore renders the restore prompt on its own URL.
//
//	GET /oauth/login/restore
//
// Exists so a refresh of the restore page does not silently become a login
// attempt with no credentials. Falls back to the login form when there is no
// pending decision.
func (h *oauthHandler) ReadRestore(c echo.Context) error {
	email, err := h.Session.Read(c, sessionKeyPendingRestore)
	if err != nil || email == "" {
		return c.Redirect(http.StatusFound, "/oauth/login")
	}

	user, err := h.Repos.Users.ReadByEmail(email)
	if err != nil || user == nil {
		_ = h.Session.Delete(c, sessionKeyPendingRestore)
		return c.Redirect(http.StatusFound, "/oauth/login")
	}

	return h.renderTemplate(c, "account_restore.html", map[string]any{
		"PurgeDate": h.purgeDate(user),
	})
}

// AddRestore restores the account and sends the person back to sign in.
//
//	POST /oauth/login/restore
//
// Deliberately does NOT log them in. Restoring re-enables an account whose
// sessions, passkeys and refresh tokens were all revoked when it was deleted —
// signing in from here would skip TOTP and the account-state checks that the
// ordinary login path applies, on the one account where those checks matter
// most. The person signs in again, which takes seconds and proves the account
// works.
func (h *oauthHandler) AddRestore(c echo.Context) error {
	email, err := h.Session.Read(c, sessionKeyPendingRestore)
	if err != nil || email == "" {
		return c.Redirect(http.StatusFound, "/oauth/login")
	}

	// One-shot: cleared before the work, so a failure cannot be retried by
	// replaying the form.
	_ = h.Session.Delete(c, sessionKeyPendingRestore)

	user, err := h.Repos.Users.ReadByEmail(email)
	if err != nil || user == nil {
		log.Println("restore: user lookup failed:", err)
		return h.renderLoginError(c, "Could not restore your account. Please try again.")
	}

	restored, err := h.Repos.Users.Restore(user.ID)
	if err != nil {
		log.Println("restore: failed:", err)
		return h.renderLoginError(c, "Could not restore your account. Please try again.")
	}

	// Nothing to restore. Either the sweeper reached it between the password
	// check and this request, or it was never deleted — both mean the account
	// is not coming back this way.
	if !restored {
		log.Printf("restore: nothing to restore (person=%d)", user.ID)
		return h.renderError(c, "Account Unavailable",
			"This account is no longer available. If you believe this is a mistake, please contact support.")
	}

	// Audit. The third of the three actions that make up one account's story —
	// scheduled, then either purged or restored — and the only one written by
	// this service, because restoring is the step that happens at login.
	//
	// Best-effort: the account is already back, and failing the request over
	// the audit row would tell the person the restore had not worked.
	if err := h.Repos.NodeDeletionLogs.Add(
		user.ID, user.Username, user.PersonType, user.ID,
		repositories.DeletionActionRestored,
	); err != nil {
		log.Println("restore: audit write failed:", err)
	}

	log.Printf("restore: account restored (person=%d)", user.ID)

	go func() {
		if err := h.Notif.AccountDelete.EmailRestored(user); err != nil {
			log.Println(err)
		}
	}()

	return h.renderTemplate(c, "account_restore_success.html", map[string]any{})
}
