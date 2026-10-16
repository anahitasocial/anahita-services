package handlers

import (
	"log"
	"net/http"

	"recommendation/constants"
	"recommendation/data/repositories"

	"github.com/labstack/echo/v4"
)

// What somebody has chosen about suggestions: whether to be suggested
// people, groups and events, whether what they do may be used to choose,
// whether they may be suggested to others, and how much each reason counts
// for when choosing for them. Their own to read and change, and nobody
// else's.

type preferencesHandler struct {
	*Env
}

// PreferencesRequest takes each choice as it is, or leaves it alone when it
// is not sent: a pointer tells "no" from "not said".
type PreferencesRequest struct {
	// To be suggested people.
	SuggestPeople *bool `json:"suggest_people"`
	// To be suggested groups.
	SuggestGroups *bool `json:"suggest_groups"`
	// To be suggested events.
	SuggestEvents *bool `json:"suggest_events"`
	// That what they liked and wrote may be used to choose for them.
	UseActivity *bool `json:"use_activity"`
	// That they may be suggested to other people.
	Discoverable *bool `json:"discoverable"`
	// How much each reason counts for, by its name: 0, which turns the
	// reason off, to constants.MaxWeight. The ones sent are set and the
	// others left.
	Weights map[string]float64 `json:"weights"`
	// Back to the service's own weights, and its own limit on how lately
	// somebody was here. Before any sent with it.
	ResetWeights bool `json:"reset_weights"`
	// How lately somebody must have been here to be suggested, in days:
	// one of constants.ActiveWithinChoices.
	ActiveWithinDays *int `json:"active_within_days"`
}

// PreferencesResponse is somebody's choices as they are sent: with the
// weights they are ranked by, each reason there, and the service's own to
// go back to.
type PreferencesResponse struct {
	*repositories.Preferences
	// What each reason counts for, for the viewer, by its name.
	Weights map[string]float64 `json:"weights"`
	// The service's own weights.
	DefaultWeights map[string]float64 `json:"default_weights"`
	// The most a weight may be.
	MaxWeight float64 `json:"max_weight"`
	// The reasons, in the order to list them.
	Reasons []string `json:"reasons"`
	// What active_within_days may be, nearest first, and what it is for
	// somebody who has not chosen.
	ActiveWithinChoices []int `json:"active_within_choices"`
	// What active_within_days is for somebody who has not chosen.
	ActiveWithinDefault int `json:"active_within_default"`
}

func respondPreferences(prefs *repositories.Preferences) *PreferencesResponse {
	resp := &PreferencesResponse{
		Preferences:    prefs,
		Weights:        map[string]float64{},
		DefaultWeights: map[string]float64{},
		MaxWeight:      constants.MaxWeight,
		Reasons:        constants.ReasonOrder,

		ActiveWithinChoices: constants.ActiveWithinChoices,
		ActiveWithinDefault: constants.ActiveWithinDefault,
	}

	for reason, weight := range constants.WeightsFor(prefs.Weights) {
		resp.Weights[reason] = weight.Each
	}
	for reason, weight := range constants.Weights {
		resp.DefaultWeights[reason] = weight.Each
	}

	return resp
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

	return c.JSON(http.StatusOK, respondPreferences(prefs))
}

func (h *preferencesHandler) Edit(c echo.Context) error {
	viewer := getViewer(c)
	if !signedIn(viewer) {
		return c.NoContent(http.StatusForbidden)
	}

	req := &PreferencesRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	prefs, err := h.Repos.Preferences.Read(viewer.ID)
	if err != nil {
		log.Println("recommendations: preferences:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if !apply(prefs, req) {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "invalid_weight"})
	}

	if err := h.Repos.Preferences.Write(viewer.ID, prefs); err != nil {
		log.Println("recommendations: saving preferences:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, respondPreferences(prefs))
}

// apply puts what was sent onto somebody's choices. False, with nothing
// changed that matters, for a weight that is not one: a reason there is
// not, or a number out of range.
func apply(prefs *repositories.Preferences, req *PreferencesRequest) bool {
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
	if req.SuggestEvents != nil {
		prefs.SuggestEvents = *req.SuggestEvents
	}

	if req.ResetWeights {
		prefs.Weights = nil
		prefs.ActiveWithinDays = constants.ActiveWithinDefault
	}
	if req.ActiveWithinDays != nil {
		if !constants.IsActiveWithinChoice(*req.ActiveWithinDays) {
			return false
		}
		prefs.ActiveWithinDays = *req.ActiveWithinDays
	}
	for reason, each := range req.Weights {
		if _, known := constants.Weights[reason]; !known || each < 0 || each > constants.MaxWeight {
			return false
		}
	}
	for reason, each := range req.Weights {
		if prefs.Weights == nil {
			prefs.Weights = map[string]float64{}
		}
		prefs.Weights[reason] = each
	}

	return true
}
