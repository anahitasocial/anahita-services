package handlers

import (
	"log"
	"net/http"

	"recommendation/data/repositories"

	"github.com/labstack/echo/v4"
)

// What somebody has chosen about suggestions: whether to be suggested
// people and groups, whether what they do may be used to choose, and
// whether they may be suggested to others. Their own to read and change,
// and nobody else's.

type preferencesHandler struct {
	*Env
}

// preferencesRequest takes each choice as it is, or leaves it alone when it
// is not sent: a pointer tells "no" from "not said".
type preferencesRequest struct {
	SuggestPeople *bool `json:"suggest_people"`
	SuggestGroups *bool `json:"suggest_groups"`
	UseActivity   *bool `json:"use_activity"`
	Discoverable  *bool `json:"discoverable"`
}

func (h *preferencesHandler) Read(c echo.Context) error {
	viewer := getViewer(c)
	if !signedIn(viewer) {
		return c.NoContent(http.StatusForbidden)
	}

	prefs, err := h.Repos.Preferences.Read(viewer.ID)
	if err != nil {
		log.Println("recommendations: preferences:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, prefs)
}

func (h *preferencesHandler) Edit(c echo.Context) error {
	viewer := getViewer(c)
	if !signedIn(viewer) {
		return c.NoContent(http.StatusForbidden)
	}

	req := &preferencesRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	prefs, err := h.Repos.Preferences.Read(viewer.ID)
	if err != nil {
		log.Println("recommendations: preferences:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	apply(prefs, req)

	if err := h.Repos.Preferences.Write(viewer.ID, prefs); err != nil {
		log.Println("recommendations: saving preferences:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, prefs)
}

func apply(prefs *repositories.Preferences, req *preferencesRequest) {
	if req.SuggestPeople != nil {
		prefs.SuggestPeople = *req.SuggestPeople
	}
	if req.SuggestGroups != nil {
		prefs.SuggestGroups = *req.SuggestGroups
	}
	if req.UseActivity != nil {
		prefs.UseActivity = *req.UseActivity
	}
	if req.Discoverable != nil {
		prefs.Discoverable = *req.Discoverable
	}
}
