package handlers

import (
	"log"
	"net/http"

	"anahita-libs/constants"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// PreferencesResponse is the viewer's own steps for search's Most relevant:
// how much each sign that something was found useful counts for them, as a
// step that multiplies the installation's own weight.
type PreferencesResponse struct {
	// By name, what they have set; what is not here is 1, as the
	// installation has it.
	Steps map[string]float64 `json:"steps"`
	// What may be set: `like`, `save`, `reply`, `follow`, `cite` and
	// `following` (what the viewer follows coming first).
	Adjustable []string `json:"adjustable"`
	// What each may be set to: 0 (off), 0.5, 1 (as is), 1.5 and 2.
	Options []float64 `json:"options"`
}

// PreferencesRequest is the whole of the viewer's steps, replacing what
// there was. A name left out is as the installation has it.
type PreferencesRequest struct {
	Steps map[string]float64 `json:"steps"`
}

type preferencesHandler struct {
	*Env
}

func NewPreferencesHandler(env *Env) *preferencesHandler {
	return &preferencesHandler{env}
}

func answer(steps map[string]float64) PreferencesResponse {
	if steps == nil {
		steps = map[string]float64{}
	}
	return PreferencesResponse{
		Steps:      steps,
		Adjustable: constants.SearchAdjustable,
		Options:    constants.SearchSteps,
	}
}

// Read is the viewer's own steps.
func (h *preferencesHandler) Read(c echo.Context) error {
	viewer := getViewer(c)
	steps, err := h.Srv.SearchGRPC.ReadPreferences(viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, answer(steps))
}

// Edit keeps the viewer's steps. A name that may not be set, or a value
// that is not a step, refuses the whole and changes nothing.
func (h *preferencesHandler) Edit(c echo.Context) error {
	viewer := getViewer(c)
	req := &PreferencesRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	for name, step := range req.Steps {
		if !contains(constants.SearchAdjustable, name) || !isStep(step) {
			return c.JSON(http.StatusUnprocessableEntity, responses.Refusal{Error: "invalid_weight"})
		}
	}
	steps, err := h.Srv.SearchGRPC.SavePreferences(viewer, req.Steps)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, answer(steps))
}

func contains(names []string, name string) bool {
	for _, each := range names {
		if each == name {
			return true
		}
	}
	return false
}

func isStep(value float64) bool {
	for _, step := range constants.SearchSteps {
		if step == value {
			return true
		}
	}
	return false
}
