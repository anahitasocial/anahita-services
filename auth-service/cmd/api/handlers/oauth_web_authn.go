package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"auth/constants"
	"auth/data/entities"
	"auth/data/services"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/labstack/echo/v4"
)

// Session key for the pending passkey login ceremony. Same lifetime
// discipline as sessionKeyPendingTOTP: short-lived, one-shot, cleared
// before verification so a failed assertion can't be replayed.
const sessionKeyPasskeyLogin = "oauth_pending_passkey"

type pendingPasskeyLogin struct {
	Session   webauthn.SessionData `json:"session"`
	ExpiresAt time.Time            `json:"expires_at"`
}

// AddPasskeyLoginBegin handles POST /oauth/login/passkey/begin
//
// Unauthenticated by design — this is step one of signing in. Returns
// the assertion options for a discoverable (usernameless) login: no
// email is submitted, and the authenticator decides which account it
// holds a credential for.
//
// That property is also why this endpoint leaks nothing. The
// challenge is random and account-independent, so an attacker calling
// it repeatedly learns only that the endpoint exists.
func (h *oauthHandler) AddPasskeyLoginBegin(c echo.Context) error {
	w, err := webAuthnRP(h.Env)
	if err != nil {
		log.Println("passkey login begin config:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// User verification is fixed at Required in the RP config. That
	// is what makes this path two-factor — possession of the device
	// plus the biometric or PIN gesture — and therefore what lets it
	// skip the TOTP step the password path still requires.
	options, session, err := w.BeginDiscoverableLogin()
	if err != nil {
		log.Println("passkey login begin ceremony:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	pending := pendingPasskeyLogin{
		Session:   *session,
		ExpiresAt: time.Now().Add(constants.WebAuthnCeremonyTTL),
	}
	if err := h.writePendingPasskeyLogin(c, pending); err != nil {
		log.Println("passkey login begin state write:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, options)
}

// AddPasskeyLoginFinish handles POST /oauth/login/passkey/finish
//
// Verifies the assertion, applies the same account-state gates the
// password path applies, and finalizes the IdP session.
//
// Returns JSON with a redirect target rather than a 302. The browser
// reaches this endpoint via fetch(), and fetch follows redirects
// transparently — a 302 here would be chased in the background and
// the person would never leave the login page. The client navigates
// on redirect_to instead.
func (h *oauthHandler) AddPasskeyLoginFinish(c echo.Context) error {
	pending, err := h.readPendingPasskeyLogin(c)
	if err != nil || pending == nil {
		log.Println("passkey login finish: no pending ceremony:", err)
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "ceremony_expired",
			"message": "That took too long. Try signing in again.",
		})
	}

	// One-shot: clear before verifying.
	_ = h.Session.Delete(c, sessionKeyPasskeyLogin)

	w, err := webAuthnRP(h.Env)
	if err != nil {
		log.Println("passkey login finish config:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// resolved is captured by the discoverable-user handler below.
	// The library calls the handler mid-verification to fetch the
	// account the assertion claims, then finishes checking the
	// signature against that account's public key. We keep a
	// reference so the post-verification gates have the person
	// without a second lookup.
	var (
		resolved     *entities.User
		resolvedCred *entities.WebAuthnCredential
	)

	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		stored, err := h.Repos.WebAuthnCredentials.ReadByCredentialID(rawID)
		if err != nil {
			return nil, err
		}

		// The user handle comes from the authenticator, the owner ID
		// from our row. They were written together at enrolment and
		// must still agree — a mismatch means the assertion is
		// pointing at one account with another account's credential.
		//
		// Decoded rather than byte-compared so a handle of the wrong
		// length fails with a clear reason instead of looking like an
		// ordinary mismatch.
		handleUserID, err := entities.UserIDFromWebAuthnHandle(userHandle)
		if err != nil {
			return nil, errors.New("passkey: malformed user handle")
		}
		if handleUserID != stored.UserID {
			return nil, errors.New("passkey: user handle does not match credential owner")
		}

		user, err := h.Repos.Users.ReadByID(stored.UserID)
		if err != nil || user == nil {
			return nil, errors.New("passkey: credential owner not found")
		}

		all, err := h.Repos.WebAuthnCredentials.Browse(user.ID)
		if err != nil {
			return nil, err
		}

		resolved = user
		resolvedCred = stored

		return services.NewWebAuthnUser(user, all), nil
	}

	credential, err := w.FinishDiscoverableLogin(handler, pending.Session, c.Request())
	if err != nil {
		// Deliberately generic. The library's error text names the
		// specific check that failed, which is useful in logs and a
		// gift to anyone probing the endpoint.
		log.Println("passkey login finish verify:", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error":   "assertion_failed",
			"message": "That passkey wasn't accepted. Try again, or sign in with your password.",
		})
	}

	if resolved == nil || resolvedCred == nil {
		log.Println("passkey login finish: verification passed but no person resolved")
		return c.NoContent(http.StatusInternalServerError)
	}

	user := resolved

	// Account-state gates. Same set the password path applies in
	// AddCredentials, re-run here because a passkey login skips that
	// handler entirely — a disabled account must not slip through
	// just because it took the other door.
	if !user.Enabled {
		log.Printf("passkey: rejected disabled person (person=%d)", user.ID)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error":   "assertion_failed",
			"message": "That passkey wasn't accepted. Try again, or sign in with your password.",
		})
	}

	// Fold the authenticator's post-assertion state back onto the row
	// and stamp last_used_at. Best-effort: the person has already
	// authenticated, and failing the sign-in over a bookkeeping write
	// would be the wrong trade.
	stored := resolvedCred
	stored.ApplyAuthenticatorUpdate(credential)
	if err := h.Repos.WebAuthnCredentials.TouchAfterLogin(stored); err != nil {
		log.Println("passkey: failed to update credential after login:", err)
	}

	if stored.CloneWarning {
		// Recorded, not enforced. Platform authenticators that sync
		// legitimately report a flat counter, so blocking here would
		// lock out ordinary iCloud and Google passkey users. The
		// signal surfaces in the credentials list instead.
		log.Printf("passkey: clone warning on credential (person=%d credential=%d)",
			user.ID, stored.ID)
	}

	log.Printf("passkey: authenticated (person=%d credential=%d)", user.ID, stored.ID)

	return h.finalizePasskeyLogin(c, user)
}

// finalizePasskeyLogin writes the IdP session and returns the
// post-login destination as JSON.
//
// Mirrors finalizeLogin in oauth_login.go, which redirects instead.
// The two could share a writeViewerSession helper; kept separate here
// to avoid editing the existing login flow as part of this change.
func (h *oauthHandler) finalizePasskeyLogin(c echo.Context, user *entities.User) error {
	if err := h.writeViewerSession(c, user); err != nil {
		log.Println("passkey:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	target, ok := h.consumeReturnTo(c)
	if !ok {
		target = h.Config.GetFrontendURL()
		log.Printf("passkey: no return_to, falling back to %s (person=%d)", target, user.ID)
	}

	return c.JSON(http.StatusOK, map[string]string{"redirect_to": target})
}

func (h *oauthHandler) writePendingPasskeyLogin(c echo.Context, pending pendingPasskeyLogin) error {
	raw, err := json.Marshal(pending)
	if err != nil {
		return err
	}
	_, err = h.Session.Add(c, sessionKeyPasskeyLogin, string(raw))
	return err
}

func (h *oauthHandler) readPendingPasskeyLogin(c echo.Context) (*pendingPasskeyLogin, error) {
	raw, err := h.Session.Read(c, sessionKeyPasskeyLogin)
	if err != nil || raw == "" {
		return nil, err
	}

	var pending pendingPasskeyLogin
	if err := json.Unmarshal([]byte(raw), &pending); err != nil {
		return nil, err
	}

	if time.Now().After(pending.ExpiresAt) {
		_ = h.Session.Delete(c, sessionKeyPasskeyLogin)
		return nil, nil
	}

	return &pending, nil
}
