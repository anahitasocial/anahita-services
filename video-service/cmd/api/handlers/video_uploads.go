package handlers

import (
	"anahita-libs/responses"
	"errors"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/handlers"
	"anahita-libs/permissions"
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

// UploadStartRequest is a video about to be sent, and the post it will be.
type UploadStartRequest struct {
	// The profile the post will be on.
	OwnerID int64 `param:"owner_id" validate:"required,numeric"`
	// Its title, if it has one: neither a post nor a reply needs one.
	Title string `json:"title" form:"title" validate:"omitempty,max=100"`
	// Its words, as Markdown.
	Content string `json:"content" form:"content" validate:"omitempty,max=8000"`
	// Who may see it (permissions.md).
	Access string `json:"access" form:"access" validate:"omitempty,oneof=public registered followers leaders mutuals admins myself"`
	// The language it is written in, as a BCP 47 tag.
	Language string `json:"language" form:"language" validate:"omitempty,max=12"`
	// Who may reply: `anyone`, `nobody`, or any of `followers`, `following`
	// and `mentioned` joined by commas.
	ReplyAccess string `json:"reply_access" form:"reply_access" validate:"omitempty,max=64"`
	// Who may quote it: `anyone`, `followers` or `nobody`.
	QuotePolicy string `json:"quote_policy" form:"quote_policy" validate:"omitempty,max=16"`
	// Whether whoever can see it may download it. Left out, the post
	// starts with what the profile it is on says.
	AllowDownload *bool `json:"allow_download" form:"allow_download"`
	// In the author's media library only: put up from inside the article
	// editor to be shown in an article, and listed and announced nowhere.
	// Only on the author's own profile, and never a reply.
	LibraryOnly bool `json:"library_only" form:"library_only"`
	// What it answers, when it is a reply: then it is on its author's own
	// profile, and what it is open to, who may answer it and quoting are
	// its thread's, so those above are not read.
	ParentID int64 `json:"parent_id" form:"parent_id" validate:"omitempty,min=1"`
	// How large the file is, in bytes.
	//
	// What the browser says it is sending. The worker decides what it is.
	Size int64 `json:"size" form:"size" validate:"required,min=1"`
	// What kind of file it is: `video/mp4`, `video/quicktime`, `video/webm`,
	// `image/gif` or `image/png`.
	ContentType string `json:"content_type" form:"content_type" validate:"required,max=100"`
}

// UploadPartsRequest asks where to send some parts of an upload.
type UploadPartsRequest struct {
	// The upload's id. Read by the handler from the path itself.
	ID string `param:"id"`
	// Which parts to send, from 1: a few at a time, as they are needed.
	Numbers []int32 `json:"numbers" validate:"required,min=1,max=100"`
}

// UploadResponse is an upload as its uploader is told of it.
type UploadResponse struct {
	// Its id.
	ID string `json:"id"`
	// `uploading`, `processing`, `ready`, `failed` or `cancelled`.
	Status string `json:"status"`
	// The post's title.
	Title string `json:"title"`
	// The profile the post will be on.
	OwnerID int64 `json:"owner_id"`
	// Why it failed.
	Error string `json:"error,omitempty"`
	// The post, once it is one.
	NodeID int64 `json:"node_id,omitempty"`
	// What it answers, when it is a reply.
	ParentID int64 `json:"parent_id,omitempty"`
	// How large each part is, in bytes, while it is being sent.
	//
	// How it is to be sent, while it is being.
	PartSize int64 `json:"part_size,omitempty"`
	// How many parts to send it in.
	PartCount int64 `json:"part_count,omitempty"`
	// How large it said it is, in bytes.
	Size int64 `json:"size,omitempty"`
	// When it was begun.
	CreatedAt time.Time `json:"created_at"`
}

func newUploadResponse(upload *repositories.Upload) *UploadResponse {
	resp := &UploadResponse{
		ID:        upload.ID,
		Status:    upload.Status,
		Title:     upload.Name,
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
	return c.JSON(status, responses.Refusal{Error: code})
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
		req    = &UploadStartRequest{}
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
		return c.JSON(http.StatusRequestEntityTooLarge, UploadTooLarge{
			Error:    constants.FailedTooLarge,
			MaxBytes: maxBytes,
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
	// A library is one's own: on one's own profile, and a post, not a
	// reply.
	if req.LibraryOnly && (owner.ID != viewer.ID || req.ParentID > 0) {
		return c.NoContent(http.StatusBadRequest)
	}

	if sent >= h.Config.VideoDailyUploads {
		return refuseUpload(c, http.StatusTooManyRequests, "too_many")
	}

	upload := &repositories.Upload{
		ID:            uuid.NewString(),
		PersonID:      viewer.ID,
		OwnerID:       owner.ID,
		Name:          strings.TrimSpace(req.Title),
		Body:          req.Content,
		Access:        medium.Access,
		Language:      language,
		ReplyAccess:   replyAccess,
		QuotePolicy:   quotePolicy,
		AllowDownload: handlers.DefaultAllowDownload(req.AllowDownload, owner),
		LibraryOnly:   req.LibraryOnly,
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

	req := &UploadPartsRequest{}
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

	parts := make([]UploadPart, 0, len(urls))
	for i, url := range urls {
		parts = append(parts, UploadPart{Number: req.Numbers[i], URL: url})
	}
	return c.JSON(http.StatusOK, responses.Unpaged[UploadPart]{Data: parts})
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

	data := make([]*UploadResponse, 0, len(uploads))
	for _, upload := range uploads {
		data = append(data, newUploadResponse(upload))
	}
	return c.JSON(http.StatusOK, responses.Unpaged[*UploadResponse]{Data: data})
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
