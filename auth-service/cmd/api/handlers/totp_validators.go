package handlers

import (
	"auth/data/entities"

	"anahita-libs/constants"

	"github.com/labstack/echo/v4"
	"github.com/pquerna/otp/totp"
)

type totpValidatorHandler struct {
	*Env
}

func newTotpValidatorHandler(env *Env) *totpValidatorHandler {
	return &totpValidatorHandler{
		env,
	}
}

func (h totpValidatorHandler) ValidateTotp(c echo.Context, passcode string, user *entities.User) (bool, error) {
	if len(passcode) == constants.TotpRecoveryCodeLength {
		ok, err := h.validateTotpRecoveryCode(passcode, user.Email)
		if ok {
			return true, err
		} else {
			return false, err
		}
	}

	totpSecret, err := user.GetTotpSecret(h.SecretCrypt)
	if err != nil {
		return false, err
	}

	if totp.Validate(passcode, totpSecret) {
		return true, nil
	}

	return false, nil
}

func (h totpValidatorHandler) validateTotpRecoveryCode(code string, email string) (bool, error) {
	user, err := h.Repos.Users.ReadByEmail(email)
	if err != nil {
		return false, err
	}

	if user.ValidateRecoveryCode(code) {
		_, err = h.Repos.Users.Edit(user)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}
