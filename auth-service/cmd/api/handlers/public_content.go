package handlers

import (
	"fmt"
	"log"
	"net/http"

	"anahita-libs/policy"

	"github.com/labstack/echo/v4"
)

// What is public on the installation, and making all of it registered-only.
//
// For an installation that has set MIN_CONTENT_ACCESS=registered. The setting
// stops anything new being public and leaves alone what already is; these two
// routes are how a super administrator brings the rest into line, once.

type publicContentResponse struct {
	People int64 `json:"people"`
	Groups int64 `json:"groups"`
	Posts  int64 `json:"posts"`
	Total  int64 `json:"total"`

	// Whether new public content is still allowed here. While it is, the
	// conversion is refused: it would be undone by the next public post.
	PublicAllowed bool `json:"public_allowed"`
}

// PublicContent says how much on the installation is public.
//
//	GET /accounts/public-content
//
// Super administrators only. It is what the conversion would change, shown
// before somebody confirms it.
func (h *accountsHandler) PublicContent(c echo.Context) error {
	viewer := getViewer(c)

	if err := h.Perms.Account.CanBrowse(viewer); err != nil {
		log.Println("public content:", err)
		return c.NoContent(http.StatusForbidden)
	}

	counts, err := h.Srv.ActorsGRPC.PublicAccessCounts()
	if err != nil {
		log.Println("public content:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, &publicContentResponse{
		People:        counts.People,
		Groups:        counts.Groups,
		Posts:         counts.Posts,
		Total:         counts.People + counts.Groups + counts.Posts,
		PublicAllowed: policy.AllowsPublic(),
	})
}

type makeRegisteredRequest struct {
	// The number of things that will change, typed out: "CONVERT 8675".
	Confirm string `json:"confirm" validate:"required,max=30"`
}

// MakeRegistered makes every public person, group and post visible to
// signed-in people only.
//
//	POST /accounts/make-registered   { "confirm": "CONVERT 8675" }
//
// It cannot be undone, so it asks for the same three things a purge does: a
// super administrator, a proof of identity, and the number typed out.
//
//	200  done, with how many of each were changed
//	400  confirmation_mismatch: the number is not how much is public now
//	403  step_up_required, or not a super administrator
//	409  public_content_allowed: MIN_CONTENT_ACCESS is not "registered"
func (h *accountsHandler) MakeRegistered(c echo.Context) error {
	viewer := getViewer(c)

	if err := h.Perms.Account.CanMakeRegistered(viewer); err != nil {
		log.Println("make registered:", err)
		return c.NoContent(http.StatusForbidden)
	}

	// Refused while anything new may still be public. Converting then would
	// be undone by the next person to post, and somebody pressing this by
	// mistake on an open site would have hidden the whole of it from
	// visitors.
	if policy.AllowsPublic() {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "public_content_allowed",
			"message": "This installation still allows public content. Set " +
				policy.EnvMinContentAccess + "=registered first.",
		})
	}

	req := &makeRegisteredRequest{}
	if err := c.Bind(req); err != nil {
		log.Println("make registered:", err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println("make registered:", err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("make registered:", err)
		return stepUpRequired(c, err)
	}

	counts, err := h.Srv.ActorsGRPC.PublicAccessCounts()
	if err != nil {
		log.Println("make registered:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Against what is public NOW, not when the dialog was opened: if it has
	// changed since, the person confirmed a different number.
	total := counts.People + counts.Groups + counts.Posts
	if req.Confirm != fmt.Sprintf("CONVERT %d", total) {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": "confirmation_mismatch",
			"total": total,
		})
	}

	changed, err := h.Srv.ActorsGRPC.MakePublicRegistered(viewer)
	if err != nil {
		log.Println("make registered:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// One proof, one conversion.
	h.clearStepUp(viewer.ID)

	log.Printf("make registered: %d people, %d groups and %d posts (by person=%d)",
		changed.People, changed.Groups, changed.Posts, viewer.ID)

	return c.JSON(http.StatusOK, &publicContentResponse{
		People: changed.People,
		Groups: changed.Groups,
		Posts:  changed.Posts,
		Total:  changed.People + changed.Groups + changed.Posts,
	})
}
