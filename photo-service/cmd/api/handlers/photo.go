package handlers

import (
	"anahita-libs/cmd/api/handlers"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/requests"
	"anahita-libs/responses"
	images "anahita-libs/utils"
	"io"
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

// Add creates a new photo
func (h *photoHandler) Add(c echo.Context) error {
	var (
		req     = requests.NewMediumRequest().Add
		fileReq = services.NewFileRequest().Add
		viewer  = getViewer(c)
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

	medium := entities.NewAnahitaNode()
	medium.Type = constants.NodeMediumPhotoType
	medium.Service = constants.PhotoService
	medium.Access = req.Access
	medium.Name = req.Name
	medium.Alias = slug.Make(medium.Name)
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

	medium, err = h.Srv.MediaGRPC.Add(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Upload the photo
	file, err := c.FormFile("file")
	if err != nil {
		log.Println("Error getting file:", err)
		return c.NoContent(http.StatusBadRequest)
	}

	// Check if the file was uploaded
	if file == nil {
		log.Println("No file uploaded")
		return c.NoContent(http.StatusBadRequest)
	}

	// Open the file from the request
	fileHandler, err := file.Open()
	if err != nil {
		log.Println("Error opening file:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	defer fileHandler.Close()

	medium, err = h.Srv.MediaGRPC.Read(medium.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// Read the file content into a []byte
	fileBytes, err := io.ReadAll(fileHandler)
	if err != nil {
		log.Println("Error reading file:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Correct the orientation of the image
	fileBytes, err = images.CorrectOrientation(fileBytes)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Detect the MIME type of the corrected image
	mimeType := http.DetectContentType(fileBytes)
	if !images.IsValidMimeType(mimeType) {
		return c.NoContent(http.StatusUnsupportedMediaType)
	}

	// Check if the file size is valid
	if len(fileBytes) > constants.MaxFileSize {
		return c.NoContent(http.StatusRequestEntityTooLarge)
	}

	// Generate a unique filename based on the MIME type
	medium.Filename, err = images.GenerateUniqueFilename(mimeType)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Retrieve avatar sizes and process the image for each size
	portraits, err := medium.GetPortraits()
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, portrait := range portraits {
		// Resize the image
		content, err := images.ResizeImage(fileBytes, portrait.Size.Width, portrait.Size.Height)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		// Prepare the file for upload
		fileReq.Path = portrait.Url
		fileReq.Content = content
		fileReq.Mimetype = mimeType
		fileReq.Size = int64(len(content))
		if err := c.Validate(fileReq); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusBadRequest)
		}

		// Upload the resized image
		if _, err := h.Srv.FilesGRPC.Add(fileReq); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// Update the photo with the new filename
	medium, err = h.Srv.MediaGRPC.Edit(medium, viewer)
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

// Delete removes a photo
func (h *photoHandler) Delete(c echo.Context) error {
	var (
		err     error
		req     = requests.NewMediumRequest().Delete
		fileReq = services.NewFileRequest().Delete
		viewer  = getViewer(c)
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

	// delete image files
	portraits, err := medium.GetPortraits()
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	go func(portraits map[string]entities.AnahitaNodeImage) {
		for _, portrait := range portraits {
			fileReq.Path = portrait.Url
			if err = h.Srv.FilesGRPC.Delete(fileReq); err != nil {
				log.Println(err)
			}
		}
	}(portraits)

	if err = h.Srv.MediaGRPC.Delete(medium.ID, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

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
	item.Authorized.Delete, _ = h.Perms.Media.CanDelete(mediumAccessCtx)
	item.Authorized.Administration, _ = h.Perms.Media.CanAdministrate(mediumAccessCtx)

	// Who may comment on and like it, so the web app shows only the
	// buttons that will work.
	handlers.SetPostFlags(c, h.Srv.ActorsGRPC, item, viewer, mediumAccessCtx.OwnerViewerRel)

	return nil
}
