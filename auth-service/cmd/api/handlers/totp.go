package handlers

import (
	"bytes"
	"encoding/json"
	"image/png"
	"log"
	"net/http"
	"strconv"
	"time"

	"auth/requests"

	"anahita-libs/constants"
	"auth/data/entities"

	"github.com/labstack/echo/v4"
	"github.com/pquerna/otp/totp"
)

type totpHandler struct {
	*Env
}

func newTotpHandler(env *Env) *totpHandler {
	return &totpHandler{
		env,
	}
}

// Verifies a TOTP passcode
func (h *totpHandler) Verify(c echo.Context) error {
	var (
		viewer        = getViewer(c)
		req           = &requests.Totp{}
		totpValidator = newTotpValidatorHandler(h.Env)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if !user.TotpEnabled {
		log.Println("TOTP is not enabled")
		return c.NoContent(http.StatusForbidden)
	}

	if ok, err := totpValidator.ValidateTotp(c, req.TotpPasscode, user); !ok {
		log.Println(err, "TOTP validation failed")
		return c.NoContent(http.StatusForbidden)
	} else {
		user.TotpLastVerifiedAt = time.Now()
		user, err = h.Repos.Users.Edit(user)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	return c.NoContent(http.StatusOK)
}

func (h *totpHandler) Edit(c echo.Context) error {
	var (
		err    error
		viewer = getViewer(c)
	)

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if err := h.Perms.Totp.CanEdit(viewer, user); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	user.TotpEnabled = true
	user.TotpLastVerifiedAt = time.Now()
	user, err = h.Repos.Users.Edit(user)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Update session
	if err = h.updateSession(c, user); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Neutral in tone, unlike the disable notice: enabling 2FA strengthens
	// the account, and alarming somebody about an improvement is how they
	// learn to ignore the message that matters.
	notifyUser := *user
	go func() {
		if err := h.Notif.Totp.EmailTotpEnabled(&notifyUser); err != nil {
			log.Println(err)
		}
	}()

	return c.NoContent(http.StatusOK)
}

// Given the user password, it creates a TOTP Secret, a QR code, and a list of Recovery Codes.
// The QR code is returned in the response while the secret and recovery codes are stored in the database.
// In this state, TOTP is still NOT enabled.
func (h *totpHandler) Add(c echo.Context) error {
	var (
		viewer = getViewer(c)
		req    = &requests.TotpPassword{}
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// validate password
	if !user.CheckPassword(req.Password) {
		log.Println("Invalid password")
		return c.NoContent(http.StatusNotAcceptable)
	}

	// AccountName comes from the person record, NOT from the viewer.
	//
	// The viewer is built from the access token, and the token
	// deliberately carries no profile data — see BuildViewer in
	// anahita-libs/middleware/oauth.go, which keeps it thin rather than
	// trusting claims that go stale after a profile change. So
	// viewer.Email is always empty here, totp.Generate rejected the
	// options with "AccountName must be set", and enrolment answered
	// 500 for everyone. `user` was already read from the database above
	// and has the address.
	//
	// AccountName stays per-person and cannot be configured away: it is
	// the "(you@example.com)" an authenticator shows, and the only thing
	// distinguishing two entries for the same site. The deployment-wide
	// half of that label is the issuer, which is what GetTotpIssuer
	// supplies.
	if user.Email == "" {
		log.Printf("totp add: person has no email address (person=%d)", user.ID)
		return c.NoContent(http.StatusInternalServerError)
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      h.Config.GetTotpIssuer(),
		AccountName: user.Email,
	})

	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err = h.Perms.Totp.CanAdd(viewer, user); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	user.TotpEnabled = false
	if err := user.SetTotpSecret(h.SecretCrypt, key.Secret()); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	user, err = h.Repos.Users.Edit(user)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Convert TOTP key into a QR code encoded as a PNG image.
	var buf bytes.Buffer
	img, err := key.Image(200, 200)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	png.Encode(&buf, img)

	return c.Blob(http.StatusCreated, "image/x-png", buf.Bytes())
}

// Creates recovery codes for a person
func (h *totpHandler) AddRecoveryCodes(c echo.Context) error {
	var (
		viewer = getViewer(c)
		err    error
		rc     = totpRecoveryCodes{}
	)

	type Request struct {
		Passcode string `form:"passcode" validate:"min=6,max=8,alphanum,required"`
	}

	req := &Request{}
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if err := h.Perms.Totp.CanAddRecoveryCodes(viewer, user); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	totpSecret, err := user.GetTotpSecret(h.SecretCrypt)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// first validating with totp before showing the recovering codes
	if !totp.Validate(req.Passcode, totpSecret) {
		return c.NoContent(http.StatusUnauthorized)
	}

	rCodes := rc.generateCodes(constants.TotpNumRecoveryCodes, constants.TotpRecoveryCodeLength)

	// The error was dropped here. SetRecoveryCodes bcrypts each code
	// before storing it, and on failure it leaves the field untouched —
	// so the response still handed the person sixteen codes that would
	// match nothing, which they would only discover when locked out and
	// reaching for one.
	if err := user.SetRecoveryCodes(rCodes); err != nil {
		log.Println("totp recovery codes:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	user, err = h.Repos.Users.Edit(user)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Update session
	if err = h.updateSession(c, user); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Generating a set silently invalidates the previous one and mints a
	// fresh way past 2FA, so it is worth telling the owner about. Somebody
	// who did not do this is looking at an attacker who holds their password
	// AND a live passcode, and who has just given themselves a durable
	// bypass — this is the only place that surfaces.
	notifyUser := *user
	issued := len(rCodes)
	go func() {
		if err := h.Notif.Totp.EmailRecoveryCodesIssued(&notifyUser, issued); err != nil {
			log.Println("totp: recovery codes notification failed:", err)
		}
	}()

	return c.JSON(http.StatusOK, rCodes)
}

// Delete disables 2FA on an account.
//
//	DELETE /totp/:person_id
//
// Two callers: a person turning off their own second factor, and an
// administrator recovering somebody who has lost their authenticator and their
// recovery codes with it. Permissions.Totp.CanDelete allows the first
// unconditionally and the second with the actors:write scope.
//
// Step-up gated, because this WEAKENS an account: it is the one change here
// that leaves the door easier to open than it found it, so it wants the same
// proof as changing a password.
//
// Status codes:
//
//	204 — disabled, or already off
//	400 — bad person id
//	403 — with a JSON body, the caller may not act on this account;
//	      with an EMPTY body, no recent step-up. The client tells them apart
//	      by the body: a bare 403 means re-authenticate and retry, and a
//	      403 carrying `error` means stop. They share a status because the
//	      step-up contract across /password, /email and /username is 403, and
//	      the permission failure genuinely is a forbidden.
//	404 — no such person
//	500 — database failure
func (h *totpHandler) Delete(c echo.Context) error {
	var (
		viewer = getViewer(c)
		req    = requests.NewTotpRequest().Delete
	)

	if viewer == nil || viewer.ID == 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	if err := c.Bind(req); err != nil {
		log.Println("totp delete bind:", err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println("totp delete validate:", err)
		return c.NoContent(http.StatusBadRequest)
	}

	user, err := h.Repos.Users.ReadByID(req.PersonID)
	if err != nil || user == nil {
		log.Println("totp delete lookup:", err)
		return c.NoContent(http.StatusNotFound)
	}

	// Before the step-up, so a self-service 403 can only ever mean
	// "re-authenticate" — which is what lets the client act on it without
	// inspecting the body first.
	if err := h.Perms.Totp.CanDelete(viewer, user); err != nil {
		log.Println("totp delete permission:", err)
		return c.JSON(http.StatusForbidden, map[string]string{
			"error":       "forbidden",
			"description": err.Error(),
		})
	}

	// Already off. Answered before the step-up so nobody is sent through a
	// re-authentication to change nothing; the caller has already proved it
	// may act on this account, so this reveals nothing it could not read.
	if !user.TotpEnabled && user.TotpSecret == "" {
		return c.NoContent(http.StatusNoContent)
	}

	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("totp delete:", err)
		return c.NoContent(reauthStatus(err))
	}

	// The secret goes with the flag. Leaving it behind would let a later
	// enrolment silently inherit a secret somebody else may hold, and
	// Users.Edit now writes the column, so clearing it here finally takes.
	user.TotpSecret = ""
	user.TotpRecoveryCodes = ""
	user.TotpEnabled = false

	user, err = h.Repos.Users.Edit(user)
	if err != nil {
		log.Println("totp delete:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// One step-up authorises one credential change.
	h.clearStepUp(viewer.ID)

	// SELF ONLY. updateSession writes the record it is handed into the
	// CURRENT session, so calling it on an administrator disabling somebody
	// else's 2FA would replace the administrator's own session with their
	// target's. Best-effort besides: the change is already committed, and the
	// session is refreshed on the next request anyway.
	if viewer.ID == user.ID {
		if err := h.updateSession(c, user); err != nil {
			log.Println("totp delete: session refresh failed:", err)
		}
	}

	log.Printf("totp: disabled (person=%d by=%d)", user.ID, viewer.ID)

	// Fire-and-forget. This is the tripwire: an unexpected "your 2FA was
	// turned off" is how somebody finds out an administrator, or an attacker
	// holding a step-up, has weakened their account.
	//
	// Two messages, chosen by who acted. Asking "was this you?" of somebody
	// who did not touch their account is worse than useless — they answer no
	// and reset a password that was never at risk, while the actual question
	// (why did an administrator do this?) goes unasked.
	notifyUser := *user
	byAdmin := viewer.ID != user.ID
	go func() {
		var err error
		if byAdmin {
			err = h.Notif.Totp.EmailTotpDisabledByAdmin(&notifyUser)
		} else {
			err = h.Notif.Totp.EmailTotpDisabled(&notifyUser)
		}
		if err != nil {
			log.Println(err)
		}
	}()

	return c.NoContent(http.StatusNoContent)
}

// Read handles GET /totp
//
// Reports whether the viewer has TOTP enrolled, and whether that
// enrolment's last verification has aged out.
//
// The viewer's own account only — no :person_id parameter, deliberately.
// A status endpoint that took an identifier would answer "does this
// person have 2FA?" for anyone who asked, which is a map of which
// accounts are worth phishing. /auth/is/totp accepts an email because
// the login page needs it before there is a session; there is no such
// excuse behind authorize.
//
// This is the read the settings card and the step-up dialog both need.
// Without it neither can tell an enrolled account from a fresh one:
// the enrolment wizard opened on step one regardless of state, and the
// step-up dialog rendered no passcode field, so a TOTP-enrolled person
// submitted a password alone and the server rejected it forever.
//
//	200 — {"data": {"enabled": bool, "expired": bool}}
//	500 — database failure
//
// expired is false whenever enabled is false. An account with no TOTP
// has nothing to have expired, and reporting the raw IsMfaExpired for
// one would be true — a value the client would reasonably read as
// "re-verify your second factor" for a factor that does not exist.
// ReadForPerson reports whether one person has TOTP enabled.
//
//	GET /totp/:person_id
//
// The admin counterpart to Read above, which answers for the viewer and
// takes no id. An administration screen offering to remove somebody's
// second factor needs to know whether they have one — without this it
// can only show a button and hope.
//
// Same body as Read: enabled and expired, and nothing else. The secret
// and the recovery codes are not somebody else's to see, and are not
// this endpoint's to send even to their owner.
func (h *totpHandler) ReadForPerson(c echo.Context) error {
	viewer := getViewer(c)

	if viewer == nil || viewer.ID == 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	personID, err := strconv.ParseInt(c.Param("person_id"), 10, 64)
	if err != nil || personID <= 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	user, err := h.Repos.Users.ReadByID(personID)
	if err != nil || user == nil {
		log.Println("totp read for person:", err)
		return c.NoContent(http.StatusNotFound)
	}

	if err := h.Perms.Totp.CanRead(viewer, user); err != nil {
		log.Println("totp read for person:", err)
		return c.NoContent(http.StatusForbidden)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data": map[string]any{
			"enabled": user.TotpEnabled,
			"expired": user.TotpEnabled && user.IsMfaExpired(),
		},
	})
}

func (h *totpHandler) Read(c echo.Context) error {
	var (
		viewer = getViewer(c)
	)

	if viewer == nil || viewer.ID == 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || user == nil {
		log.Println("totp read:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data": map[string]any{
			"enabled": user.TotpEnabled,
			"expired": user.TotpEnabled && user.IsMfaExpired(),
		},
	})
}

// IsExpired
func (h *totpHandler) IsExpired(c echo.Context) error {
	var (
		viewer = getViewer(c)
	)

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if user.IsMfaExpired() {
		return c.NoContent(http.StatusOK)
	}

	return c.NoContent(http.StatusNoContent)
}

// updateSession updates the viewer object in session with the new values of the person
func (h *totpHandler) updateSession(c echo.Context, user *entities.User) error {
	jsonUser, err := json.Marshal(user)
	if err != nil {
		return err
	}

	if err := h.Session.Edit(c, constants.UserSessionKey, string(jsonUser)); err != nil {
		log.Println(err)
		return err
	}

	return nil
}
