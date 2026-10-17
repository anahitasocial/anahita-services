package handlers

import (
	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/responses"
	"log"
	"net/http"
	"strconv"
	"text/constants"

	"github.com/labstack/echo/v4"
)

// Replies.
//
// A reply is a note that answers a post or another reply. It is owned by its
// author and has no audience of its own, so nothing about it is decided the
// way it is for an ordinary note. Everything is decided by the post at the
// top of its thread: see permissions/reply.go.
//
// The post at the top can be any kind of post, a photo included. A reply
// made here is a note; a reply can also be a photo post or a video post,
// made by those services (POST /photos/:id/replies, and a video upload with
// a parent). Whatever its kind, its words are changed here, and its thread
// is served here.

// readers are what a thread is read with.
func (h *mediumHandler) readers() an_handlers.ThreadReaders {
	return an_handlers.ThreadReaders{Media: h.Srv.MediaGRPC, Actors: h.Srv.ActorsGRPC, Socialgraph: h.Srv.SocialgraphGRPC}
}

// thread reads the post at the top of a thread, once per request: see
// an_handlers.Thread.
func (h *mediumHandler) thread(c echo.Context, rootID int64, viewer *entities.Viewer) (*permissions.ReplyAccessContext, error) {
	return an_handlers.Thread(c, h.readers(), rootID, viewer)
}

func forReply(thread *permissions.ReplyAccessContext, reply *entities.AnahitaNode) *permissions.ReplyAccessContext {
	return an_handlers.ForReply(thread, reply)
}

func setReplyFlags(thread *permissions.ReplyAccessContext, reply *entities.AnahitaNode) {
	an_handlers.SetReplyFlags(thread, reply)
}

func blankRemoved(reply *entities.AnahitaNode) {
	an_handlers.BlankRemoved(reply)
}

