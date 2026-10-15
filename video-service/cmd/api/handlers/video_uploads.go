package handlers

import (
	"errors"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/videos"
	"video/constants"
	"video/data/repositories"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// A video is too big to be sent to this service. The browser sends it
// straight to storage, in parts, at addresses asked for here:
//
//	POST   /videos/:owner_id/uploads      begin: the post and the file's size
//	POST   /videos/uploads/:id/parts      addresses for some of its parts
//	POST   /videos/uploads/:id/complete   all sent: make it a video
//	GET    /videos/uploads/:id            how it is getting on
//	GET    /videos/uploads                what the viewer has in hand
//	DELETE /videos/uploads/:id            give it up
//
// The post is described when the upload begins and made when the video is
// ready, by the worker. Until then there is no post: nothing to hide from
// feeds, and nothing left behind by an upload that is never finished.

// uploadStartRequest is a video about to be sent, and the post it will be.
type uploadStartRequest struct {
	OwnerID int64 `param:"owner_id" validate:"required,numeric"`
	// Required of a post; a reply need not have one.
	Name        string `json:"name" form:"name" validate:"omitempty,max=100"`
	Body        string `json:"body" form:"body" validate:"omitempty,max=8000"`
	Access      string `json:"access" form:"access" validate:"omitempty,oneof=public registered followers leaders mutuals admins myself"`
	Language    string `json:"language" form:"language" validate:"omitempty,max=12"`
	ReplyAccess string `json:"reply_access" form:"reply_access" validate:"omitempty,max=64"`
	QuotePolicy string `json:"quote_policy" form:"quote_policy" validate:"omitempty,max=16"`
	// Whether whoever can see it may download it. Left out, the post
	// starts with what the profile it is on says.
	AllowDownload *bool `json:"allow_download" form:"allow_download"`
	// What it answers, when it is a reply: then it is on its author's own
	// profile, and what it is open to, who may answer it and quoting are
	// its thread's, so those above are not read.
	ParentID int64 `json:"parent_id" form:"parent_id" validate:"omitempty,min=1"`
	// What the browser says it is sending. The worker decides what it is.
	Size        int64  `json:"size" form:"size" validate:"required,min=1"`
	ContentType string `json:"content_type" form:"content_type" validate:"required,max=100"`
}

type uploadPartsRequest struct {
	Numbers []int32 `json:"numbers" validate:"required,min=1,max=100"`
}

// uploadResponse is an upload as its uploader is told of it.
type uploadResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Name    string `json:"name"`
	OwnerID int64  `json:"owner_id"`
	// Why it failed.
	Error string `json:"error,omitempty"`
	// The post, once it is one.
	NodeID int64 `json:"node_id,omitempty"`
	// What it answers, when it is a reply.
	ParentID int64 `json:"parent_id,omitempty"`
	// How it is to be sent, while it is being.
	PartSize  int64     `json:"part_size,omitempty"`
	PartCount int64     `json:"part_count,omitempty"`
	Size      int64     `json:"size,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func newUploadResponse(upload *repositories.Upload) *uploadResponse {
	resp := &uploadResponse{
		ID:        upload.ID,
		Status:    upload.Status,
		Name:      upload.Name,
		OwnerID:   upload.OwnerID,
		Error:     upload.Error,
		NodeID:    upload.NodeID,
		ParentID:  upload.ParentID,
		CreatedAt: upload.CreatedAt,
	}
	if upload.Status == videos.StatusUploading {
		resp.Size = upload.DeclaredBytes
		resp.PartSize = constants.PartSize
		resp.PartCount = (upload.DeclaredBytes + constants.PartSize - 1) / constants.PartSize
	}
	return resp
}

func refuseUpload(c echo.Context, status int, code string) error {
	return c.JSON(status, map[string]any{"error": code})
}

// placeholderVideo is a video on this profile that does not exist yet:
// what the permission to post is judged on.
func placeholderVideo(owner *entities.AnahitaNode, viewer *entities.Viewer) *entities.AnahitaNode {
	medium := entities.NewAnahitaNode()
	medium.Type = constants.NodeMediumVideoType
	medium.Service = constants.VideoService
	medium.OwnerID = owner.ID
	medium.OwnerType = owner.Type
	medium.Owner = owner
	medium.CreatedBy = viewer.ID
	medium.ModifiedBy = viewer.ID
	return medium
}

// UploadStart begins a video: everything about the post is settled here,
// as it is when any post is made, and the upload is begun in storage.
func (h *videoHandler) UploadStart(c echo.Context) error {
	var (
		req    = &uploadStartRequest{}
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

	// Where the installation has not said videos may be posted, none is
	// begun. The ones there are go on being shown.
	if !videos.UploadsOn() {
		return refuseUpload(c, http.StatusForbidden, "uploads_off")
	}

	contentType, _, _ := strings.Cut(req.ContentType, ";")
	contentType = strings.TrimSpace(strings.ToLower(contentType))
	if !slices.Contains(constants.UploadTypes, contentType) {
		return refuseUpload(c, http.StatusUnsupportedMediaType, constants.FailedNotAVideo)
	}
	// A GIF or an animated PNG is held to less than a video. What it says
	// it is is checked by the worker, which holds it to the same.
	maxBytes := h.Config.VideoMaxBytes
	if slices.Contains(constants.AnimationTypes, contentType) {
		maxBytes = h.Config.AnimationMaxBytes
	}
	if req.Size > maxBytes {
		return c.JSON(http.StatusRequestEntityTooLarge, map[string]any{
			"error":     constants.FailedTooLarge,
			"max_bytes": maxBytes,
		})
	}

	// A reply: what it answers has to be there, and its thread open to
	// the viewer, before any of it is sent. The worker asks again before
	// the reply is made, since a thread can close meanwhile.
	var parent *entities.AnahitaNode
	if req.ParentID > 0 {
		// On its author's own profile, as every reply is.
		if req.OwnerID != viewer.ID {
			return c.NoContent(http.StatusBadRequest)
		}
		var err error
		if parent, err = h.Srv.MediaGRPC.Read(req.ParentID, viewer); err != nil || parent == nil {
			log.Println(err)
			return c.NoContent(http.StatusNotFound)
		}
		if parent.RootID > 0 && parent.DeletedAt != nil {
			return c.NoContent(http.StatusGone)
		}
		thread, err := handlers.ThreadOf(c, h.readers(), parent, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusNotFound)
		}
		if !permissions.CanReply(thread) {
			return c.NoContent(http.StatusForbidden)
		}
	} else if strings.TrimSpace(req.Name) == "" {
		return c.NoContent(http.StatusBadRequest)
	}

	owner, err := h.Srv.ActorsGRPC.Read(req.OwnerID, viewer)
	if err != nil || owner == nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	medium := placeholderVideo(owner, viewer)

	language, valid := handlers.RequestLanguage(req.Language)
	if !valid {
		return handlers.RefuseLanguage(c)
	}
	replyAccess, valid := handlers.RequestReplyAccess(req.ReplyAccess)
	if !valid {
		return handlers.RefuseReplyAccess(c)
	}
	quotePolicy, valid := handlers.RequestQuotePolicy(req.QuotePolicy)
	if !valid {
		return handlers.RefuseQuotePolicy(c)
	}

	if parent != nil {
		// Its thread's: none of its own.
		medium.Access, replyAccess, quotePolicy = "", "", ""
		if language == "" {
			language = parent.Language
		}
	} else {
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
		if !permissions.IsMediumAudienceAllowed(mediumAccessCtx, req.Access) {
			log.Printf("refused: audience %q is not allowed on this profile", req.Access)
			return handlers.RefuseAudience(c, mediumAccessCtx)
		}
	}

	// So many a day, counted from the uploads themselves.
	sent, err := h.Srv.Uploads.CountSince(viewer.ID, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if sent >= h.Config.VideoDailyUploads {
		return refuseUpload(c, http.StatusTooManyRequests, "too_many")
	}

	upload := &repositories.Upload{
		ID:            uuid.NewString(),
		PersonID:      viewer.ID,
		OwnerID:       owner.ID,
		Name:          strings.TrimSpace(req.Name),
		Body:          req.Body,
		Access:        medium.Access,
		Language:      language,
		ReplyAccess:   replyAccess,
		QuotePolicy:   quotePolicy,
		AllowDownload: handlers.DefaultAllowDownload(req.AllowDownload, owner),
		DeclaredBytes: req.Size,
		ParentID:      req.ParentID,
	}
	upload.SourceKey = videos.SourceKey(viewer.ID, upload.ID)

	upload.StorageUploadID, err = h.Srv.Transfers.UploadStart(upload.SourceKey)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.Srv.Uploads.Add(upload); err != nil {
		log.Println(err)
		_ = h.Srv.Transfers.UploadAbort(upload.SourceKey, upload.StorageUploadID)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, newUploadResponse(upload))
}

// ownUpload reads the upload in the address, for the person who began it.
// Anybody else is told there is none.
func (h *videoHandler) ownUpload(c echo.Context) (*repositories.Upload, int) {
	viewer := getViewer(c)

	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return nil, http.StatusNotFound
	}

	upload, err := h.Srv.Uploads.Read(id)
	if err != nil {
		log.Println(err)
		return nil, http.StatusInternalServerError
	}
	if upload == nil || upload.PersonID != viewer.ID {
		return nil, http.StatusNotFound
	}

	return upload, 0
}

// UploadParts signs the addresses some of its parts are sent to. Asked for
// a few at a time, as they are needed: an address is good for an hour.
func (h *videoHandler) UploadParts(c echo.Context) error {
	upload, refused := h.ownUpload(c)
	if refused != 0 {
		return c.NoContent(refused)
	}
	if upload.Status != videos.StatusUploading {
		return refuseUpload(c, http.StatusConflict, "not_uploading")
	}

	req := &uploadPartsRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	count := int32((upload.DeclaredBytes + constants.PartSize - 1) / constants.PartSize)
	for _, number := range req.Numbers {
		if number < 1 || number > count {
			return c.NoContent(http.StatusBadRequest)
		}
	}

	urls, err := h.Srv.Transfers.UploadParts(upload.SourceKey, upload.StorageUploadID, req.Numbers)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	parts := make([]map[string]any, 0, len(urls))
	for i, url := range urls {
		parts = append(parts, map[string]any{"number": req.Numbers[i], "url": url})
	}
	return c.JSON(http.StatusOK, map[string]any{"data": parts})
}

// UploadComplete makes one file of what was sent and hands it to the
// worker. What arrived is asked of storage: a file larger than is allowed,
// whatever was said when it began, is thrown away.
func (h *videoHandler) UploadComplete(c echo.Context) error {
	upload, refused := h.ownUpload(c)
	if refused != 0 {
		return c.NoContent(refused)
	}
	if upload.Status != videos.StatusUploading {
		// Sent twice: the answer is how it stands.
		return c.JSON(http.StatusOK, newUploadResponse(upload))
	}

	size, err := h.Srv.Transfers.UploadComplete(upload.SourceKey, upload.StorageUploadID, h.Config.VideoMaxBytes)
	if err != nil {
		log.Println(err)
		switch {
		case strings.Contains(err.Error(), "larger than is allowed"):
			_ = h.Srv.Uploads.ToFailed(upload.ID, constants.FailedTooLarge)
			return refuseUpload(c, http.StatusRequestEntityTooLarge, constants.FailedTooLarge)
		case strings.Contains(err.Error(), "no part of the file"):
			return refuseUpload(c, http.StatusConflict, "nothing_sent")
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.Srv.Uploads.ToProcessing(upload.ID, size); err != nil && !errors.Is(err, repositories.ErrNotInState) {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	upload, err = h.Srv.Uploads.Read(upload.ID)
	if err != nil || upload == nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, newUploadResponse(upload))
}

// UploadRead says how an upload is getting on.
func (h *videoHandler) UploadRead(c echo.Context) error {
	upload, refused := h.ownUpload(c)
	if refused != 0 {
		return c.NoContent(refused)
	}
	return c.JSON(http.StatusOK, newUploadResponse(upload))
}

// UploadBrowse lists what the viewer has in hand: being sent, being made,
// or failed in the last day.
func (h *videoHandler) UploadBrowse(c echo.Context) error {
	viewer := getViewer(c)

	uploads, err := h.Srv.Uploads.OfPerson(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	data := make([]*uploadResponse, 0, len(uploads))
	for _, upload := range uploads {
		data = append(data, newUploadResponse(upload))
	}
	return c.JSON(http.StatusOK, map[string]any{"data": data})
}

// UploadCancel gives an upload up: one still being sent, one waiting for
// the worker, or one that failed and is being put out of sight. A worker
// that has it finds out when it tries to finish, and throws its work away.
func (h *videoHandler) UploadCancel(c echo.Context) error {
	upload, refused := h.ownUpload(c)
	if refused != 0 {
		return c.NoContent(refused)
	}

	if err := h.Srv.Uploads.ToCancelled(upload.ID); err != nil {
		if errors.Is(err, repositories.ErrNotInState) {
			return refuseUpload(c, http.StatusConflict, "finished")
		}
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if upload.StorageUploadID != "" {
		if err := h.Srv.Transfers.UploadAbort(upload.SourceKey, upload.StorageUploadID); err != nil {
			log.Println(err)
		}
	}
	h.deleteStored([]string{upload.SourceKey})

	return c.NoContent(http.StatusNoContent)
}
