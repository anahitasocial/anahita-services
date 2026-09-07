package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"auth/constants"
	"auth/data/services"
	"auth/requests"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/labstack/echo/v4"
)

// ---------------------------------------------------------------------
// Step-up re-authentication (authenticated, JSON API)
// ---------------------------------------------------------------------
//
// These endpoints change nothing. They prove the caller is who the
// session says they are, and record the short-lived marker that
// credential-mutating endpoints check — see reauth_helper.go.
//
// Splitting the proof from the mutation is what lets a passkey stand in
// for a password. A WebAuthn assertion cannot travel in a JSON body
// beside a new password, but it can be what wrote the marker a minute
// earlier. Before this existed, /password/change took the current
// password inline, which meant someone who signs in by passkey daily —
// and may never have typed their password — had no route to change it
// except forgot-password, which deletes every passkey on the account.
//
// Every route is behind the standard authorize middleware. The session
// is the floor, not the proof: these endpoints exist precisely because
// holding a session is not enough on its own.

// sessionKeyReauthPasskey holds the pending assertion ceremony.
//
// Its own key rather than sharing sessionKeyPasskeyLogin. The two
// ceremonies mean different things — one signs you in, one authorises a
// credential change — and a shared key would let a challenge issued for
// one be finished by the other. Starting a step-up mid-login would also
// silently clobber the login's state.
const sessionKeyReauthPasskey = "reauth_passkey"

type pendingReauthPasskey struct {
	Session   webauthn.SessionData `json:"session"`
	ExpiresAt time.Time            `json:"expires_at"`
	// UserID pins the ceremony to the account that began it. FinishLogin
	// verifies the assertion against whatever user we hand it, so
	// without this a session that changed hands between begin and finish
	// would have its challenge checked against the wrong account's keys.
	UserID int64 `json:"user_id"`
}

type reauthHandler struct {
	*Env
}

func newReauthHandler(env *Env) *reauthHandler {
	return &reauthHandler{env}
}

