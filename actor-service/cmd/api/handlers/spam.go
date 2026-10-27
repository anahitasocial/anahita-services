package handlers

import (
	"log"
	"net/http"

	"anahita-libs/constants"
	"anahita-libs/data/services"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/requests"

	"github.com/labstack/echo/v4"
)

// Removing an account as spam (roadmap row 102): a moderator's decision, so
// an administrator's as well as a super administrator's, and at once and for
// good, without the thirty days a deleted account waits. "By the time we get
// to it, it might be too late" (the user, 2026-10-19).
//
// The purge a super administrator asks for, with three differences: an
// administrator may; never an administrator's or a super administrator's
// account, which a cleanup does not touch either, nor the viewer's own; and
// recorded as removed-as-spam with who decided. No proof of identity: the app
// asks once more before sending it, and removing spam is done often and in
// a hurry.

// RemoveAsSpam removes a person as spam.
//
//	POST /people/:id/spam
func (h *personHandler) RemoveAsSpam(c echo.Context) error {
	return removeAsSpam(c, h.Env, constants.NodeActorPersonEntityType)
}

// RemoveAsSpam removes a group as spam.
//
//	POST /groups/:id/spam
func (h *groupHandler) RemoveAsSpam(c echo.Context) error {
	return removeAsSpam(c, h.Env, h.nodeType)
}

func removeAsSpam(c echo.Context, env *Env, nodeType string) error {
	req := requests.NewActorRequest().Delete
	viewer := getViewer(c)
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if viewer == nil || !viewer.IsAdmin() {
		log.Println("remove as spam: refused, the viewer is not an administrator")
		return c.NoContent(http.StatusForbidden)
	}

	result, err := env.Srv.ActorsGRPC.RequestPurge(&services.ActorPurgeRequest{
		IDs:    []int64{req.ID},
		Type:   nodeType,
		Reason: constants.PurgeReasonSpam,
		// Never somebody who runs the site, as a cleanup never is.
		AllowAdministrators: false,
	}, viewer)
	if err != nil {
		log.Println("remove as spam:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return an_handlers.RespondPurgeRequested(c, result)
}
