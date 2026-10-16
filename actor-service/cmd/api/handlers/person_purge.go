package handlers

import (
	"anahita-libs/responses"
	"log"
	"net/http"

	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/requests"

	"github.com/labstack/echo/v4"
)

// The hard delete, for site cleanup — clearing out stale and abandoned
// accounts.
//
// SUPER ADMINISTRATORS ONLY, and unlike every other permission in this service
// that check is written here rather than in CanDelete: CanDelete answers "may
// this caller remove this person", which an ordinary administrator and the owner both may. This asks something
// narrower, and conflating them would let them reach it.
//
// Separate routes from DELETE /people/:id for the same reason the RPC is
// separate from Delete — the difference between "recoverable for thirty days"
// and "gone" should not be a parameter.
//
// Nothing here waits for the purge. The person is hidden at once and queued;
// the answer carries a batch id, and GET /people/purge-batch/:batch_id says
// how far the work has got.
//
// Notifies nobody. A super administrator clearing out an account is not a conversation,
// and the grace-period notification is for people who might still act on it.

// purgeGate is what every purge route checks first: a super administrator who
// has just proved who they are. It writes the refusal and returns false.
func (h *personHandler) purgeGate(c echo.Context, viewer *entities.Viewer) bool {
	// Not IsAdmin — an ordinary administrator may schedule a deletion, which
	// is recoverable; this is not.
	if viewer == nil || viewer.PersonType != constants.PersonTypeSuperAdmin {
		log.Println("person purge: refused, viewer is not a super administrator")
		_ = c.NoContent(http.StatusForbidden)
		return false
	}

	// Prove it. The heaviest action in the system gets the same gate as the
	// reversible one, which is the floor rather than the ceiling.
	if err := h.requireStepUp(viewer.ID); err != nil {
		log.Println("person purge:", err)
		_ = stepUpRefusal(c, err)
		return false
	}

	return true
}

// Purge removes one named person.
//
//	DELETE /people/:id/purge
//
// 202 with a batch id, 409 with the reason if it cannot be purged, 200 if it
// already was.
//
// An administrator can be removed this way, deliberately and by name. The last
// super administrator cannot.
func (h *personHandler) Purge(c echo.Context) error {
	var (
		req    = requests.NewPersonRequest().Delete
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if !h.purgeGate(c, viewer) {
		return nil
	}

	return h.requestPurge(c, viewer, []int64{req.ID}, true)
}

// PurgeBatch removes many accounts at once.
//
//	POST /people/purge-batch   { "ids": [...], "confirm": "PURGE 37" }
//
// One proof of identity covers the whole request, and the typed confirmation
// must name the number of ids in it.
//
// Refused whole, with 409 and the reasons, if any of them is an administrator
// or super administrator: change their role first. A cleanup should not be
// able to remove the people who run the site.
func (h *personHandler) PurgeBatch(c echo.Context) error {
	var (
		req    = &requests.ActorPurgeBatchRequest{}
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if !h.purgeGate(c, viewer) {
		return nil
	}

	// After the gate, so somebody who may not purge learns nothing about what
	// a valid request looks like.
	if !req.Confirmed() {
		return c.JSON(http.StatusBadRequest, responses.Refusal{Error: "confirmation_mismatch"})
	}

	return h.requestPurge(c, viewer, req.IDs, false)
}

func (h *personHandler) requestPurge(c echo.Context, viewer *entities.Viewer, ids []int64, allowAdministrators bool) error {
	result, err := h.Srv.ActorsGRPC.RequestPurge(&services.ActorPurgeRequest{
		IDs:                 ids,
		Type:                constants.NodeActorPersonEntityType,
		AllowAdministrators: allowAdministrators,
	}, viewer)
	if err != nil {
		log.Println("person purge:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// One proof authorises one request, however many it names. A refused
	// request changed nothing, so it has not spent the proof.
	if len(result.Refused) == 0 {
		h.clearStepUp(viewer.ID)
	}

	return an_handlers.RespondPurgeRequested(c, result)
}

// PurgeProgress reports how far a purge request has got.
//
//	GET /people/purge-batch/:batch_id
//
// Super administrators only. No proof of identity: it changes nothing, and
// the client asks every few seconds.
func (h *personHandler) PurgeProgress(c echo.Context) error {
	var (
		req    = &requests.ActorPurgeBatchReadRequest{}
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if viewer == nil || viewer.PersonType != constants.PersonTypeSuperAdmin {
		return c.NoContent(http.StatusForbidden)
	}

	batch, err := h.Srv.ActorsGRPC.ReadPurgeBatch(req.BatchID)
	if err != nil {
		log.Println("person purge progress:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return an_handlers.RespondPurgeProgress(c, batch)
}