// AddPassword handles POST /reauth/password
//
// Re-authenticates with the current password, plus a TOTP passcode when
// the account has one enrolled.
//
// Status codes, and the reason they are split this way: the dialog puts
// the error on the field that was actually wrong, so "password was
// wrong" and "code was wrong" cannot share a status.
//
//	204 — proved; the step-up marker is live for stepUpTTL
//	400 — bad body
//	401 — wrong password
//	403 — missing or invalid TOTP passcode
//	429 — too many attempts
//	500 — store, database, or crypto failure
func (h *reauthHandler) AddPassword(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || viewer.ID == 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	// Counted before anything else, including the bind. An attacker
	// probing passwords controls the body, so letting a malformed one
	// pass free would hand back an unmetered retry for the price of a
	// deliberately broken payload.
	subject := fmt.Sprintf("%d", viewer.ID)
	if exceeded, first := h.rateLimitCheck(rateLimitScopeReauth, subject); exceeded {
		// Tell the owner, once per window. Reaching this endpoint at all
		// requires a live session, so whoever is guessing already holds one —
		// this is not somebody knocking on the front door, it is somebody
		// inside trying the locks. Nothing has changed yet, which is exactly
		// why it is worth a message now.
		//
		// Only on the first crossing: see rateLimitCheck. The user is re-read
		// because the notification needs a name and an address, and the viewer
		// carries neither reliably.
		if first {
			go func() {
				user, err := h.Repos.Users.ReadByID(viewer.ID)
				if err != nil || user == nil {
					log.Println("reauth lockout notification: user lookup failed:", err)
					return
				}
				if err := h.Notif.Reauth.EmailLockedOut(user); err != nil {
					log.Println("reauth lockout notification failed:", err)
				}
			}()
		}

		return c.NoContent(http.StatusTooManyRequests)
	}

	req := &requests.ReauthPassword{}
	if err := c.Bind(req); err != nil {
		log.Println("reauth password bind:", err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println("reauth password validate:", err)
		return c.NoContent(http.StatusBadRequest)
	}

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || user == nil {
		log.Println("reauth password lookup:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if !user.CheckPassword(req.Password) {
		log.Printf("reauth: password rejected (person=%d)", user.ID)
		return c.NoContent(http.StatusUnauthorized)
	}

	// Second factor, when the account has one. Read from the account
	// rather than inferred from the payload: a client that omits the
	// passcode must not thereby opt out of the check.
	if user.TotpEnabled {
		if req.Passcode == "" {
			log.Printf("reauth: passcode required but absent (person=%d)", user.ID)
			return c.NoContent(http.StatusForbidden)
		}

		totpValidator := newTotpValidatorHandler(h.Env)
		ok, err := totpValidator.ValidateTotp(c, req.Passcode, user)
		if err != nil {
			log.Println("reauth totp validate:", err)
			return c.NoContent(http.StatusInternalServerError)
		}
		if !ok {
			log.Printf("reauth: passcode rejected (person=%d)", user.ID)
			return c.NoContent(http.StatusForbidden)
		}

		// TotpLastVerifiedAt is deliberately NOT stamped here, unlike
		// totpHandler.Verify. That field feeds User.IsMfaExpired, which
		// the LOGIN flow reads to decide whether to ask for a code at
		// all — so stamping it would let a step-up quietly extend how
		// long the next sign-in skips its second factor. A step-up
		// authorises one credential change; it should not also buy
		// freshness on a different gate.
	}

	if err := h.markStepUp(user.ID); err != nil {
		log.Println("reauth password mark:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Proved. The failures before it are no longer evidence of
	// anything, so the budget goes back.
	h.rateLimitReset(rateLimitScopeReauth, subject)

	log.Printf("reauth: password step-up granted (person=%d)", user.ID)

	return c.NoContent(http.StatusNoContent)
}

// AddPasskeyBegin handles POST /reauth/passkey/begin
//
// Returns assertion options scoped to this person's enrolled
// credentials. Takes no body — the account is the session's.
//
// Non-discoverable (BeginLogin, not BeginDiscoverableLogin) because we
// already know who is asking. Sending allowCredentials means the
// browser prompts for one of THIS person's passkeys rather than
// offering an account picker, which would be a strange thing to see
// while already signed in.
//
//	200 — assertion options
//	409 — no passkeys enrolled; the client falls back to the password
//	500 — store or ceremony failure
func (h *reauthHandler) AddPasskeyBegin(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || viewer.ID == 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || user == nil {
		log.Println("reauth passkey begin lookup:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	creds, err := h.Repos.WebAuthnCredentials.Browse(user.ID)
	if err != nil {
		log.Println("reauth passkey begin browse:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// 409 rather than 403: nothing is forbidden, the account simply has
	// no credential to assert. The dialog treats this as "the hint I was
	// rendered with was stale" and switches to the password form, so it
	// must be distinguishable from a failed assertion.
	if len(creds) == 0 {
		log.Printf("reauth: no passkeys enrolled (person=%d)", user.ID)
		return c.NoContent(http.StatusConflict)
	}

	w, err := webAuthnRP(h.Env)
	if err != nil {
		log.Println("reauth passkey begin config:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	options, session, err := w.BeginLogin(services.NewWebAuthnUser(user, creds))
	if err != nil {
		log.Println("reauth passkey begin ceremony:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	pending := pendingReauthPasskey{
		Session:   *session,
		ExpiresAt: time.Now().Add(constants.WebAuthnCeremonyTTL),
		UserID:    user.ID,
	}
	if err := h.writePendingReauthPasskey(c, pending); err != nil {
		log.Println("reauth passkey begin state write:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, options)
}

// AddPasskeyFinish handles POST /reauth/passkey/finish
//
// Verifies the assertion against the stashed challenge and records the
// step-up.
//
// A passkey stands alone as proof — the gesture guarding it, a
// fingerprint or face or device PIN, is already a second factor, and
// the relying party is configured with user verification Required. So
// there is no passcode step after this, even for a TOTP-enrolled
// account.
//
//	204 — proved
//	400 — no pending ceremony, or it expired
//	401 — the assertion did not verify
//	500 — store or ceremony failure
func (h *reauthHandler) AddPasskeyFinish(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || viewer.ID == 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	pending, err := h.readPendingReauthPasskey(c)
	if err != nil || pending == nil {
		log.Println("reauth passkey finish: no pending ceremony:", err)
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "ceremony_expired",
			"message": "That took too long. Try again.",
		})
	}

	// One-shot: cleared before verification, so a failed assertion
	// cannot be replayed against the same challenge.
	_ = h.Session.Delete(c, sessionKeyReauthPasskey)

	// The ceremony belongs to whoever began it. A session that changed
	// hands in between gets a fresh challenge, not this one.
	if pending.UserID != viewer.ID {
		log.Printf("reauth passkey finish: ceremony owner mismatch (pending=%d viewer=%d)",
			pending.UserID, viewer.ID)
		return c.NoContent(http.StatusUnauthorized)
	}

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || user == nil {
		log.Println("reauth passkey finish lookup:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	creds, err := h.Repos.WebAuthnCredentials.Browse(user.ID)
	if err != nil {
		log.Println("reauth passkey finish browse:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	w, err := webAuthnRP(h.Env)
	if err != nil {
		log.Println("reauth passkey finish config:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// FinishLogin checks the assertion against this user's credential
	// set, so a valid assertion from someone else's passkey fails here
	// rather than needing a separate ownership check.
	credential, err := w.FinishLogin(services.NewWebAuthnUser(user, creds), pending.Session, c.Request())
	if err != nil {
		// Deliberately generic. The library's error text names the
		// specific check that failed — useful in the log, a gift to
		// anyone probing the endpoint.
		log.Println("reauth passkey finish verify:", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error":   "assertion_failed",
			"message": "That passkey wasn't accepted. Try again, or use your password.",
		})
	}

	// Fold the authenticator's post-assertion state back onto the row.
	// Best-effort: the person has already proved themselves, and failing
	// the step-up over a bookkeeping write would be the wrong trade.
	for _, stored := range creds {
		if string(stored.CredentialID) != string(credential.ID) {
			continue
		}

		stored.ApplyAuthenticatorUpdate(credential)
		if err := h.Repos.WebAuthnCredentials.TouchAfterLogin(stored); err != nil {
			log.Println("reauth: failed to update credential after assertion:", err)
		}

		if stored.CloneWarning {
			// Recorded, not enforced — same reasoning as the login
			// path. Synced platform authenticators legitimately report
			// a flat counter, so blocking here would lock out ordinary
			// iCloud and Google passkey users.
			log.Printf("reauth: clone warning on credential (person=%d credential=%d)",
				user.ID, stored.ID)
		}

		break
	}

	if err := h.markStepUp(user.ID); err != nil {
		log.Println("reauth passkey mark:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Clears the password budget too. The person proved themselves,
	// just by another means — leaving earlier password failures on the
	// counter would let a passkey holder be locked out of the password
	// route by attempts they already superseded.
	h.rateLimitReset(rateLimitScopeReauth, fmt.Sprintf("%d", user.ID))

	log.Printf("reauth: passkey step-up granted (person=%d)", user.ID)

	return c.NoContent(http.StatusNoContent)
}

// writePendingReauthPasskey serializes ceremony state into the session.
func (h *reauthHandler) writePendingReauthPasskey(c echo.Context, pending pendingReauthPasskey) error {
	raw, err := json.Marshal(pending)
	if err != nil {
		return err
	}
	_, err = h.Session.Add(c, sessionKeyReauthPasskey, string(raw))
	return err
}

// readPendingReauthPasskey returns the pending ceremony, or nil if it
// is missing, malformed, or expired. Expired state is cleared as a side
// effect so it cannot linger across requests.
func (h *reauthHandler) readPendingReauthPasskey(c echo.Context) (*pendingReauthPasskey, error) {
	raw, err := h.Session.Read(c, sessionKeyReauthPasskey)
	if err != nil || raw == "" {
		return nil, err
	}

	var pending pendingReauthPasskey
	if err := json.Unmarshal([]byte(raw), &pending); err != nil {
		return nil, err
	}

	if time.Now().After(pending.ExpiresAt) {
		_ = h.Session.Delete(c, sessionKeyReauthPasskey)
		return nil, nil
	}

	return &pending, nil
}
