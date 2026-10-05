package handlers

import (
	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

type mediumHandler struct {
	*Env
}

// NewMediumHandler creates a new medium handler
func NewMediumHandler(env *Env) *mediumHandler {
	return &mediumHandler{
		env,
	}
}

// Read returns a medium by ID
func (h *mediumHandler) Read(c echo.Context) error {
	var (
		req    = requests.NewMediumRequest().Read
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

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Medium.CanRead(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err = h.onAfterFetched(c, medium); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{medium})

	mediaResp := &responses.MediumNodeResponse{}
	resp := mediaResp.MapMediumNode(medium)

	return c.JSON(http.StatusOK, resp)
}

func (h *mediumHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewMediumRequest().Delete
		viewer = getViewer(c)
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	medium, err := h.Srv.MediaGRPC.Read(req.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Medium.CanDelete(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err = h.Srv.MediaGRPC.Delete(medium.ID, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

// onAfterFetched is a callback function that is called after fetching a photo
func (h *mediumHandler) onAfterFetched(c echo.Context, item *entities.AnahitaNode) error {
	var (
		err    error
		viewer = getViewer(c)
	)

	mediumAccessCtx, err := h.getMediumAccessContext(item, viewer)
	if err != nil {
		return err
	}

	item.Authorized.Edit, _ = h.Perms.Medium.CanEdit(mediumAccessCtx)
	// Who it may be shown to, for whoever may change that: the rule the
	// change-access route enforces, so the web app offers only audiences
	// that will be accepted.
	if item.Authorized.Edit {
		item.Authorized.Audiences = permissions.MediumAudiences(mediumAccessCtx)
	}
	item.Authorized.Delete, _ = h.Perms.Medium.CanDelete(mediumAccessCtx)
	item.Authorized.Administration, _ = h.Perms.Medium.CanAdministrate(mediumAccessCtx)

	// Who may comment on and like it, so the web app shows only the
	// buttons that will work.
	an_handlers.SetPostFlags(c, h.Srv.ActorsGRPC, item, viewer, mediumAccessCtx.OwnerViewerRel)

	return err
}
