package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/photos"
	"anahita-libs/responses"

	an_handlers "anahita-libs/handlers"

	"text/constants"
	"text/requests"

	"github.com/labstack/echo/v4"
)

// A thread written in one go (roadmap row 101): several notes posted
// together, in order, all or none, each after the first a reply to the one
// before. The first is a post, or a reply; the rest are replies, written by
// whoever writes the thread. The graph makes them in one transaction
// (Medium.AddThread).

// ThreadPart is one note of a thread: what it says, in what language, and
// the link in it to show a card for, as on a note; or, with pictures, a
// photo post in the thread (roadmap row 143), which needs no words.
type ThreadPart struct {
	// Its words, as Markdown. A part with pictures need not have any. Up to 500
	// characters, a link counted as 23.
	Content string `json:"content" validate:"omitempty,max=8000,post_length"`
	// The language it is written in, as a BCP 47 tag.
	Language string `json:"language" validate:"omitempty,max=12"`
	// The link in it to show a card for. Not on a part with pictures.
	LinkPreviewURL string `json:"link_preview_url" validate:"omitempty,max=2000"`
	// Its pictures, as a photo post's: each uploaded first through
	// POST /photos/:owner_id/uploads, for the profile the part is on, which
	// is the thread's for the first part and the writer's for the rest.
	Uploads []photos.FileRef `json:"uploads" validate:"omitempty,max=20"`
}

// hasPictures is whether a part is a photo post.
func (p ThreadPart) hasPictures() bool {
	return len(p.Uploads) > 0
}

// linkPreviewURL is the link to show a card for: none on a part with
// pictures, which show in its place.
func (p ThreadPart) linkPreviewURL() string {
	if p.hasPictures() {
		return ""
	}
	return p.LinkPreviewURL
}

// NoteThreadAddRequest is a thread that begins a post: what a post takes,
// for its first part, and the parts, in order.
type NoteThreadAddRequest struct {
	// The profile it is posted on.
	OwnerID int64 `param:"owner_id" validate:"required,numeric"`
	// As on NoteAddRequest, for the first part: the rest are replies, seen
	// by whoever may see it.
	Access      string  `json:"access" validate:"omitempty,oneof=public registered followers leaders mutuals admins myself"`
	ReplyAccess string  `json:"reply_access" validate:"omitempty,max=64"`
	QuotePolicy string  `json:"quote_policy" validate:"omitempty,max=16"`
	QuoteID     int64   `json:"quote_id" validate:"omitempty,numeric"`
	LocationIDs []int64 `json:"location_ids" validate:"omitempty,max=5,dive,gt=0"`
	// Whether the first part's pictures may be downloaded, when it has
	// any, as on a photo post. Not saying leaves it to the profile.
	AllowDownload *bool `json:"allow_download"`
	// Its content warning, as on a note, given to every part.
	Warning     string `json:"warning" validate:"omitempty,max=16"`
	WarningText string `json:"warning_text" validate:"omitempty,max=400"`
	// The notes of the thread, first to last: up to ten.
	Parts []ThreadPart `json:"parts" validate:"required,min=1,max=10,dive"`
}

// ReplyThreadAddRequest is a thread that answers a post or a reply: the
// parts, in order, the first answering :id.
type ReplyThreadAddRequest struct {
	// The id of the post or the reply it answers. Read by the handler from
	// the path itself.
	ParentID int64 `param:"id"`
	// Its content warning, as on a reply, given to every part.
	Warning     string `json:"warning" validate:"omitempty,max=16"`
	WarningText string `json:"warning_text" validate:"omitempty,max=400"`
	// The notes of the thread, first to last: up to ten.
	Parts []ThreadPart `json:"parts" validate:"required,min=1,max=10,dive"`
}

// ThreadAdded is a thread as it was made: its notes, first to last.
type ThreadAdded struct {
	Parts []*responses.MediumNode `json:"parts"`
}

