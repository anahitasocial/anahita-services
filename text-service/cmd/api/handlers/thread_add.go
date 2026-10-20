package handlers

import (
	"log"
	"net/http"
	"strconv"

	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/responses"

	an_handlers "anahita-libs/cmd/api/handlers"

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
// the link in it to show a card for, as on a note.
type ThreadPart struct {
	Content        string `json:"content" validate:"required,max=8000"`
	Language       string `json:"language" validate:"omitempty,max=12"`
	LinkPreviewURL string `json:"link_preview_url" validate:"omitempty,max=2000"`
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
	// The notes of the thread, first to last: up to ten.
	Parts []ThreadPart `json:"parts" validate:"required,min=1,max=10,dive"`
}

// ReplyThreadAddRequest is a thread that answers a post or a reply: the
// parts, in order, the first answering :id.
type ReplyThreadAddRequest struct {
	// The id of the post or the reply it answers. Read by the handler from
	// the path itself.
	ParentID int64 `param:"id"`
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

	// The first part is checked as a post is.
	first, refuse := h.newNote(c, &requests.NoteAddRequest{
		OwnerID:        req.OwnerID,
		Content:        req.Parts[0].Content,
		Access:         req.Access,
		Language:       req.Parts[0].Language,
		ReplyAccess:    req.ReplyAccess,
		QuotePolicy:    req.QuotePolicy,
		QuoteID:        req.QuoteID,
		LinkPreviewURL: req.Parts[0].LinkPreviewURL,
	}, viewer)
	if refuse != nil {
		return refuse()
	}

	return h.addThread(c, 0, first, req.Parts[1:], req.LocationIDs, viewer)
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

	// The first part is checked as a reply is.
	first, _, refuse := h.newReply(c, parentID, &ReplyAddRequest{
		ParentID:       parentID,
		Content:        req.Parts[0].Content,
		Language:       req.Parts[0].Language,
		LinkPreviewURL: req.Parts[0].LinkPreviewURL,
	}, viewer)
	if refuse != nil {
		return refuse()
	}

	return h.addThread(c, parentID, first, req.Parts[1:], nil, viewer)
}

// addThread makes the first part and the rest in one go, and answers with
// them all.
func (h *noteHandler) addThread(c echo.Context, parentID int64, first *entities.AnahitaNode, rest []ThreadPart, locationIDs []int64, viewer *entities.Viewer) error {
	parts := []*entities.AnahitaNode{first}
	for _, each := range rest {
		part, refuse := h.nextPart(c, first, each, viewer)
		if refuse != nil {
			return refuse()
		}
		parts = append(parts, part)
	}

	made, err := h.Srv.MediaGRPC.AddThread(parentID, parts, viewer, locationIDs)
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
	part.Meta = h.withLinkPreview(part.Meta, each.LinkPreviewURL, part.Body, false)
	return part, nil
}
