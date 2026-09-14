package handlers

import (
	"log"
	"net/http"

	libs_requests "anahita-libs/requests"

	"auth/requests"

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

// Username returns whether a username is already taken.
//
// 200 taken, 404 free — the same shape as Email beside it.
//
// Safe to expose in a way the email check is not: a username IS public. It is
// the address of a profile, /people/<username> resolves for anybody, and the
// actor namespace is the thing that makes it so. Confirming one is taken
// discloses nothing that a single page load does not.
//
// Scoped to every actor type, because IsUsername is — a group holding the name
// makes it unavailable to a person, and saying otherwise here would offer a
// username the signup then refuses.
func (h *isHandler) Username(c echo.Context) error {
	req := &requests.UsernameAvailability{}

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		// A malformed username is not an available one. Answering 404 would
		// have the form say "available" about something that cannot be
		// registered.
		return c.NoContent(http.StatusBadRequest)
	}

	taken, err := h.Repos.Users.IsUsername(req.Username)
	if err != nil {
		log.Println("is username:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if taken {
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
