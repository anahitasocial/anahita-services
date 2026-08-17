package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type storyHandler struct {
	*Env
}

// NewStoryHandler creates a new story handler
func NewStoryHandler(env *Env) *storyHandler {
	return &storyHandler{
		env,
	}
}

// Browse returns a list of story
func (h *storyHandler) Browse(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}
