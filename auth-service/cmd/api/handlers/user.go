package handlers

import (
	"auth/requests"
	"auth/responses"
	"net/http"

	"github.com/labstack/echo/v4"
)

type userHandler struct {
	*Env
}

// NewUserHandler creates a new user handler
func NewUserHandler(env *Env) *userHandler {
	return &userHandler{
		env,
	}
}

// Browse returns a list of users
func (h *userHandler) Browse(c echo.Context) error {
	var (
		req    = requests.NewUserRequest().Browse
		viewer = getViewer(c)
	)

	if ok, err := h.Perms.Users.CanBrowse(viewer); !ok {
		return c.JSON(http.StatusForbidden, err)
	}

	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	users, err := h.Repos.Users.Browse(req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	userResp := &responses.UserResponse{}
	resp := userResp.MapNodeItems(users)

	return c.JSON(http.StatusAccepted, resp)
}

// Read returns a user
func (h *userHandler) Read(c echo.Context) error {
	var (
		req    = requests.NewUserRequest().Read
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	user, err := h.Repos.Users.ReadByUsername(req.Username)
	if err != nil {
		return c.JSON(http.StatusNotFound, err)
	}

	if ok, err := h.Perms.Users.CanRead(viewer, user); !ok {
		return c.JSON(http.StatusForbidden, err)
	}

	userResp := &responses.UserResponse{}
	resp := userResp.MapNode(user)

	return c.JSON(http.StatusAccepted, resp)
}

// Edit is gone. It set Username and Email together from a single
// unauthenticated-by-anything-but-a-session payload, which is the shape
// this service has moved away from: a handle is a sign-in identifier and
// a public address, an address is what password reset delivers to, and
// both now move only through their own step-up-gated flows —
// PATCH /username/change and PATCH /email/change.
//
// It could not have worked in any case. Users.Edit no longer writes
// either column, so the handler would have reported success and changed
// nothing. Nothing routed it.

// Delete deletes a user
func (h *userHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewUserRequest().Delete
		viewer = getViewer(c)
	)

	if err = c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	if err = c.Validate(req); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	user, err := h.Repos.Users.ReadByUsername(req.Username)
	if err != nil {
		return c.JSON(http.StatusNotFound, err)
	}

	if ok, err := h.Perms.Users.CanDelete(viewer, user); !ok {
		return c.JSON(http.StatusForbidden, err)
	}

	if err = h.Repos.Users.Delete(user.ID); err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	userResp := &responses.UserResponse{}
	resp := userResp.MapNode(user)

	return c.JSON(http.StatusAccepted, resp)
}
