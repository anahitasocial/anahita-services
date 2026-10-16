package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/responses"
	images "anahita-libs/utils"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	photo "photo/constants"
	"strconv"

	"github.com/labstack/echo/v4"
)

// refuseFiles answers a request whose images were refused, saying why and
// how many a post can hold.
func refuseFiles(c echo.Context, code string) error {
	return c.JSON(refusalStatus(code), FilesRefusal{Error: code, Max: constants.PhotoMaxFiles})
}

// placeholderPhoto is a photo on this profile that does not exist yet: what
// the permission to post is judged on, and what says where an image for this
// profile is stored.
func placeholderPhoto(owner *entities.AnahitaNode, viewer *entities.Viewer) *entities.AnahitaNode {
	medium := entities.NewAnahitaNode()
	medium.Type = photo.NodeMediumPhotoType
	medium.Service = photo.PhotoService
	medium.OwnerID = owner.ID
	medium.OwnerType = owner.Type
	medium.Owner = owner
	medium.CreatedBy = viewer.ID
	medium.ModifiedBy = viewer.ID
	return medium
}

// storeImage takes one uploaded image, turns it the right way up, and stores
// every size of it for this profile.
//
// The image is always written out again by the resizing, never stored as it
// arrived, so nothing a file carries besides its picture reaches storage.
//
// Returns what was stored, or why the file was refused.
func (h *photoHandler) storeImage(c echo.Context, owner *entities.AnahitaNode, viewer *entities.Viewer, header *multipart.FileHeader) (*entities.PhotoUpload, string, error) {
	opened, err := header.Open()
	if err != nil {
		return nil, errFileUnreadable, err
	}
	defer opened.Close()

	content, err := io.ReadAll(opened)
	if err != nil {
		return nil, errFileUnreadable, err
	}

	// Checked by the rules every uploaded image is held to; see
	// images.Accept.
	content, mimeType, err := images.Accept(content, photo.MaxFileSize, photo.MaxPixels)
	if err != nil {
		return nil, refusalFor(err), err
	}

	filename, err := images.GenerateUniqueFilename(mimeType)
	if err != nil {
		return nil, "", err
	}

	portraits, err := placeholderPhoto(owner, viewer).PortraitsFor(filename)
	if err != nil {
		return nil, "", err
	}

	// The picture at the size it was taken, up to a width that keeps it
	// within what one message to storage carries. Every other size is made
	// from this and not from what arrived, so a photo of tens of megapixels
	// is opened once, not once for each size.
	largest, err := images.FitImage(content, photo.OriginalMaxWidth)
	if err != nil {
		return nil, errFileUnreadable, err
	}

	// The size recorded is that of the largest copy kept, which is what a
	// client can actually be given.
	width, height, err := images.Dimensions(largest)
	if err != nil {
		return nil, errFileUnreadable, err
	}

	for size, portrait := range portraits {
		var resized []byte

		switch size {
		case entities.PortraitSizeSquare:
			// The middle of the picture. This used to squeeze the whole of
			// it into a square.
			resized, err = images.SquareImage(largest, portrait.Size.Width)
		case entities.PortraitSizeOriginal:
			// This used to be a 320 pixel square, and it is what the
			// photo's own page and the zoom show.
			resized = largest
		default:
			resized, err = images.FitImage(largest, portrait.Size.Width)
		}
		if err != nil {
			return nil, errFileUnreadable, err
		}
		if len(resized) > photo.MaxStoredSize {
			return nil, errFileTooLarge, nil
		}

		fileReq := services.NewFileRequest().Add
		fileReq.Path = portrait.Url
		fileReq.Content = resized
		fileReq.Mimetype = mimeType
		fileReq.Size = int64(len(resized))
		if err := c.Validate(fileReq); err != nil {
			return nil, errFileUnreadable, err
		}

		if _, err := h.Srv.FilesGRPC.Add(fileReq); err != nil {
			return nil, "", err
		}
	}

	return &entities.PhotoUpload{
		OwnerID:   owner.ID,
		CreatedBy: viewer.ID,
		Filename:  filename,
		Mimetype:  mimeType,
		Width:     width,
		Height:    height,
	}, "", nil
}

