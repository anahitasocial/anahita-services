package handlers

import (
	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	libconstants "anahita-libs/constants"
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
// The post at the top can be any kind of post, a photo included. Replies are
// served under /notes/ whatever they answer, because a reply is a note.

// requestThreadsKey holds the threads read so far in this request, by root.
const requestThreadsKey = "anahita.replies.threads"

// thread reads the post at the top of a thread as the viewer, with the
// profile it is on and that profile's relationship to the viewer: what every
// rule about a reply is judged against.
//
// Once per root per request. A thread is many replies with one root.
func (h *mediumHandler) thread(c echo.Context, rootID int64, viewer *entities.Viewer) (*permissions.ReplyAccessContext, error) {
	threads, _ := c.Get(requestThreadsKey).(map[int64]*permissions.ReplyAccessContext)
	if threads == nil {
		threads = map[int64]*permissions.ReplyAccessContext{}
		c.Set(requestThreadsKey, threads)
	}

	if ctx, ok := threads[rootID]; ok {
		return ctx, nil
	}

	root, err := h.Srv.MediaGRPC.Read(rootID, viewer)
	if err != nil {
		return nil, err
	}

	// The owner as the actor read returns it: its access, and its
	// permissions with the defaults merged in. A post carries only a
	// summary of its owner, which has neither.
	owner, err := h.Srv.ActorsGRPC.Read(root.OwnerID, viewer)
	if err != nil {
		return nil, err
	}
	root.Owner = owner

	rels, err := h.Srv.SocialgraphGRPC.Browse([]int64{root.OwnerID}, viewer.ID)
	if err != nil {
		return nil, err
	}

	ctx := &permissions.ReplyAccessContext{
		Viewer:             viewer,
		Root:               root,
		RootOwnerViewerRel: rels[root.OwnerID],
	}
	threads[rootID] = ctx

	return ctx, nil
}

// forReply is the thread's context with one reply in it.
func forReply(thread *permissions.ReplyAccessContext, reply *entities.AnahitaNode) *permissions.ReplyAccessContext {
	withReply := *thread
	withReply.Reply = reply
	return &withReply
}

// setReplyFlags answers what the viewer may do with a reply, onto its
// Authorized, from the rules for its thread.
func setReplyFlags(thread *permissions.ReplyAccessContext, reply *entities.AnahitaNode) {
	if reply.Authorized == nil {
		reply.Authorized = &entities.AnahitaNodeAuthorized{}
	}

	ctx := forReply(thread, reply)

	// A removed reply is a placeholder. Nothing can be done to it but
	// answer what is under it.
	removed := reply.DeletedAt != nil

	reply.Authorized.Edit = !removed && permissions.CanEditReply(ctx)
	reply.Authorized.Delete = !removed && permissions.CanDeleteReply(ctx)

	hide := !removed && permissions.CanHideReply(ctx)
	canReply := !removed && permissions.CanReply(ctx)
	like := !removed && permissions.CanLikeReply(ctx)
	reply.Authorized.Hide = &hide
	reply.Authorized.Comment = &canReply
	reply.Authorized.Like = &like
}

// blankRemoved takes out of a removed reply what was not already emptied
// when it was removed: who wrote it. The placeholder says that something was
// here, and nothing about what or whose.
func blankRemoved(reply *entities.AnahitaNode) {
	if reply.DeletedAt == nil {
		return
	}

	reply.Body = ""
	reply.Name = ""
	reply.Author = nil
	reply.Owner = nil
	reply.Editor = nil
	reply.LikesCount = 0
	reply.IsLikedByViewer = false
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
		return c.JSON(http.StatusConflict, map[string]any{
			"error":   "not_a_root",
			"root_id": thread.Root.RootID,
		})
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

	return c.JSON(http.StatusOK, map[string]any{
		"data": threadResponse(replies),
		// For whoever looks after the thread: how many of the replies sent
		// are hidden from everybody else.
		"hidden_count": hidden,
		// Whether the viewer may reply here at all, for a thread with
		// nothing in it yet.
		"can_reply": canReply,
		// True when the thread is longer than is sent at once.
		"truncated": total >= replyThreadLimit,
	})
}

// As many as the graph reads a thread with.
const replyThreadLimit = 500

// replyAddRequest is a new reply: what it says, and the language it says it
// in. Where it goes is in the address; who may see it is the thread's to
// say, so there is no audience to send.
type replyAddRequest struct {
	Body     string `json:"body" form:"body" validate:"required,max=8000"`
	Language string `json:"language" form:"language" validate:"omitempty,max=12"`
}

// AddReply answers a post or a reply.
//
// POST /notes/:id/replies, where :id is what is being answered.
func (h *mediumHandler) AddReply(c echo.Context) error {
	var (
		req    = &replyAddRequest{}
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
	reply.Body = req.Body

	if language, valid := an_handlers.RequestLanguage(req.Language); !valid {
		return an_handlers.RefuseLanguage(c)
	} else if language != "" {
		reply.Language = language
	}

	// A reply belongs to whoever wrote it, wherever the thread is. That is
	// what lets them edit it and take it down, and what a reply is on the
	// networks this will federate with.
	reply.OwnerID = viewer.ID
	reply.OwnerType = libconstants.NodeActorPersonEntityType
	reply.CreatedBy = viewer.ID
	reply.ModifiedBy = viewer.ID
	reply.Enabled = true
	reply.CommentStatus = true

	// Only what it answers is sent. The graph works out the root from
	// that, and gives the reply no audience of its own.
	reply.ParentID = parent.ID

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

// replyHideRequest hides a reply from its thread, or shows it again.
type replyHideRequest struct {
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
		req    = &replyHideRequest{}
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

	return c.JSON(http.StatusOK, map[string]any{
		"id":     reply.ID,
		"hidden": req.Hidden,
	})
}

// replyRuling answers a request about one reply by the rules for its thread:
// which of read, edit and delete the viewer may do. Used by the routes a
// reply shares with every other note, which would otherwise judge it by its
// author's profile.
//
// Returns the thread, and 0 when the answer is yes, or the status to refuse
// with.
func (h *mediumHandler) replyRuling(c echo.Context, reply *entities.AnahitaNode, viewer *entities.Viewer, may func(*permissions.ReplyAccessContext) bool) (*permissions.ReplyAccessContext, int) {
	thread, err := h.thread(c, reply.RootID, viewer)
	if err != nil {
		log.Println(err)
		return nil, http.StatusNotFound
	}

	if !may(forReply(thread, reply)) {
		return thread, http.StatusForbidden
	}

	return thread, 0
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
