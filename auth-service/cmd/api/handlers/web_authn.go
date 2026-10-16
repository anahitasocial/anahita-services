package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"auth/constants"
	"auth/data/entities"
	"auth/data/repositories"
	"auth/data/services"
	"auth/requests"
	"auth/responses"

	libs_responses "anahita-libs/responses"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/labstack/echo/v4"
)

// Session key + state for the registration ceremony.
//
// The challenge lives in the session rather than a bare Redis key for
// the same reason pendingTOTPState does: it is per-browser state that
// must not be usable from any other browser, and the session cookie
// already gives us that binding for free. A raw Redis key would need
// us to invent an equivalent binding by hand.
const sessionKeyWebAuthnRegister = "webauthn_register"

type pendingWebAuthnRegistration struct {
	Session   webauthn.SessionData `json:"session"`
	ExpiresAt time.Time            `json:"expires_at"`
}

type webAuthnHandler struct {
	*Env
}

func newWebAuthnHandler(env *Env) *webAuthnHandler {
	return &webAuthnHandler{
		env,
	}
}

// Browse handles GET /webauthn/credentials
//
// Lists the viewer's own enrolled passkeys.
func (h *webAuthnHandler) Browse(c echo.Context) error {
	var (
		viewer = getViewer(c)
	)

	if ok, permErr := h.Perms.WebAuthn.CanBrowse(viewer); !ok {
		return permForbidden(c, permErr)
	}

	creds, err := h.Repos.WebAuthnCredentials.Browse(viewer.ID)
	if err != nil {
		log.Println("webauthn browse:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	webauthnResp := &responses.WebAuthnCredentialResponse{}
	return c.JSON(http.StatusOK, libs_responses.Unpaged[*responses.WebAuthnCredentialItem]{
		Data: webauthnResp.MapCredentials(creds),
	})
}

// AddRegisterBegin handles POST /webauthn/credentials/register/begin
//
// Returns the creation options the browser hands to the authenticator,
// and stashes the matching challenge in session. Takes no request
// body — there is nothing the client can usefully tell us here.
//
// The response body is the library's protocol.CredentialCreation,
// which marshals binary fields as base64url — the client JS has to
// decode them back to ArrayBuffers before calling
// navigator.credentials.create. See web/static/js/webauthn.js.
func (h *webAuthnHandler) AddRegisterBegin(c echo.Context) error {
	var (
		viewer = getViewer(c)
	)

	if ok, permErr := h.Perms.WebAuthn.CanAdd(viewer); !ok {
		return permForbidden(c, permErr)
	}

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		log.Println("webauthn register begin person lookup:", err)
		return c.NoContent(http.StatusNotFound)
	}

	count, err := h.Repos.WebAuthnCredentials.Count(viewer.ID)
	if err != nil {
		log.Println("webauthn register begin count:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if count >= constants.WebAuthnMaxCredentialsPerPerson {
		return refuse(c, http.StatusConflict, "credential_limit_reached",
			"Remove a passkey before adding another.")
	}

	existing, err := h.Repos.WebAuthnCredentials.Browse(viewer.ID)
	if err != nil {
		log.Println("webauthn register begin browse:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Exclusions stop the same authenticator being enrolled twice.
	// Without them the browser happily creates a second credential on
	// a device that already has one, and the person ends up with two
	// identical-looking entries and no way to tell which is which.
	exclusions := make([]protocol.CredentialDescriptor, 0, len(existing))
	for _, cred := range existing {
		exclusions = append(exclusions, cred.Descriptor())
	}

	w, err := webAuthnRP(h.Env)
	if err != nil {
		log.Println("webauthn register begin config:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	webAuthnUser := services.NewWebAuthnUser(user, existing)

	options, session, err := w.BeginRegistration(
		webAuthnUser,
		webauthn.WithExclusions(exclusions),
	)
	if err != nil {
		log.Println("webauthn register begin ceremony:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	pending := pendingWebAuthnRegistration{
		Session:   *session,
		ExpiresAt: time.Now().Add(constants.WebAuthnCeremonyTTL),
	}
	if err := h.writePendingRegistration(c, pending); err != nil {
		log.Println("webauthn register begin state write:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, options)
}

// AddRegisterFinish handles POST /webauthn/credentials/register/finish
//
// Verifies the attestation against the stashed challenge and persists
// the credential.
//
// The request body is the raw attestation response produced by
// navigator.credentials.create — the library parses it directly off
// the request, so nothing else can travel in this body.
func (h *webAuthnHandler) AddRegisterFinish(c echo.Context) error {
	var (
		viewer = getViewer(c)
	)

	if ok, permErr := h.Perms.WebAuthn.CanAdd(viewer); !ok {
		return permForbidden(c, permErr)
	}

	pending, err := h.readPendingRegistration(c)
	if err != nil || pending == nil {
		log.Println("webauthn register finish: no pending ceremony:", err)
		return refuse(c, http.StatusBadRequest, "ceremony_expired",
			"That took too long. Start again.")
	}

	// One-shot. Cleared before verification so a failed attempt can't
	// be replayed against the same challenge.
	_ = h.Session.Delete(c, sessionKeyWebAuthnRegister)

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		log.Println("webauthn register finish person lookup:", err)
		return c.NoContent(http.StatusNotFound)
	}

	existing, err := h.Repos.WebAuthnCredentials.Browse(viewer.ID)
	if err != nil {
		log.Println("webauthn register finish browse:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	w, err := webAuthnRP(h.Env)
	if err != nil {
		log.Println("webauthn register finish config:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	webAuthnUser := services.NewWebAuthnUser(user, existing)

	credential, err := w.FinishRegistration(webAuthnUser, pending.Session, c.Request())
	if err != nil {
		// Verification failures are the person's browser or
		// authenticator disagreeing with us, not a server fault.
		// 400, with no detail — the error text names internal
		// checks and tells an attacker which gate they tripped.
		log.Println("webauthn register finish verify:", err)
		return refuse(c, http.StatusBadRequest, "registration_failed",
			"That passkey couldn't be registered. Try again.")
	}

	// The nickname is derived from the AAGUID and transports the
	// authenticator just reported — see entities.DeriveNickname. The
	// person is never asked for one up front, because until this
	// moment neither they nor we knew what was being enrolled.
	cred := entities.NewWebAuthnCredentialFromLibrary(
		credential,
		user.ID,
	)

	persisted, err := h.Repos.WebAuthnCredentials.Add(cred)
	if err != nil {
		log.Println("webauthn register finish add:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Security notification. Async for the same reason the login
	// notification is: a slow mail server must not hold up the
	// response. A passkey appearing on an account the person didn't
	// touch is exactly the signal worth sending mail about.
	notifyUser := *user
	notifyNickname := persisted.Nickname
	go func() {
		if err := h.Notif.WebAuthn.EmailPasskeyAdded(&notifyUser, notifyNickname); err != nil {
			log.Println("webauthn: passkey-added notification failed:", err)
		}
	}()

	log.Printf("webauthn: passkey registered (person=%d credential=%d)", user.ID, persisted.ID)

	webAuthnResp := &responses.WebAuthnCredentialResponse{}
	return c.JSON(http.StatusCreated, libs_responses.Item[*responses.WebAuthnCredentialItem]{
		Data: webAuthnResp.MapCredential(persisted),
	})
}

// Edit handles PATCH /webauthn/credentials/:id
//
// Renames a passkey. The only mutable field.
func (h *webAuthnHandler) Edit(c echo.Context) error {
	var (
		viewer = getViewer(c)
		req    = requests.NewWebAuthnCredentialRequest().Edit
	)

	if err := c.Bind(req); err != nil {
		log.Println("webauthn edit bind:", err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println("webauthn edit validate:", err)
		return c.NoContent(http.StatusBadRequest)
	}

	cred, err := h.Repos.WebAuthnCredentials.ReadByID(req.ID)
	if err != nil {
		if errors.Is(err, repositories.ErrWebAuthnCredentialNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		log.Println("webauthn edit read:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if ok, permErr := h.Perms.WebAuthn.CanEdit(viewer, cred); !ok {
		return permForbidden(c, permErr)
	}

	cred.Nickname = req.Nickname

	persisted, err := h.Repos.WebAuthnCredentials.Edit(cred)
	if err != nil {
		log.Println("webauthn edit:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	webauthnResp := &responses.WebAuthnCredentialResponse{}
	return c.JSON(http.StatusOK, libs_responses.Item[*responses.WebAuthnCredentialItem]{
		Data: webauthnResp.MapCredential(persisted),
	})
}

// Delete handles DELETE /webauthn/credentials/:id
//
// Revokes a passkey. Removing the last one is permitted — password +
// TOTP is the permanent baseline, so there is no lockout to guard
// against.
func (h *webAuthnHandler) Delete(c echo.Context) error {
	var (
		viewer = getViewer(c)
		req    = requests.NewWebAuthnCredentialRequest().Delete
	)

	if err := c.Bind(req); err != nil {
		log.Println("webauthn delete bind:", err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println("webauthn delete validate:", err)
		return c.NoContent(http.StatusBadRequest)
	}

	cred, err := h.Repos.WebAuthnCredentials.ReadByID(req.ID)
	if err != nil {
		if errors.Is(err, repositories.ErrWebAuthnCredentialNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		log.Println("webauthn delete read:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if ok, permErr := h.Perms.WebAuthn.CanDelete(viewer, cred); !ok {
		return permForbidden(c, permErr)
	}

	if err := h.Repos.WebAuthnCredentials.Delete(cred.ID); err != nil {
		log.Println("webauthn delete:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		// The revocation succeeded; only the notification is at risk.
		// Log and return success rather than implying the passkey is
		// still active.
		log.Println("webauthn delete person lookup for notification:", err)
	} else {
		notifyPerson := *user
		notifyNickname := cred.Nickname
		go func() {
			if err := h.Notif.WebAuthn.EmailPasskeyRemoved(&notifyPerson, notifyNickname); err != nil {
				log.Println("webauthn: passkey-removed notification failed:", err)
			}
		}()
	}

	log.Printf("webauthn: passkey revoked (person=%d credential=%d)", viewer.ID, cred.ID)

	return c.NoContent(http.StatusOK)
}

// ReadCountForPerson handles GET /webauthn/credentials/person/:person_id
//
// Returns how many passkeys a person holds. Nothing else — no device
// models, no transports, no last-used times.
//
// That restraint is the point. A list would profile which devices a
// person carries; a count only answers whether there is anything
// to revoke, which is the minimum the admin UI needs to avoid
// offering an action that might silently do nothing.
//
// Global admin, or your own account. No TOTP step-up: this is a read
// that populates a card, and requiring fresh TOTP to render a person
// page would be a lot of friction for one integer.
func (h *webAuthnHandler) ReadCountForPerson(c echo.Context) error {
	var (
		viewer = getViewer(c)
		req    = requests.NewWebAuthnCredentialRequest().User
	)

	if err := c.Bind(req); err != nil {
		log.Println("webauthn count bind:", err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println("webauthn count validate:", err)
		return c.NoContent(http.StatusBadRequest)
	}

	actor, err := hydrateViewer(h.Env, viewer)
	if err != nil {
		log.Println("webauthn count actor lookup:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	target, err := h.Repos.Users.ReadByID(req.UserID)
	if err != nil || target == nil {
		log.Println("webauthn count person lookup:", err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, permErr := h.Perms.WebAuthn.CanReadCount(viewer, actor, target); !ok {
		return permForbidden(c, permErr)
	}

	count, err := h.Repos.WebAuthnCredentials.Count(target.ID)
	if err != nil {
		log.Println("webauthn count:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, libs_responses.Item[PasskeyCount]{Data: PasskeyCount{Count: count}})
}

// DeleteAll handles DELETE /webauthn/credentials/person/:person_id
//
// Revokes every passkey belonging to one person. Global admins only,
// except when acting on your own account.
//
// Blunt by design: there is no listing counterpart, so an admin can
// remove someone's passkeys without ever seeing which devices they
// held. See permissions.CanDeleteAll for why the asymmetry is
// deliberate.
//
// Not a lockout. Email + password + TOTP remains the permanent
// baseline, so this removes a shortcut rather than access.
func (h *webAuthnHandler) DeleteAll(c echo.Context) error {
	var (
		viewer = getViewer(c)
		req    = requests.NewWebAuthnCredentialRequest().User
	)

	if err := c.Bind(req); err != nil {
		log.Println("webauthn delete all bind:", err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println("webauthn delete all validate:", err)
		return c.NoContent(http.StatusBadRequest)
	}

	// Hydrate before the permission check. CanDeleteAll reads person
	// state — TotpEnabled, person type — none of which the JWT
	// middleware puts on the viewer. Skipping this denies every
	// caller with a message that contradicts their account. See
	// hydrateViewer.
	actor, err := hydrateViewer(h.Env, viewer)
	if err != nil {
		log.Println("webauthn delete all actor lookup:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	target, err := h.Repos.Users.ReadByID(req.UserID)
	if err != nil || target == nil {
		log.Println("webauthn delete all person lookup:", err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, permErr := h.Perms.WebAuthn.CanDeleteAll(viewer, actor, target); !ok {
		return permForbidden(c, permErr)
	}

	// Presence, after authority. Revoking somebody else's passkeys is
	// destructive and irreversible for them, so an administrator session
	// left open is not on its own enough — the same bar the OAuth writes
	// hold, and by the same mechanism, which accepts a passkey as
	// readily as a passcode.
	//
	// Self-service is exempt above and never reaches this: CanDeleteAll
	// returns early when the target is the actor, so clearing your own
	// passkeys stays the ordinary path it already was.
	if target.ID != actor.ID {
		if err := h.requireStepUp(actor.ID); err != nil {
			log.Println("webauthn delete all:", err)
			return stepUpRequired(c, err)
		}
	}

	revoked, err := h.Repos.WebAuthnCredentials.DeleteByPersonID(target.ID)
	if err != nil {
		log.Println("webauthn delete all:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// One proof, one revocation. Stripping a second person's passkeys
	// means proving yourself again, which is the point — each one is a
	// separate account somebody is being locked out of a shortcut to.
	if target.ID != actor.ID {
		h.clearStepUp(actor.ID)
	}

	// Audit line. Logged unconditionally, including the zero case,
	// because "an admin ran this and it found nothing" is itself
	// worth being able to reconstruct later.
	//
	// TODO: this belongs in auth_logs, not just stdout. The existing
	// authLogHandler.AddSystem requires an OAuthRefreshTokenID, which
	// an admin action against someone else's account does not have —
	// recording it properly needs an auth_logs variant that takes an
	// actor and a subject instead.
	log.Printf("webauthn: admin revoked all passkeys (admin=%d target=%d revoked=%d)",
		viewer.ID, target.ID, revoked)

	// Notify only when something was actually removed, and only when
	// the admin is someone else — telling a person "an administrator
	// removed your passkeys" about an action they just took
	// themselves is confusing, and telling them about a revocation
	// that removed nothing is noise.
	if revoked > 0 && target.ID != viewer.ID {
		notifyPerson := *target
		notifyCount := revoked
		go func() {
			if err := h.Notif.WebAuthn.EmailPasskeysRevoked(&notifyPerson, notifyCount); err != nil {
				log.Println("webauthn: passkeys-revoked notification failed:", err)
			}
		}()
	}

	return c.JSON(http.StatusOK, libs_responses.Item[PasskeysRevoked]{Data: PasskeysRevoked{Revoked: revoked}})
}

// writePendingRegistration serializes ceremony state into the session.
func (h *webAuthnHandler) writePendingRegistration(c echo.Context, pending pendingWebAuthnRegistration) error {
	raw, err := json.Marshal(pending)
	if err != nil {
		return err
	}
	_, err = h.Session.Add(c, sessionKeyWebAuthnRegister, string(raw))
	return err
}

// readPendingRegistration returns the pending ceremony, or nil if it
// is missing, malformed, or expired. Expired state is cleared as a
// side effect so it can't linger across requests.
func (h *webAuthnHandler) readPendingRegistration(c echo.Context) (*pendingWebAuthnRegistration, error) {
	raw, err := h.Session.Read(c, sessionKeyWebAuthnRegister)
	if err != nil || raw == "" {
		return nil, err
	}

	var pending pendingWebAuthnRegistration
	if err := json.Unmarshal([]byte(raw), &pending); err != nil {
		return nil, err
	}

	if time.Now().After(pending.ExpiresAt) {
		_ = h.Session.Delete(c, sessionKeyWebAuthnRegister)
		return nil, nil
	}

	return &pending, nil
}

// PasskeyCount is how many passkeys somebody holds, and nothing else
// about them.
type PasskeyCount struct {
	// How many.
	Count int `json:"count"`
}

// PasskeysRevoked is how many passkeys were removed.
type PasskeysRevoked struct {
	// How many.
	Revoked int64 `json:"revoked"`
}
