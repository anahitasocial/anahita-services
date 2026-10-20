package handlers

import (
	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/linkpreviews"
	"anahita-libs/responses"
	"log"
	"net/http"
	"strings"
	"text/constants"
	"text/requests"

	"github.com/labstack/echo/v4"
)

type noteHandler struct {
	*Env
	*mediumHandler
}

// NewNoteHandler creates a new note handler
func NewNoteHandler(env *Env) *noteHandler {
	return &noteHandler{
		env,
		NewMediumHandler(env),
	}
}

// Browse returns a list of note
func (h *noteHandler) Browse(c echo.Context) error {
	var (
		err    error
		req    = requests.NewNoteRequest().Browse
		gReq   = services.NewMediaGRPCRequest().Browse
		viewer = getViewer(c)
	)

	if ok, err := h.Perms.Medium.CanBrowse(viewer); !ok {
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

	resp := &responses.List[*responses.MediumNodeItem]{}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	gReq.Types = []string{constants.NodeMediumNoteType}
	gReq.Offset = req.Offset
	gReq.Limit = req.Limit
	gReq.Direction = req.Direction
	gReq.Ordering = req.Ordering
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

	media, err := h.Srv.MediaGRPC.Browse(gReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, medium := range media {
		if err = h.onAfterFetched(c, medium); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	batchPresignPaths(h.Env, media)
	// What the viewer may do with each, as a post's own page says.
	an_handlers.SetListPostFlags(c, h.Srv.ActorsGRPC, h.Srv.SocialgraphGRPC, media, viewer)

	mediaResp := &responses.MediumNodeResponse{}
	resp.Data = mediaResp.MapMediumNodeItems(media)

	return c.JSON(http.StatusOK, resp)
}

// Edit updates a note
func (h *noteHandler) Edit(c echo.Context) error {
	var (
		err    error
		req    = requests.NewNoteRequest().Edit
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

	// A reply is changed by its author only, and only while they can still
	// see the thread. Not by whoever looks after the thread: see
	// permissions.CanEditReply.
	if medium.RootID > 0 {
		if medium.DeletedAt != nil {
			return c.NoContent(http.StatusGone)
		}
		if _, refused := h.replyRuling(c, medium, viewer, permissions.CanEditReply); refused != 0 {
			return c.NoContent(refused)
		}
	} else {
		// A website's own note follows its site: nobody changes its words.
		if permissions.IsWebsiteItem(medium) {
			return c.JSON(http.StatusForbidden, responses.Refusal{Error: permissions.WebsiteItemRefusal})
		}
		mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		if ok, err := h.Perms.Medium.CanEdit(mediumAccessCtx); !ok {
			log.Println(err)
			return c.NoContent(http.StatusForbidden)
		}
	}

	medium.Body = req.Content

	// The language the author says it is written in. Not saying is not an
	// error; a tag that is not one is.
	if language, valid := an_handlers.RequestLanguage(req.Language); !valid {
		return an_handlers.RefuseLanguage(c)
	} else if language != "" {
		medium.Language = language
	}
	medium.ModifiedBy = viewer.ID
	// Its preview follows the edit: the one kept now, or none.
	medium.Meta = h.withLinkPreview(medium.Meta, req.LinkPreviewURL, medium.Body, req.LinkPreviewRemove)

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

// newNote is a note as `req` describes it, checked: who may post where,
// with what audience, quoting what. Or, when it may not be made, what to
// answer instead.
func (h *noteHandler) newNote(c echo.Context, req *requests.NoteAddRequest, viewer *entities.Viewer) (*entities.AnahitaNode, func() error) {
	owner, err := h.Srv.ActorsGRPC.Read(req.OwnerID, viewer)
	if err != nil {
		log.Println(err)
		return nil, func() error { return c.NoContent(http.StatusNotFound) }
	}

	medium := entities.NewAnahitaNode()
	medium.Type = constants.NodeMediumNoteType
	medium.Service = constants.TextService
	medium.Access = req.Access
	medium.Body = req.Content

	// The language the author says it is written in. Not saying is not an
	// error; a tag that is not one is.
	if language, valid := an_handlers.RequestLanguage(req.Language); !valid {
		return nil, func() error { return an_handlers.RefuseLanguage(c) }
	} else if language != "" {
		medium.Language = language
	}
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

	// Who may reply, when the author says. Not saying leaves it to the
	// profile; a value that is not one is an error.
	if replyAccess, valid := an_handlers.RequestReplyAccess(req.ReplyAccess); !valid {
		return nil, func() error { return an_handlers.RefuseReplyAccess(c) }
	} else if replyAccess != "" {
		an_handlers.SetReplyAccess(medium, replyAccess)
	}

	// Who may quote it, when the author says. Not saying leaves it to
	// their own setting.
	if quotePolicy, valid := an_handlers.RequestQuotePolicy(req.QuotePolicy); !valid {
		return nil, func() error { return an_handlers.RefuseQuotePolicy(c) }
	} else if quotePolicy != "" {
		medium.QuotePolicy = quotePolicy
	}
	medium.Access = an_handlers.DetermineMediumNodeAccess(req.Access, medium.Owner.Access)
	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return nil, func() error { return c.NoContent(http.StatusInternalServerError) }
	}
	if ok, err := h.Perms.Medium.CanAdd(mediumAccessCtx); !ok {
		log.Println(err)
		return nil, func() error { return c.NoContent(http.StatusForbidden) }
	}

	// The audience has to be one this post can have where it is going: no
	// "admins" on a person's profile, no "only me" on somebody else's.
	if !permissions.IsMediumAudienceAllowed(mediumAccessCtx, req.Access) {
		log.Printf("refused: audience %q is not allowed on this profile", req.Access)
		return nil, func() error { return an_handlers.RefuseAudience(c, mediumAccessCtx) }
	}

	// A note that quotes a post. Whether it may is the quoted post's to
	// say, and is asked last, once the note itself is known to be allowed.
	if req.QuoteID > 0 {
		if allowed, reason, status := h.mayQuote(req.QuoteID, viewer); !allowed {
			if status != 0 {
				return nil, func() error { return c.NoContent(status) }
			}
			log.Printf("refused: %d may not quote %d: %s", viewer.ID, req.QuoteID, reason)
			return nil, func() error { return an_handlers.RefuseQuote(c, reason) }
		}
		medium.QuoteID = req.QuoteID
	}

	// The preview of a link in it, when its writer kept one.
	medium.Meta = h.withLinkPreview(medium.Meta, req.LinkPreviewURL, medium.Body, false)

	return medium, nil
}

// Add creates a new note
func (h *noteHandler) Add(c echo.Context) error {
	var (
		err    error
		req    = requests.NewNoteRequest().Add
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

	medium, refuse := h.newNote(c, req, viewer)
	if refuse != nil {
		return refuse()
	}

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

	if err = h.onAfterFetched(c, medium); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{medium})

	mediaResp := &responses.MediumNodeResponse{}
	resp := mediaResp.MapMediumNode(medium)

	return c.JSON(http.StatusCreated, resp)
}

// withLinkPreview puts the preview of a link into a note's meta, keeps the
// one that is there, or takes it out.
//
// The browser says which link, and nothing else. The link has to be in the
// note's own text, and the preview is the one link-preview-service read
// and kept: what a card says of a page is what this installation found
// there, so a card cannot be made to say something the page does not.
//
// A note that already has a card keeps it for as long as its link is still
// in the note, whatever is sent: a card is as the page was when the note
// was written, and correcting a word is no reason to lose it or to fetch
// it again. With the link gone, so is the card; and `remove` is its writer
// taking it off while the link stays.
func (h *mediumHandler) withLinkPreview(meta []byte, address, body string, remove bool) []byte {
	existing := linkpreviews.FromMeta(meta)

	if remove {
		if existing == nil {
			return meta
		}
		return linkpreviews.IntoMeta(meta, nil)
	}

	if normal, ok := linkpreviews.Normalise(address); ok && h.Previews != nil && strings.Contains(body, strings.TrimSpace(address)) {
		// The card it already has for this link is the one it keeps.
		if existing != nil && existing.URL == normal {
			return meta
		}
		if preview, kept := h.Previews.Get(normal); kept && preview.Usable() {
			return linkpreviews.IntoMeta(meta, linkpreviews.NewSnapshot(preview))
		}
	}

	// Nothing to put in and nothing there to take out: left as it was,
	// which for most notes is no meta at all.
	if existing == nil {
		return meta
	}
	// Its address is kept tidied, with the host in lower case, where the
	// note has it as it was typed.
	if strings.Contains(strings.ToLower(body), strings.ToLower(existing.URL)) {
		return meta
	}

	return linkpreviews.IntoMeta(meta, nil)
}
