package handlers

import (
	"bytes"
	"encoding/json"
	"image/png"
	"log"
	"net/http"
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

	// Email
	// go func() {
	// 	if err := h.Notif.Totp.EmailTotpEnabled(user); err != nil {
	// 		log.Println(err)
	// 	}
	// }()

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

	return c.JSON(http.StatusOK, rCodes)
}

// Disables a person's TOTP 2FA
func (h *totpHandler) Delete(c echo.Context) error {
	var (
		viewer = getViewer(c)
		req    = requests.NewTotpRequest().Delete
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	user, err := h.Repos.Users.ReadByEmailOrUsername(req.EmailOrUsername)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if err := h.Perms.Totp.CanDelete(viewer, user); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotImplemented)
	}

	user.TotpSecret = ""
	user.TotpRecoveryCodes = ""
	user.TotpEnabled = false
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

	// Email
	// go func() {
	// 	if err := h.Notif.Totp.EmailTotpDisabled(user); err != nil {
	// 		log.Println(err)
	// 	}
	// }()

	return c.NoContent(http.StatusOK)
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
