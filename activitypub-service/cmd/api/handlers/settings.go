package handlers

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// A person's choice to be on the fediverse.
//
// Nobody is unless they say so. What is sent to another server cannot be
// called back: it is copied by servers this one has never heard of. So it
// is a switch each person turns on for themselves, it is off until they
// do, and it can only be turned on by somebody whose profile is already
// open to everybody.

// Settings is what a person is shown about being on the fediverse.
type Settings struct {
	// Whether they could be, and why not when they could not.
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	// Whether they are.
	Enabled bool `json:"enabled"`
	// What they are, or would be, called from other servers, and where
	// those servers find them.
	Handle string `json:"handle,omitempty"`
	Actor  string `json:"actor,omitempty"`
}

type settingsRequest struct {
	Enabled *bool `json:"enabled" validate:"required"`
}

// ReadSettings says whether the viewer is on the fediverse and could be.
func (h *Handlers) ReadSettings(c echo.Context) error {
	settings, err := h.settingsOf(getViewer(c).ID)
	if err != nil {
		log.Println("activitypub settings:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	return c.JSON(http.StatusOK, settings)
}

// EditSettings turns it on or off for the viewer.
func (h *Handlers) EditSettings(c echo.Context) error {
	viewer := getViewer(c)

	req := &settingsRequest{}
	if err := c.Bind(req); err != nil || req.Enabled == nil {
		return refuse(c, http.StatusBadRequest, "enabled_required")
	}

	settings, err := h.settingsOf(viewer.ID)
	if err != nil {
		log.Println("activitypub settings:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}

	// Off is always allowed. On only when they could be.
	if *req.Enabled && !settings.Available {
		return refuse(c, http.StatusConflict, settings.Reason)
	}

	if *req.Enabled {
		// The key is made before anybody is told they are here, so that
		// the first server to ask finds it.
		if _, err := h.publicKeyOf(viewer.ID); err != nil {
			log.Println("activitypub key:", err)
			return refuse(c, http.StatusInternalServerError, "unavailable")
		}
	}

	if err := h.Accounts.Set(viewer.ID, *req.Enabled); err != nil {
		log.Println("activitypub settings:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}

	settings.Enabled = *req.Enabled
	return c.JSON(http.StatusOK, settings)
}

func (h *Handlers) settingsOf(personID int64) (*Settings, error) {
	settings := &Settings{}

	person, err := h.Nodes.Read(personID)
	if err != nil || person == nil {
		return nil, err
	}

	account, err := h.Accounts.Read(personID)
	if err != nil {
		return nil, err
	}
	settings.Enabled = account.Enabled

	if ok, reason := h.siteFederates(); !ok {
		settings.Reason = reason
		// A site that does not federate has nobody on the fediverse,
		// whatever they once chose.
		settings.Enabled = false
		return settings, nil
	}

	settings.Handle = h.Site.Handle(person.Alias)
	settings.Actor = h.addresses.Actor(person.UUID)

	if !h.mayFederate(person) {
		settings.Reason = reasonProfileNotPublic
		return settings, nil
	}

	settings.Available = true
	return settings, nil
}