// uploadImage stores the image a request carries and records it as an
// upload. Recorded even when it is about to go straight into a post: if the
// post then fails to be made, the sweeper finds the upload and removes what
// was stored.
func (h *photoHandler) uploadImage(c echo.Context, owner *entities.AnahitaNode, viewer *entities.Viewer) (*entities.PhotoUpload, string, error) {
	header, err := c.FormFile("file")
	if err != nil || header == nil {
		return nil, errFilesRequired, err
	}

	upload, refused, err := h.storeImage(c, owner, viewer, header)
	if refused != "" || err != nil {
		return nil, refused, err
	}

	upload.ID, err = h.Srv.MediaGRPC.AddPhotoUpload(upload)
	if err != nil {
		return nil, "", err
	}

	return upload, "", nil
}

// Upload stores one image for a post that has not been made yet.
//
// A post with several images is made in steps: each image in a request of
// its own, then the post from the uploads. No request then carries more than
// one image, however many the post has.
func (h *photoHandler) Upload(c echo.Context) error {
	viewer := getViewer(c)

	ownerID, err := strconv.ParseInt(c.Param("owner_id"), 10, 64)
	if err != nil || ownerID <= 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	owner, err := h.Srv.ActorsGRPC.Read(ownerID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// Whoever may not post here may not store images here either.
	mediumAccessCtx, err := h.getMediumAccessContext(placeholderPhoto(owner, viewer), viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Media.CanAdd(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

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

	// Shown the way an image of a post is, so the composer can draw it.
	preview := placeholderPhoto(owner, viewer)
	preview.Files = []*entities.PhotoFile{{
		Filename: upload.Filename,
		Width:    upload.Width,
		Height:   upload.Height,
	}}
	preview.SetFilePortraits()
	services.BatchPresignPhotoFiles(h.Srv.FilesGRPC, []*entities.AnahitaNode{preview})

	return c.JSON(http.StatusCreated, Uploaded{
		UploadID: upload.ID,
		Width:    upload.Width,
		Height:   upload.Height,
		URLs:     responses.NewPortrait(preview.Files[0].Portrait),
	})
}

// PhotoFilesRequest is the whole list of images a post is to have.
type PhotoFilesRequest struct {
	// The post's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Every image it is to have, in order. One left out is removed.
	Files []FileRef `json:"files"`
}

// EditFiles changes a post's images: their order, their descriptions, and
// which there are.
//
// The request is the whole list as it is to be. An image left out is
// removed; one named by an upload is added. That is one rule to explain, and
// no order of separate changes to get wrong.
func (h *photoHandler) EditFiles(c echo.Context) error {
	var (
		req    = &PhotoFilesRequest{}
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

	existing, err := h.Srv.MediaGRPC.BrowsePhotoFiles([]int64{medium.ID})
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	uploads, err := h.Srv.MediaGRPC.ReadPhotoUploads(uploadIDs(req.Files))
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	plan, refused := planFiles(existing, uploads, req.Files, viewer.ID, medium.GetOwnerID())
	if refused != "" {
		return refuseFiles(c, refused)
	}

	if _, err := h.Srv.MediaGRPC.SetPhotoFiles(medium.ID, plan.Files, plan.UploadIDs); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Only now that the post no longer names them.
	h.deleteStored(medium, plan.Removed)

	// Read again: the first image may have changed, and the post carries a
	// copy of its name. Then marked as edited, by whoever did it.
	medium, err = h.Srv.MediaGRPC.Read(medium.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
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

// uploadIDs are the uploads a list of images names.
func uploadIDs(refs []FileRef) []int64 {
	var ids []int64
	for _, ref := range refs {
		if ref.UploadID > 0 {
			ids = append(ids, ref.UploadID)
		}
	}
	return ids
}

// deleteStored removes every stored size of these images, in the background.
// A failure is logged and no more: the post no longer names them, so there
// is nothing to try again from.
func (h *photoHandler) deleteStored(medium *entities.AnahitaNode, files []*entities.PhotoFile) {
	var paths []string
	for _, file := range files {
		portraits, err := medium.PortraitsFor(file.Filename)
		if err != nil {
			continue
		}
		for _, portrait := range portraits {
			paths = append(paths, portrait.Url)
		}
	}

	if len(paths) == 0 {
		return
	}

	go func() {
		for _, path := range paths {
			fileReq := services.NewFileRequest().Delete
			fileReq.Path = path
			if err := h.Srv.FilesGRPC.Delete(fileReq); err != nil {
				log.Println(err)
			}
		}
	}()
}
