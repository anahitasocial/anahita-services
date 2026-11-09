package handlers

import (
	"log"
	"net/http"

	"anahita-libs/data/entities"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/permissions"
	"anahita-libs/requests"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// EditWarning gives a post a content warning, changes it, or takes it off
// (anahita-services row 122). Whoever may edit the post may; a reply is
// changed by its author alone, as its words are. A warning a moderator gave
// stays as it is: its author is answered `warning_not_allowed`.
//
// PATCH /{articles,notes,topics}/:id/warning
func (h *mediumHandler) EditWarning(c echo.Context) error {
	var (
		req    = &requests.MediumEditWarningRequest{}
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

	medium, err := h.Srv.MediaGRPC.Read(req.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if medium.RootID > 0 {
		if medium.DeletedAt != nil {
			return c.NoContent(http.StatusGone)
		}
		if _, refused := h.replyRuling(c, medium, viewer, permissions.CanEditReply); refused != 0 {
			return c.NoContent(refused)
		}
	} else {
		if permissions.IsWebsiteItem(medium) {
			return c.JSON(http.StatusForbidden, responses.Refusal{Error: permissions.WebsiteItemRefusal})
		}
		mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		if ok, err := h.Perms.Medium.CanEdit(mediumAccessCtx); !ok {
			log.Println(err)
			return c.NoContent(http.StatusForbidden)
		}
	}

	valid, allowed := an_handlers.SetWarning(medium, req.Warning, req.WarningText)
	if !valid {
		return an_handlers.RefuseWarning(c)
	}
	if !allowed {
		return an_handlers.RefuseWarningNotAllowed(c)
	}
	medium.ModifiedBy = viewer.ID

	if medium, err = h.Srv.MediaGRPC.Edit(medium, viewer); err != nil {
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
