package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/permissions"
	"anahita-libs/responses"
	"log"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// Pin pins an article, a note or a topic to the profile it is on, so that it leads the profile's
// lists. Unpin takes the pin off.
//
// PUT and DELETE /{articles,notes,topics}/:id/pin
//
// For whoever the profile belongs to: a person on their own profile, a
// group's administrators in the group. See permissions.CanPin. A profile
// has one pin (constants.MaxPinnedPosts), so pinning a post moves the pin to
// it from whichever had it.
func (h *mediumHandler) Pin(c echo.Context) error {
	return h.setPinned(c, true)
}

func (h *mediumHandler) Unpin(c echo.Context) error {
	return h.setPinned(c, false)
}

func (h *mediumHandler) setPinned(c echo.Context, pinned bool) error {
	viewer := getViewer(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	medium, err := h.Srv.MediaGRPC.Read(id, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// A reply is read in its thread. It is not pinned.
	if medium.RootID > 0 {
		return c.JSON(http.StatusUnprocessableEntity, responses.Refusal{Error: "reply_cannot_be_pinned"})
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if !permissions.CanPin(viewer, medium, mediumAccessCtx.OwnerViewerRel) {
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.Srv.MediaGRPC.SetPinned(medium.ID, pinned, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	medium, err = h.Srv.MediaGRPC.Read(id, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err = h.onAfterFetched(c, medium); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{medium})

	mediaResp := &responses.MediumNodeResponse{}
	return c.JSON(http.StatusOK, mediaResp.MapMediumNode(medium))
}