// AddThread posts a thread that begins a post.
//
// POST /notes/:owner_id/thread
func (h *noteHandler) AddThread(c echo.Context) error {
	req := &NoteThreadAddRequest{}
	viewer := getViewer(c)
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if !eachSaysSomething(req.Parts) {
		return c.NoContent(http.StatusBadRequest)
	}

	// The first part is checked as a post is: as a photo post, when it has
	// pictures, which a profile may take where it takes no notes, or not.
	first, refuse := h.newPost(c, &requests.NoteAddRequest{
		OwnerID:        req.OwnerID,
		Content:        req.Parts[0].Content,
		Access:         req.Access,
		Language:       req.Parts[0].Language,
		ReplyAccess:    req.ReplyAccess,
		QuotePolicy:    req.QuotePolicy,
		QuoteID:        req.QuoteID,
		LinkPreviewURL: req.Parts[0].linkPreviewURL(),
		Warning:        req.Warning,
		WarningText:    req.WarningText,
	}, viewer, req.Parts[0].hasPictures())
	if refuse != nil {
		return refuse()
	}

	return h.addThread(c, 0, first, req.Parts, req.LocationIDs, req.AllowDownload, viewer)
}

// AddReplyThread posts a thread that answers a post or a reply.
//
// POST /notes/:id/replies/thread, where :id is what its first part answers.
func (h *noteHandler) AddReplyThread(c echo.Context) error {
	req := &ReplyThreadAddRequest{}
	viewer := getViewer(c)
	parentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || parentID <= 0 {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if !eachSaysSomething(req.Parts) {
		return c.NoContent(http.StatusBadRequest)
	}

	// The first part is checked as a reply is, with pictures or without.
	first, _, refuse := h.newReply(c, parentID, &ReplyAddRequest{
		ParentID:       parentID,
		Content:        req.Parts[0].Content,
		Language:       req.Parts[0].Language,
		LinkPreviewURL: req.Parts[0].linkPreviewURL(),
		Warning:        req.Warning,
		WarningText:    req.WarningText,
	}, viewer)
	if refuse != nil {
		return refuse()
	}

	return h.addThread(c, parentID, first, req.Parts, nil, nil, viewer)
}

// eachSaysSomething is whether every part has words or pictures.
func eachSaysSomething(parts []ThreadPart) bool {
	for _, each := range parts {
		if strings.TrimSpace(each.Content) == "" && !each.hasPictures() {
			return false
		}
	}
	return true
}

// addThread makes the first part and the rest in one go, and answers with
// them all. `all` is every part asked for, the first included, which is
// `first` checked; `allowDownload` is what its writer said of the first
// part's pictures.
func (h *noteHandler) addThread(c echo.Context, parentID int64, first *entities.AnahitaNode, all []ThreadPart, locationIDs []int64, allowDownload *bool, viewer *entities.Viewer) error {
	parts := []*entities.AnahitaNode{first}
	for _, each := range all[1:] {
		part, refuse := h.nextPart(c, first, each, viewer)
		if refuse != nil {
			return refuse()
		}
		parts = append(parts, part)
	}

	files, refuse := h.partPictures(c, parts, all, allowDownload, viewer)
	if refuse != nil {
		return refuse()
	}

	made, err := h.Srv.MediaGRPC.AddThread(parentID, parts, viewer, locationIDs, files)
	// A group's or an event's rules changed since they agreed: agreed to
	// again first (roadmap row 102, part 5c).
	if an_handlers.IsRulesNotAgreed(err) {
		return an_handlers.RefuseRulesNotAgreed(c)
	}
	if err != nil || len(made) != len(parts) {
		log.Printf("a thread of %d parts by %d was not made: %v", len(parts), viewer.ID, err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, part := range made {
		if err := h.onAfterFetched(c, part); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}
	batchPresignPaths(h.Env, made)

	mapper := &responses.MediumNodeResponse{}
	answer := ThreadAdded{}
	for _, part := range made {
		answer.Parts = append(answer.Parts, mapper.MapMediumNode(part))
	}
	return c.JSON(http.StatusCreated, answer)
}

// nextPart is a part after the first: a reply to the one before, written
// by whoever writes the thread. It belongs to them, as any reply does; or,
// in a website's thread written as the website, to the website, as the
// first part does.
func (h *noteHandler) nextPart(c echo.Context, first *entities.AnahitaNode, each ThreadPart, viewer *entities.Viewer) (*entities.AnahitaNode, func() error) {
	part := entities.NewAnahitaNode()
	part.Type = constants.NodeMediumNoteType
	part.Service = constants.TextService
	part.Body = each.Content
	if language, valid := an_handlers.RequestLanguage(each.Language); !valid {
		return nil, func() error { return an_handlers.RefuseLanguage(c) }
	} else if language != "" {
		part.Language = language
	}
	part.Enabled = true
	part.CommentStatus = true
	// The thread's warning is every part's: what it holds goes on.
	part.Warning = first.Warning
	part.WarningText = first.WarningText
	part.ModifiedBy = viewer.ID
	if first.OwnerType == an_constants.NodeActorWebsiteEntityType {
		part.OwnerID = first.OwnerID
		part.OwnerType = first.OwnerType
		part.CreatedBy = first.OwnerID
		part.Meta = an_handlers.WithWrittenBy(part.Meta, viewer.ID)
	} else {
		part.OwnerID = viewer.ID
		part.OwnerType = an_constants.NodeActorPersonEntityType
		part.CreatedBy = viewer.ID
	}
	part.Meta = h.withLinkPreview(part.Meta, each.linkPreviewURL(), part.Body, false)
	return part, nil
}

// partPictures makes each part with pictures a photo post, its pictures
// checked by the rules a photo post's are (anahita-libs/photos): uploaded by
// the writer, for the profile the part is on, none twice in the thread.
// What it answers is each one's images, by its place, for the graph to set
// with the thread, in one go.
func (h *noteHandler) partPictures(c echo.Context, parts []*entities.AnahitaNode, all []ThreadPart, allowDownload *bool, viewer *entities.Viewer) (map[int]services.ThreadPartFiles, func() error) {
	var refs []photos.FileRef
	for _, each := range all {
		refs = append(refs, each.Uploads...)
	}
	if len(refs) == 0 {
		return nil, nil
	}
	refuse := func(code string) func() error {
		return func() error {
			status, body := photos.Refusal(code)
			return c.JSON(status, body)
		}
	}

	// An upload is stored once, and can be one picture of one post.
	seen := map[int64]bool{}
	for _, id := range photos.UploadIDs(refs) {
		if seen[id] {
			return nil, refuse(photos.ErrFileRepeated)
		}
		seen[id] = true
	}

	uploads, err := h.Srv.MediaGRPC.ReadPhotoUploads(photos.UploadIDs(refs))
	if err != nil {
		log.Println(err)
		return nil, func() error { return c.NoContent(http.StatusInternalServerError) }
	}

	// Whether the pictures may be downloaded: as the writer says for the
	// first part, and as the profile each is on says otherwise, read once
	// for the parts after it.
	var author *entities.AnahitaNode
	files := map[int]services.ThreadPartFiles{}
	for position, each := range all {
		if !each.hasPictures() {
			continue
		}
		part := parts[position]
		plan, refused := photos.PlanFiles(nil, uploads, each.Uploads, viewer.ID, part.OwnerID)
		if refused != "" {
			return nil, refuse(refused)
		}

		owner := part.Owner
		if owner == nil || owner.ID != part.OwnerID {
			if author == nil || author.ID != part.OwnerID {
				if author, err = h.Srv.ActorsGRPC.Read(part.OwnerID, viewer); err != nil {
					log.Println(err)
					return nil, func() error { return c.NoContent(http.StatusNotFound) }
				}
			}
			owner = author
		}

		part.Type = an_constants.NodeMediumPhotoEntityType
		part.Service = an_constants.PhotoServiceName
		said := (*bool)(nil)
		if position == 0 {
			said = allowDownload
		}
		part.AllowDownload = an_handlers.DefaultAllowDownload(said, owner)
		// The post names its first image itself, for everything that
		// shows one.
		part.Filename = plan.Files[0].Filename
		part.Mimetype = plan.Files[0].Mimetype
		files[position] = services.ThreadPartFiles{Files: plan.Files, UploadIDs: plan.UploadIDs}
	}
	return files, nil
}
