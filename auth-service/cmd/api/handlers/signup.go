package handlers

import (
	"anahita-libs/constants"
	"auth/data/entities"
	"auth/requests"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

type signupHandler struct {
	*Env
}

// newSignupHandler creates a new Signup handler
func newSignupHandler(env *Env) *signupHandler {
	return &signupHandler{
		env,
	}
}

// Signup creates a new user
func (h *signupHandler) Add(c echo.Context) error {
	var (
		req    = requests.NewSignupRequest().Add
		viewer = getViewer(c)
	)

	ok, err := h.Perms.Signup.CanAdd(viewer, nil)
	if !ok || err != nil {
		return c.JSON(http.StatusForbidden, err)
	}

	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	// making sure that the user doesn't already exist

	// Chcking if the user already exists by email
	emailUser, _ := h.Repos.Users.ReadByEmail(req.Email)
	if emailUser != nil {
		return c.JSON(http.StatusConflict, "A user with this email already exists")
	}

	// Checking if the user already exists by username
	usernameUser, _ := h.Repos.Users.ReadByUsername(req.Username)
	if usernameUser != nil {
		return c.JSON(http.StatusConflict, "A user with this username already exists")
	}

	// Creating the user
	user := entities.NewUser()
	user.UserType = constants.UserTypeRegistered
	user.Email = req.Email
	user.Username = req.Username

	// Enabled explicitly, the way invite acceptance already does it.
	//
	// NewUser leaves it false, and a false here is indistinguishable from a
	// deliberate one — so the repository cannot fill it in the way it fills
	// the timestamps. Left unset, signup created an account that existed and
	// could not sign in.
	user.Enabled = true

	if err := user.SetPassword(req.Password); err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	if err := user.Validate(); err != nil {
		return c.JSON(http.StatusForbidden, err)
	}

	user, err = h.Repos.Users.Add(user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	// Welcome email. The notifier and its copy have existed since the service
	// was written and no handler ever called them, so signing up has always
	// been silent — the first mail anybody received was whatever they
	// triggered next.
	//
	// Fire-and-forget after the row is committed, so it describes an account
	// that exists. A copy is captured because the response serialises the same
	// pointer.
	notifyUser := *user
	go func() {
		if err := h.Notif.Signup.EmailSignup(&notifyUser); err != nil {
			log.Println("signup: welcome email failed:", err)
		}
	}()

	return c.JSON(http.StatusCreated, user)
}

func (h *signupHandler) Confirm(c echo.Context) error {
	return c.NoContent(http.StatusAccepted)
}
