package handlers

import (
	"anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"
	"photo/constants"

	"github.com/gosimple/slug"
	"github.com/labstack/echo/v4"
)

type photoHandler struct {
	*Env
}

// NewPhotoHandler creates a new photo handler
func NewPhotoHandler(env *Env) *photoHandler {
	return &photoHandler{
		env,
	}
}

// Browse returns a list of photo
func (h *photoHandler) Browse(c echo.Context) error {
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

	gReq.Types = []string{constants.NodeMediumPhotoType}
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

// Read returns a photo
func (h *photoHandler) Read(c echo.Context) error {
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
	if ok, err := h.Perms.Media.CanRead(mediumAccessCtx); !ok {
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

// Edit updates a photo
func (h *photoHandler) Edit(c echo.Context) error {
	var (
		req    = requests.NewMediumRequest().Edit
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
	if ok, err := h.Perms.Media.CanEdit(mediumAccessCtx); !ok {
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

// photoAddRequest is a new photo post.
//
// Sent one of two ways. As JSON, naming the images that were uploaded for it:
// the way to make a post with several. Or as a form carrying one image in
// "file", which is how a photo was posted before it could hold more, and
// still works.
type photoAddRequest struct {
	requests.MediumAddRequest
	// The images, in order, each an upload made for this profile.
	Uploads []fileRef `json:"uploads" form:"-"`
	// What the one image of a form post shows.
	AltText string `json:"-" form:"alt_text"`
}

// Add creates a new photo
func (h *photoHandler) Add(c echo.Context) error {
	var (
		req    = &photoAddRequest{MediumAddRequest: *requests.NewMediumRequest().Add}
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

	owner, err := h.Srv.ActorsGRPC.Read(req.OwnerID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	medium := placeholderPhoto(owner, viewer)
	medium.Access = req.Access
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
	medium.Enabled = true
	medium.CommentStatus = true

	// Who may reply, when the author says. Not saying leaves it to the
	// profile; a value that is not one is an error.
	if replyAccess, valid := handlers.RequestReplyAccess(req.ReplyAccess); !valid {
		return handlers.RefuseReplyAccess(c)
	} else if replyAccess != "" {
		handlers.SetReplyAccess(medium, replyAccess)
	}

	// Who may quote it, when the author says. Not saying leaves it to
	// their own setting.
	if quotePolicy, valid := handlers.RequestQuotePolicy(req.QuotePolicy); !valid {
		return handlers.RefuseQuotePolicy(c)
	} else if quotePolicy != "" {
		medium.QuotePolicy = quotePolicy
	}
	medium.Access = handlers.DetermineMediumNodeAccess(req.Access, medium.Owner.Access)

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Media.CanAdd(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	// The audience has to be one this post can have where it is going: no
	// "admins" on a person's profile, no "only me" on somebody else's.
	if !permissions.IsMediumAudienceAllowed(mediumAccessCtx, req.Access) {
		log.Printf("refused: audience %q is not allowed on this profile", req.Access)
		return handlers.RefuseAudience(c, mediumAccessCtx)
	}

	// The images are settled before the post is made. It used to be made
	// first, so a file that turned out not to be an image left a photo post
	// with no photo.
	wanted := req.Uploads
	var uploads []*entities.PhotoUpload

	if len(wanted) > 0 {
		uploads, err = h.Srv.MediaGRPC.ReadPhotoUploads(uploadIDs(wanted))
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	} else {
		upload, refused, err := h.uploadImage(c, owner, viewer)
		if err != nil {
			log.Println(err)
		}
		if refused != "" {
			return refuseFiles(c, refused)
		}
		if err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}

		uploads = []*entities.PhotoUpload{upload}
		wanted = []fileRef{{UploadID: upload.ID, AltText: req.AltText}}
	}

	plan, refused := planFiles(nil, uploads, wanted, viewer.ID, owner.ID)
	if refused != "" {
		return refuseFiles(c, refused)
	}

	// The post names its first image itself, for everything that shows one.
	medium.Filename = plan.Files[0].Filename
	medium.Mimetype = plan.Files[0].Mimetype

	medium, err = h.Srv.MediaGRPC.Add(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if _, err := h.Srv.MediaGRPC.SetPhotoFiles(medium.ID, plan.Files, plan.UploadIDs); err != nil {
		log.Println(err)
		// A photo post with no images is worse than no post. The uploads
		// are still uploads, so the same request can be sent again.
		if err := h.Srv.MediaGRPC.Delete(medium.ID, viewer); err != nil {
			log.Println(err)
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	medium, err = h.Srv.MediaGRPC.Read(medium.ID, viewer)
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

	return c.JSON(http.StatusCreated, resp)
}

// Delete removes a photo
func (h *photoHandler) Delete(c echo.Context) error {
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
	if ok, err := h.Perms.Media.CanDelete(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	// Every image of the post, read before the post goes: afterwards nothing
	// says which they were. If the list cannot be read, the one image the
	// post itself names is still removed.
	files, err := h.Srv.MediaGRPC.BrowsePhotoFiles([]int64{medium.ID})
	if err != nil {
		log.Println(err)
	}
	if len(files) == 0 && medium.Filename != "" {
		files = []*entities.PhotoFile{{Filename: medium.Filename}}
	}

	if err = h.Srv.MediaGRPC.Delete(medium.ID, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.deleteStored(medium, files)

	return c.NoContent(http.StatusOK)
}

// onAfterFetched is a callback function that is called after fetching a photo
func (h *photoHandler) onAfterFetched(c echo.Context, item *entities.AnahitaNode) error {
	viewer := getViewer(c)

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
