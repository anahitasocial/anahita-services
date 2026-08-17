package handlers

import (
	"log"
	"net/http"

	libs_requests "anahita-libs/requests"

	"github.com/labstack/echo/v4"
)

// isHandler is the handler for "is" checks
type isHandler struct {
	*Env
}

// NewAuthHandlers returns a new auth handler
func newIsHandlers(env *Env) *isHandler {
	return &isHandler{
		env,
	}
}

// Totp returns whether or not the user has totp enabled
func (h *isHandler) Totp(c echo.Context) error {
	var (
		req = &libs_requests.EmailRequest{}
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	user, err := h.Repos.Users.ReadByEmail(req.Email)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if user.TotpEnabled {
		return c.NoContent(http.StatusOK)
	}

	return c.NoContent(http.StatusNotFound)
}

// Email returns whether or not the email already exists
func (h *isHandler) Email(c echo.Context) error {
	var (
		err error
		req = &libs_requests.EmailRequest{}
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	exists, err := h.Repos.Users.ReadByEmail(req.Email)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if exists == nil {
		return c.NoContent(http.StatusNotFound)
	}

	return c.NoContent(http.StatusOK)
}
