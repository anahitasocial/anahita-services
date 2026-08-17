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

	issuer := constants.TotpIssuer
	if h.Config.IsStaging() {
		issuer = constants.TotpIssuerStaging
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: viewer.Email,
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

	user.SetRecoveryCodes(rCodes)

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
