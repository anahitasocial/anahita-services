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

// Read returns what the flow needs about the viewer that userinfo does not
// carry.
//
//	GET /onboarding
//
// Today that is only who invited them, so the featured step can show the
// inviter first. By alias, because that is what the web app resolves a
// profile by: it reads the inviter through /people/:alias, which is where
// blocks and profile access are decided — not here, where the answer would be
// a second, drifting copy of that check.
//
// invited_by is null when nobody invited them, and when the inviter no longer
// resolves: deleted, disabled, or gone. The step then simply has no inviter
// to show, which is the same thing as never having had one.
//
// 200: { "invited_by": "<alias>" | null }
// 401: no signed-in person
func (h *onboardingHandler) Read(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || viewer.ID == 0 || viewer.IsGuest() {
		return c.NoContent(http.StatusUnauthorized)
	}

	user, err := h.Repos.Users.ReadByID(viewer.ID)
	if err != nil || user == nil {
		log.Printf("onboarding read: could not read the viewer (person=%d): %v",
			viewer.ID, err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"invited_by": h.inviterAlias(user.InvitedBy),
	})
}

// inviterAlias resolves an inviter to the handle the web app reads them by, or
// nil when there is nobody to show.
//
// A failed lookup is nil, not an error. The inviter is a nicety at the top of
// one step; a person whose inviter has since deleted their account must still
// get through onboarding, and a 500 here would take the whole step with it.
func (h *onboardingHandler) inviterAlias(id int64) any {
	if id == 0 {
		return nil
	}

	inviter, err := h.Repos.Users.ReadByID(id)
	if err != nil || inviter == nil {
		log.Printf("onboarding read: inviter %d does not resolve: %v", id, err)
		return nil
	}

	// Disabled, or scheduled for deletion — which also disables. Offering to
	// follow either is offering something that will not work.
	if !inviter.Enabled || inviter.DeletedAt != nil || inviter.Alias == "" {
		return nil
	}

	return inviter.Alias
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
