package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/handlers"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/permissions"
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
	// The viewer's own media library: all their own, with what they put up
	// from inside the article editor, which no other list holds.
	if req.Library {
		if viewer.ID <= 0 {
			return c.NoContent(http.StatusForbidden)
		}
		gReq.OwnerID = viewer.ID
		gReq.Library = true
	}
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
	// What the viewer may do with each, as a post's own page says.
	handlers.SetListPostFlags(c, h.Srv.ActorsGRPC, h.Srv.SocialgraphGRPC, nodes, viewer)

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
	if ok, err := h.mayRead(c, medium, viewer, mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	// Out of sight for every reader where the profile it is on has blocked
	// its writer (docs/permissions.md): the graph's own judgement, which the
	// lists use too. 404, as for anything the viewer may not see.
	if !permissions.CanReadByID(h.Srv.GraphNodeGRPC, medium.ID, viewer) {
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

// Edit updates a photo
func (h *photoHandler) Edit(c echo.Context) error {
	var (
		req    = &PhotoEditRequest{}
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
	if ok, err := h.mayEdit(c, medium, viewer, mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	medium.Name = req.Title
	medium.Alias = slug.Make(medium.Name)
	medium.Body = req.Content

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

// PhotoAddRequest is a new photo post.
//
// Sent one of two ways. As JSON, naming the images that were uploaded for it:
// the way to make a post with several. Or as a form carrying one image in
// "file", which is how a photo was posted before it could hold more, and
// still works.
//
// The title and the words are both optional, as for a reply with pictures:
// the pictures are the post.
type PhotoAddRequest struct {
	// The profile it is posted on.
	OwnerID int64 `param:"owner_id" validate:"required,numeric"`
	// Its title, if it has one.
	Title string `form:"title" validate:"omitempty,max=100"`
	// Its words, as Markdown, if it has any.
	Content string `form:"content" validate:"omitempty,max=8000"`
	// Who may see it (permissions.md). The widest the installation allows
	// when left out.
	Access string `form:"access" validate:"omitempty,oneof=public registered followers leaders mutuals admins myself"`
	// The language it is written in, as a BCP 47 tag: "en", "fr-CA".
	Language string `form:"language" validate:"omitempty,max=12"`
	// Who may reply: "anyone", "nobody", or any of "followers", "following"
	// and "mentioned" joined by commas. Left out, anyone the profile lets.
	ReplyAccess string `json:"reply_access" form:"reply_access" validate:"omitempty,max=64"`
	// Who may quote it: "anyone", "followers" or "nobody". Left out, the
	// author's own setting applies.
	QuotePolicy string `json:"quote_policy" form:"quote_policy" validate:"omitempty,max=16"`
	// The images, in order, each an upload made for this profile.
	Uploads []FileRef `json:"uploads" form:"-"`
	// What the one image of a form post shows.
	AltText string `json:"-" form:"alt_text"`
	// Whether whoever can see it may download it. Left out, the post
	// starts with what the profile it is on says.
	AllowDownload *bool `json:"allow_download" form:"allow_download"`
	// In the author's media library only: put up from inside the article
	// editor to be shown in an article, and listed and announced nowhere.
	// Only on the author's own profile.
	LibraryOnly bool `json:"library_only" form:"library_only"`
	// The locations its author chose for it, tied to it as it is made, so
	// that it goes to other servers with them: the first as its
	// `location`, all as `Place`s in its `tag` (roadmap row 82, part 3b).
	// Up to five; what is not a location is passed over.
	LocationIDs []int64 `json:"location_ids" form:"location_ids" validate:"omitempty,max=5,dive,gt=0"`
}

// NewPhotoAddRequest is a photo post as the handler starts it: open to as
// many as the installation lets, as any post.
func NewPhotoAddRequest() *PhotoAddRequest {
	return &PhotoAddRequest{Access: requests.NewMediumRequest().Add.Access}
}

// PhotoEditRequest changes what a photo post says. The title and the words
// are both optional, as when it is posted.
type PhotoEditRequest struct {
	// The post's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Its title, if it has one.
	Title string `form:"title" validate:"omitempty,max=100"`
	// Its words, as Markdown, if it has any.
	Content string `form:"content" validate:"omitempty,max=8000"`
	// The language it is written in, as a BCP 47 tag.
	Language string `form:"language" validate:"omitempty,max=12"`
}

// Add creates a new photo
func (h *photoHandler) Add(c echo.Context) error {
	var (
		req    = NewPhotoAddRequest()
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
	medium.Name = req.Title
	medium.Alias = slug.Make(medium.Name)
	medium.Body = req.Content

	// The language the author says it is written in. Not saying is not an
	// error; a tag that is not one is.
	if language, valid := handlers.RequestLanguage(req.Language); !valid {
		return handlers.RefuseLanguage(c)
	} else if language != "" {
		medium.Language = language
	}
	medium.Enabled = true
	medium.CommentStatus = true
	medium.AllowDownload = handlers.DefaultAllowDownload(req.AllowDownload, owner)
	// A library is one's own: on one's own profile and nowhere else.
	if req.LibraryOnly && owner.ID != viewer.ID {
		return c.NoContent(http.StatusBadRequest)
	}
	medium.LibraryOnly = req.LibraryOnly

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
		wanted = []FileRef{{UploadID: upload.ID, AltText: req.AltText}}
	}

	plan, refused := planFiles(nil, uploads, wanted, viewer.ID, owner.ID)
	if refused != "" {
		return refuseFiles(c, refused)
	}

	// The post names its first image itself, for everything that shows one.
	medium.Filename = plan.Files[0].Filename
	medium.Mimetype = plan.Files[0].Mimetype

	medium, err = h.Srv.MediaGRPC.AddWithLocations(medium, viewer, req.LocationIDs)
	// A group's or an event's rules changed since they agreed: agreed to
	// again first (roadmap row 102, part 5c).
	if an_handlers.IsRulesNotAgreed(err) {
		return an_handlers.RefuseRulesNotAgreed(c)
	}
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
	if ok, err := h.mayDelete(c, medium, viewer, mediumAccessCtx); !ok {
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
