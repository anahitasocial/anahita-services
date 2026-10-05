package handlers

import (
	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/responses"
	"log"
	"net/http"
	"text/constants"
	"text/requests"

	"github.com/labstack/echo/v4"
)

type noteHandler struct {
	*Env
	*mediumHandler
}

// NewNoteHandler creates a new note handler
func NewNoteHandler(env *Env) *noteHandler {
	return &noteHandler{
		env,
		NewMediumHandler(env),
	}
}

// Browse returns a list of note
func (h *noteHandler) Browse(c echo.Context) error {
	var (
		err    error
		req    = requests.NewNoteRequest().Browse
		gReq   = services.NewMediaGRPCRequest().Browse
		viewer = getViewer(c)
	)

	if ok, err := h.Perms.Medium.CanBrowse(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	type Response struct {
		Data       []*responses.MediumNodeItem `json:"data"`
		Pagination responses.Pagination        `json:"pagination"`
	}
	resp := &Response{}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	gReq.Types = []string{constants.NodeMediumNoteType}
	gReq.Offset = req.Offset
	gReq.Limit = req.Limit
	gReq.Direction = req.Direction
	gReq.Ordering = req.Ordering
	gReq.OwnerID = req.OwnerID
	if err := c.Validate(gReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	resp.Pagination.Total, err = h.Srv.MediaGRPC.Count(gReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	media, err := h.Srv.MediaGRPC.Browse(gReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, medium := range media {
		if err = h.onAfterFetched(c, medium); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	batchPresignPaths(h.Env, media)

	mediaResp := &responses.MediumNodeResponse{}
	resp.Data = mediaResp.MapMediumNodeItems(media)

	return c.JSON(http.StatusOK, resp)
}

// Edit updates a note
func (h *noteHandler) Edit(c echo.Context) error {
	var (
		err    error
		req    = requests.NewNoteRequest().Edit
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
	if ok, err := h.Perms.Medium.CanEdit(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	medium.Body = req.Body
	medium.ModifiedBy = viewer.ID
	medium, err = h.Srv.MediaGRPC.Edit(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
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

// Add creates a new note
func (h *noteHandler) Add(c echo.Context) error {
	var (
		err    error
		req    = requests.NewNoteRequest().Add
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

	owner, err := h.Srv.ActorsGRPC.Read(req.OwnerID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	medium := entities.NewAnahitaNode()
	medium.Type = constants.NodeMediumNoteType
	medium.Service = constants.TextService
	medium.Access = req.Access
	medium.Body = req.Body
	medium.OwnerID = owner.ID
	medium.OwnerType = owner.Type
	// The owner as read above, merged defaults and all. It was read and
	// never attached, so CanAdd judged the empty placeholder owner — no
	// access, no add_permissions — and only the owner themselves, let
	// through before the permissions are read, could post here.
	medium.Owner = owner
	medium.CreatedBy = viewer.ID
	medium.ModifiedBy = viewer.ID
	medium.Enabled = true
	medium.CommentStatus = true
	medium.Access = an_handlers.DetermineMediumNodeAccess(req.Access, medium.Owner.Access)
	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Medium.CanAdd(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	// The audience has to be one this post can have where it is going: no
	// "admins" on a person's profile, no "only me" on somebody else's.
	if !permissions.IsMediumAudienceAllowed(mediumAccessCtx, req.Access) {
		log.Printf("refused: audience %q is not allowed on this profile", req.Access)
		return an_handlers.RefuseAudience(c, mediumAccessCtx)
	}

	medium, err = h.Srv.MediaGRPC.Add(medium, viewer)
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
	resp := mediaResp.MapMediumNode(medium)

	return c.JSON(http.StatusCreated, resp)
}
