package handlers

import (
	"actor/responses"
	"anahita-libs/data/entities"
	"anahita-libs/requests"
	libs_responses "anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// A person's choice of Show, Warn or Hide for each kind of content warning
// (roadmap row 122, phase 2): whether a post of that kind is shown at once,
// behind its warning, or as one line that opens it. Theirs alone to read
// and change: what somebody chose not to see is nobody else's business.

// ErrContentPreferenceInvalid is a kind that is not one, or a choice that
// is not Show, Warn or Hide.
const ErrContentPreferenceInvalid = "content_preference_invalid"

// ReadContentPreferences is the viewer's choice for every kind.
//
// GET /people/:id/content-preferences
func (h *personHandler) ReadContentPreferences(c echo.Context) error {
	req := &requests.PersonContentPreferencesRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	viewer := getViewer(c)
	if viewer == nil || viewer.ID != req.ID {
		return c.NoContent(http.StatusForbidden)
	}

	choices, err := h.Srv.ActorsGRPC.ReadContentPreferences(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, responses.ContentPreferences{Choices: choices})
}

// EditContentPreferences changes the viewer's choice for the kinds named;
// the others stay as they were.
//
// PATCH /people/:id/content-preferences
func (h *personHandler) EditContentPreferences(c echo.Context) error {
	req := &requests.PersonEditContentPreferencesRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	viewer := getViewer(c)
	if viewer == nil || viewer.ID != req.ID {
		return c.NoContent(http.StatusForbidden)
	}
	for kind, choice := range req.Choices {
		if !entities.IsWarningKind(kind) || !entities.IsWarningChoice(choice) {
			return c.JSON(http.StatusBadRequest, libs_responses.Refusal{
				Error:       ErrContentPreferenceInvalid,
				Description: "each kind must be one, and each choice \"show\", \"warn\" or \"hide\"",
			})
		}
	}

	choices, err := h.Srv.ActorsGRPC.ReadContentPreferences(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	for kind, choice := range req.Choices {
		choices[kind] = choice
	}
	if choices, err = h.Srv.ActorsGRPC.SaveContentPreferences(viewer.ID, choices); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, responses.ContentPreferences{Choices: choices})
}
