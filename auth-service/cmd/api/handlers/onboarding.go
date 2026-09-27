package handlers

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Onboarding: the walk-through the web app offers after the agreements to a
// person whose profile is incomplete.
//
// The web app decides when to show it, from onboarded_at and has_bio on
// userinfo and from whether there is an avatar. Like the agreements gate it is
// a gate on the interface, not on the API: tokens are unchanged, and nothing
// here refuses a request because onboarding is outstanding.
//
// These endpoints act on THE SIGNED-IN VIEWER and take no person id, for the
// same reason /agreements does: there is nobody else whose onboarding anybody
// should be able to finish, so it is made unexpressible rather than guarded.

type onboardingHandler struct {
	*Env
}

func newOnboardingHandler(env *Env) *onboardingHandler {
	return &onboardingHandler{env}
}

// Edit records that the viewer has been through onboarding.
//
//	PATCH /onboarding
//
// Called when the person reaches the dashboard through the flow, whether they
// completed the steps or skipped them — both mean "do not send me back". No
// body: there is nothing to say but that it happened.
//
// Set once. Calling it again leaves the original date alone, and the response
// carries whichever date is stored, so a retry answers the same as the first.
//
// 200: { "onboarded_at": "<RFC 3339>" }
// 401: no signed-in person
func (h *onboardingHandler) Edit(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || viewer.ID == 0 || viewer.IsGuest() {
		return c.NoContent(http.StatusUnauthorized)
	}

	if err := h.Repos.Users.MarkOnboarded(viewer.ID); err != nil {
		log.Printf("onboarding: could not record (person=%d): %v", viewer.ID, err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Read back rather than answering with now: when the person had already
	// been through it, the stored date is the true one and now is not.
	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || user == nil {
		log.Printf("onboarding: could not read the viewer back (person=%d): %v",
			viewer.ID, err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"onboarded_at": user.OnboardedAt,
	})
}
