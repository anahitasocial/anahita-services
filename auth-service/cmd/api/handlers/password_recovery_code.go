package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type passwordRecoveryCodeHandler struct {
	*Env
}

func NewPasswordRecoveryCodeHandler(env *Env) *passwordRecoveryCodeHandler {
	return &passwordRecoveryCodeHandler{env}
}

func (h *passwordRecoveryCodeHandler) Read(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}