// Thread returns everything said under a post: every reply at every depth,
// oldest first, for the viewer to arrange by what answers what.
//
// GET /notes/:id/thread, where :id is the post at the top.
func (h *mediumHandler) Thread(c echo.Context) error {
	viewer := getViewer(c)

	rootID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || rootID <= 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	thread, err := h.thread(c, rootID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// A thread hangs from a post. Asked for under a reply, the answer is
	// the thread that reply is in.
	if thread.Root.RootID > 0 {
		return c.JSON(http.StatusConflict, NotARoot{Error: "not_a_root", RootID: thread.Root.RootID})
	}

	if !permissions.CanSeeThread(thread) {
		return c.NoContent(http.StatusForbidden)
	}

	replies, err := h.Srv.MediaGRPC.Thread(rootID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Whoever looks after the thread is sent the hidden replies too, marked,
	// so they can show one again. Nobody else is sent them at all.
	looksAfter := permissions.LooksAfterThread(thread)
	total := len(replies)
	replies = services.VisibleInThread(replies, looksAfter)

	hidden := 0
	for _, reply := range replies {
		setReplyFlags(thread, reply)
		if reply.Hidden {
			hidden++
		}
	}

	batchPresignPaths(h.Env, replies)

	for _, reply := range replies {
		blankRemoved(reply)
	}

	canReply := permissions.CanReply(thread)

	return c.JSON(http.StatusOK, Thread{
		Data:        threadResponse(replies),
		HiddenCount: hidden,
		CanReply:    canReply,
		ReplyAccess: permissions.RepliesLimitedTo(thread.Root),
		Truncated:   total >= replyThreadLimit,
	})
}

// As many as the graph reads a thread with.
const replyThreadLimit = 500

// ReplyAddRequest is a new reply: what it says, and the language it says it
// in. Where it goes is in the address; who may see it is the thread's to
// say, so there is no audience to send.
type ReplyAddRequest struct {
	// The id of the post or the reply it answers. Read by the handler from
	// the path itself.
	ParentID int64 `param:"id"`
	// Its words, as Markdown.
	Content string `json:"content" form:"content" validate:"required,max=8000"`
	// The language it is written in, as a BCP 47 tag.
	Language string `json:"language" form:"language" validate:"omitempty,max=12"`
	// The link in it to show a card for, as on a note. See NoteAddRequest.
	LinkPreviewURL string `json:"link_preview_url" form:"link_preview_url" validate:"omitempty,max=2000"`
}

// AddReply answers a post or a reply.
//
// POST /notes/:id/replies, where :id is what is being answered.
func (h *mediumHandler) AddReply(c echo.Context) error {
	var (
		req    = &ReplyAddRequest{}
		viewer = getViewer(c)
	)

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

	parent, err := h.Srv.MediaGRPC.Read(parentID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// Nothing is said under a reply that has been removed.
	if parent.RootID > 0 && parent.DeletedAt != nil {
		return c.NoContent(http.StatusGone)
	}

	rootID := parent.RootID
	if rootID <= 0 {
		rootID = parent.ID
	}

	thread, err := h.thread(c, rootID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if !permissions.CanReply(thread) {
		return c.NoContent(http.StatusForbidden)
	}

	reply := entities.NewAnahitaNode()
	reply.Type = constants.NodeMediumNoteType
	reply.Service = constants.TextService
	reply.Body = req.Content

	if language, valid := an_handlers.RequestLanguage(req.Language); !valid {
		return an_handlers.RefuseLanguage(c)
	} else if language != "" {
		reply.Language = language
	}

	reply.Enabled = true
	reply.CommentStatus = true

	// Only what it answers is sent. The graph works out the root from
	// that, and gives the reply no audience of its own.
	reply.ParentID = parent.ID

	// A reply is a note, and carries a card for a link like one.
	reply.Meta = h.withLinkPreview(reply.Meta, req.LinkPreviewURL, reply.Body, false)

	// A reply belongs to whoever wrote it, wherever the thread is. That is
	// what lets them edit it and take it down, and what a reply is on the
	// networks this federates with. Under a Pulse's own post, one by its
	// administrator is the Pulse's.
	an_handlers.OwnReply(thread, reply, viewer)

	reply, err = h.Srv.MediaGRPC.Add(reply, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	setReplyFlags(thread, reply)
	batchPresignPaths(h.Env, []*entities.AnahitaNode{reply})

	mediaResp := &responses.MediumNodeResponse{}
	return c.JSON(http.StatusCreated, mediaResp.MapMediumNode(reply))
}

// ReplyHideRequest hides a reply from its thread, or shows it again.
type ReplyHideRequest struct {
	// The reply's id. Read by the handler from the path itself.
	ID int64 `param:"id"`
	// Hide it, or show it again.
	Hidden bool `json:"hidden" form:"hidden"`
}

// HideReply hides a reply from its thread for everybody, with everything
// said under it, or shows it again.
//
// The reply is not destroyed and its author is not told. For whoever looks
// after the thread; see permissions.CanHideReply.
//
// PATCH /notes/:id/hidden
func (h *mediumHandler) HideReply(c echo.Context) error {
	var (
		req    = &ReplyHideRequest{}
		viewer = getViewer(c)
	)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	reply, err := h.Srv.MediaGRPC.Read(id, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// Only a reply is in somebody's thread to be hidden from it.
	if reply.RootID <= 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	thread, err := h.thread(c, reply.RootID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if !permissions.CanHideReply(forReply(thread, reply)) {
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.Srv.MediaGRPC.HideReply(reply.ID, req.Hidden, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, HiddenAnswer{ID: reply.ID, Hidden: req.Hidden})
}

// replyRuling answers a request about one reply by the rules for its
// thread: see an_handlers.ReplyRuling.
func (h *mediumHandler) replyRuling(c echo.Context, reply *entities.AnahitaNode, viewer *entities.Viewer, may func(*permissions.ReplyAccessContext) bool) (*permissions.ReplyAccessContext, int) {
	return an_handlers.ReplyRuling(c, h.readers(), reply, viewer, may)
}

// threadResponse maps the replies of a thread in full.
//
// Not as list items. A list item leaves out `authorized`, what the viewer
// may do with the post, because a list of posts is drawn without controls.
// A thread is drawn with them: reply, edit, delete, hide and like are each
// shown only when the answer here is yes. Sent as list items, every reply
// arrived with no answers at all, and nobody could edit or delete their own.
func threadResponse(replies []*entities.AnahitaNode) []*responses.MediumNode {
	mediaResp := &responses.MediumNodeResponse{}

	out := make([]*responses.MediumNode, 0, len(replies))
	for _, reply := range replies {
		out = append(out, mediaResp.MapMediumNode(reply))
	}
	return out
}
