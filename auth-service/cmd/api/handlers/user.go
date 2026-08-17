package handlers

import (
	"auth/requests"
	"auth/responses"
	"net/http"
	"time"

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

// Edit updates a user
func (h *userHandler) Edit(c echo.Context) error {
	var (
		req    = requests.NewUserRequest().Edit
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	user, err := h.Repos.Users.ReadByID(req.ID)
	if err != nil {
		return c.JSON(http.StatusNotFound, err)
	}

	if ok, err := h.Perms.Users.CanEdit(viewer, user); !ok {
		return c.JSON(http.StatusForbidden, err)
	}

	user.Username = req.Username
	user.Email = req.Email
	user.UpdateTime = time.Now()

	if err = user.Validate(); err != nil {
		return c.JSON(http.StatusForbidden, err)
	}

	user, err = h.Repos.Users.Edit(user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	userResp := &responses.UserResponse{}
	resp := userResp.MapNode(user)

	return c.JSON(http.StatusAccepted, resp)
}

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
