package handlers

import (
	"log"
	"net/http"
	"strconv"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/photos"
	"anahita-libs/responses"
	"anahita-libs/threads"

	an_handlers "anahita-libs/handlers"

	"text/requests"

	"github.com/labstack/echo/v4"
)

// A thread written in one go (roadmap row 101): several notes posted
// together, in order, all or none, each after the first a reply to the one
// before. The first is a post, or a reply; the rest are replies, written by
// whoever writes the thread. The graph makes them in one transaction
// (Medium.AddThread).

// ThreadPart is one note of a thread (anahita-libs/threads).
type ThreadPart = threads.Part

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

	if !threads.EachSaysSomething(req.Parts) {
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
		LinkPreviewURL: req.Parts[0].CardURL(),
		Warning:        req.Warning,
		WarningText:    req.WarningText,
	}, viewer, req.Parts[0].HasPictures())
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

	if !threads.EachSaysSomething(req.Parts) {
		return c.NoContent(http.StatusBadRequest)
	}

	// The first part is checked as a reply is, with pictures or without.
	first, _, refuse := h.newReply(c, parentID, &ReplyAddRequest{
		ParentID:       parentID,
		Content:        req.Parts[0].Content,
		Language:       req.Parts[0].Language,
		LinkPreviewURL: req.Parts[0].CardURL(),
		Warning:        req.Warning,
		WarningText:    req.WarningText,
	}, viewer)
	if refuse != nil {
		return refuse()
	}

	return h.addThread(c, parentID, first, req.Parts, nil, nil, viewer)
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

// nextPart is a part after the first (threads.Next), with the card for
// the link in it, as on a note.
func (h *noteHandler) nextPart(c echo.Context, first *entities.AnahitaNode, each ThreadPart, viewer *entities.Viewer) (*entities.AnahitaNode, func() error) {
	part, ok := threads.Next(first, each, viewer.ID)
	if !ok {
		return nil, func() error { return an_handlers.RefuseLanguage(c) }
	}
	part.Meta = h.withLinkPreview(part.Meta, each.CardURL(), part.Body, false)
	return part, nil
}

// partPictures makes each part with pictures a photo post, by
// threads.Pictures, and answers a refusal as a photo post's is.
func (h *noteHandler) partPictures(c echo.Context, parts []*entities.AnahitaNode, all []ThreadPart, allowDownload *bool, viewer *entities.Viewer) (map[int]services.ThreadPartFiles, func() error) {
	files, refused, err := threads.Pictures(threads.Readers{Media: h.Srv.MediaGRPC, Actors: h.Srv.ActorsGRPC}, parts, all, allowDownload, viewer)
	if err != nil {
		log.Println(err)
		return nil, func() error { return c.NoContent(http.StatusInternalServerError) }
	}
	if refused != "" {
		return nil, func() error {
			status, body := photos.Refusal(refused)
			return c.JSON(status, body)
		}
	}
	return files, nil
}
