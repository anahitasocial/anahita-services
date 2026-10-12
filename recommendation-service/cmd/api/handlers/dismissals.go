package handlers

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// "Not interested": a profile somebody turned down is not suggested to them
// again. Theirs alone: whoever was turned down is not told, and nobody else
// can read the list.

type dismissalsHandler struct {
	*Env
}

type dismissalAddRequest struct {
	ActorID int64 `json:"actor_id" validate:"required,numeric,min=1"`
}

type dismissalRemoveRequest struct {
	ActorID int64 `param:"actor_id" validate:"required,numeric,min=1"`
}

// Browse lists the profiles the viewer turned down, most lately first, for
// taking one back.
func (h *dismissalsHandler) Browse(c echo.Context) error {
	viewer := getViewer(c)
	if !signedIn(viewer) {
		return c.NoContent(http.StatusForbidden)
	}

	ids, err := h.Repos.Dismissals.List(viewer.ID)
	if err != nil {
		log.Println("recommendations: dismissals:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{"data": h.dismissedActors(ids, viewer)})
}

func (h *dismissalsHandler) Add(c echo.Context) error {
	viewer := getViewer(c)
	if !signedIn(viewer) {
		return c.NoContent(http.StatusForbidden)
	}

	req := &dismissalAddRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil || req.ActorID == viewer.ID {
		return c.NoContent(http.StatusBadRequest)
	}

	if err := h.Repos.Dismissals.Add(viewer.ID, req.ActorID); err != nil {
		log.Println("recommendations: dismissing:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *dismissalsHandler) Delete(c echo.Context) error {
	viewer := getViewer(c)
	if !signedIn(viewer) {
		return c.NoContent(http.StatusForbidden)
	}

	req := &dismissalRemoveRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	if err := h.Repos.Dismissals.Remove(viewer.ID, req.ActorID); err != nil {
		log.Println("recommendations: taking a dismissal back:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusNoContent)
}
