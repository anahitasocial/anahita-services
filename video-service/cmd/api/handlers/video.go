package handlers

import (
	"anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"anahita-libs/videos"
	"log"
	"net/http"
	"video/constants"

	"github.com/gosimple/slug"
	"github.com/labstack/echo/v4"
)

type videoHandler struct {
	*Env
}

// NewVideoHandler creates a new video handler
func NewVideoHandler(env *Env) *videoHandler {
	return &videoHandler{
		env,
	}
}

// Browse returns a list of video
func (h *videoHandler) Browse(c echo.Context) error {
	var (
		err    error
		req    = requests.NewMediumRequest().Browse
		gReq   = services.NewMediaGRPCRequest().Browse
		viewer = getViewer(c)
	)

	if ok, err := h.Perms.Media.CanBrowse(viewer); !ok {
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

	gReq.Types = []string{constants.NodeMediumVideoType}
	gReq.Offset = req.Offset
	gReq.Limit = req.Limit
	gReq.Ordering = req.Ordering
	gReq.Direction = req.Direction
	gReq.Disabled = req.Disabled
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

	nodes, err := h.Srv.MediaGRPC.Browse(gReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	for _, node := range nodes {
		if err = h.onAfterFetched(c, node); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	batchPresignPaths(h.Env, nodes)

	mediaResp := &responses.MediumNodeResponse{}
	resp.Data = mediaResp.MapMediumNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// Read returns a video
func (h *videoHandler) Read(c echo.Context) error {
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
	if err != nil || medium == nil || medium.Type != constants.NodeMediumVideoType {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.mayRead(c, medium, viewer, mediumAccessCtx); !ok {
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

// VideoEditRequest changes what a video post says. As for any post, but
// for its text: a video needs a title and may have nothing more said of it.
type VideoEditRequest struct {
	// The post's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Its title.
	Name string `json:"name" form:"name" validate:"required,max=100"`
	// Its words, as Markdown.
	Body string `json:"body" form:"body" validate:"omitempty,max=8000"`
	// The language it is written in, as a BCP 47 tag.
	Language string `json:"language" form:"language" validate:"omitempty,max=12"`
}

// Edit updates a video
func (h *videoHandler) Edit(c echo.Context) error {
	var (
		req    = &VideoEditRequest{}
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
	if err != nil || medium == nil || medium.Type != constants.NodeMediumVideoType {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.mayEdit(c, medium, viewer, mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	medium.Name = req.Name
	medium.Alias = slug.Make(medium.Name)
	medium.Body = req.Body

	// The language the author says it is written in. Not saying is not an
	// error; a tag that is not one is.
	if language, valid := handlers.RequestLanguage(req.Language); !valid {
		return handlers.RefuseLanguage(c)
	} else if language != "" {
		medium.Language = language
	}
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

// Delete removes a video
func (h *videoHandler) Delete(c echo.Context) error {
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
	if err != nil || medium == nil || medium.Type != constants.NodeMediumVideoType {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.mayDelete(c, medium, viewer, mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	// Its files, read before the post goes: afterwards nothing says which
	// they were.
	video := videos.FromMeta(medium.Meta)

	if err = h.Srv.MediaGRPC.Delete(medium.ID, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.deleteStored(video.Keys())

	return c.NoContent(http.StatusOK)
}

// onAfterFetched is a callback function that is called after fetching a video
func (h *videoHandler) onAfterFetched(c echo.Context, item *entities.AnahitaNode) error {
	viewer := getViewer(c)

	// A reply: what may be done with it is its thread's to say.
	if item.RootID > 0 {
		if item.Authorized == nil {
			item.Authorized = &entities.AnahitaNodeAuthorized{}
		}
		if thread, err := handlers.Thread(c, h.readers(), item.RootID, viewer); err == nil {
			handlers.SetReplyFlags(thread, item)
		}
		return nil
	}

	mediumAccessCtx, err := h.getMediumAccessContext(item, viewer)
	if err != nil {
		return err
	}

	item.Authorized.Edit, _ = h.Perms.Media.CanEdit(mediumAccessCtx)
	// Who it may be shown to, for whoever may change that: the rule the
	// change-access route enforces, so the web app offers only audiences
	// that will be accepted.
	if item.Authorized.Edit {
		item.Authorized.Audiences = permissions.MediumAudiences(mediumAccessCtx)
	}
	item.Authorized.Delete, _ = h.Perms.Media.CanDelete(mediumAccessCtx)
	item.Authorized.Administration, _ = h.Perms.Media.CanAdministrate(mediumAccessCtx)

	// Who may comment on and like it, so the web app shows only the
	// buttons that will work.
	handlers.SetPostFlags(c, h.Srv.ActorsGRPC, item, viewer, mediumAccessCtx.OwnerViewerRel)

	return nil
}
