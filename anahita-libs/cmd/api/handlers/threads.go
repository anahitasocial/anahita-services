package handlers

import (
	"log"
	"net/http"

	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"

	"github.com/labstack/echo/v4"
)

// Threads, for every service whose posts can be replies.
//
// A reply can be any kind of post: a note, a photo post, a video post. It is
// owned by its author and has no audience of its own; everything about it is
// decided by the post at the top of its thread (permissions/reply.go). These
// are what a service needs to judge a reply by that rule: the thread read
// once per request, what the viewer may do with a reply, and the answer to a
// request about one.

// ThreadReaders are what a thread is read with.
type ThreadReaders struct {
	Media       services.MediaGRPCInterface
	Actors      services.ActorsGRPCInterface
	Socialgraph services.SocialgraphGRPCInterface
}

// requestThreadsKey holds the threads read so far in this request, by root.
const requestThreadsKey = "anahita.replies.threads"

// Thread reads the post at the top of a thread as the viewer, with the
// profile it is on and that profile's relationship to the viewer: what every
// rule about a reply is judged against.
//
// Once per root per request, when there is a request: a thread is many
// replies with one root. c may be nil, as in a worker.
func Thread(c echo.Context, r ThreadReaders, rootID int64, viewer *entities.Viewer) (*permissions.ReplyAccessContext, error) {
	var threads map[int64]*permissions.ReplyAccessContext
	if c != nil {
		threads, _ = c.Get(requestThreadsKey).(map[int64]*permissions.ReplyAccessContext)
		if threads == nil {
			threads = map[int64]*permissions.ReplyAccessContext{}
			c.Set(requestThreadsKey, threads)
		}
		if ctx, ok := threads[rootID]; ok {
			return ctx, nil
		}
	}

	root, err := r.Media.Read(rootID, viewer)
	if err != nil {
		return nil, err
	}

	// The owner as the actor read returns it: its access, and its
	// permissions with the defaults merged in. A post carries only a
	// summary of its owner, which has neither.
	owner, err := r.Actors.Read(root.OwnerID, viewer)
	if err != nil {
		return nil, err
	}
	root.Owner = owner

	// The author's relationship as well as the profile's. They are the
	// same actor for a post on its author's own profile and not for one in
	// a group, and who the author lets reply is measured against the author.
	actorIDs := []int64{root.OwnerID}
	if authorID := root.GetAuthorID(); authorID > 0 && authorID != root.OwnerID {
		actorIDs = append(actorIDs, authorID)
	}

	rels, err := r.Socialgraph.Browse(actorIDs, viewer.ID)
	if err != nil {
		return nil, err
	}

	ctx := &permissions.ReplyAccessContext{
		Viewer:              viewer,
		Root:                root,
		RootOwnerViewerRel:  rels[root.OwnerID],
		RootAuthorViewerRel: rels[root.GetAuthorID()],
	}
	if threads != nil {
		threads[rootID] = ctx
	}

	return ctx, nil
}

// ThreadOf reads the thread a post would answer, given that post: the
// thread it is in, or the one it begins.
func ThreadOf(c echo.Context, r ThreadReaders, parent *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.ReplyAccessContext, error) {
	rootID := parent.RootID
	if rootID <= 0 {
		rootID = parent.ID
	}
	return Thread(c, r, rootID, viewer)
}

// ForReply is the thread's context with one reply in it.
func ForReply(thread *permissions.ReplyAccessContext, reply *entities.AnahitaNode) *permissions.ReplyAccessContext {
	withReply := *thread
	withReply.Reply = reply
	return &withReply
}

// SetReplyFlags answers what the viewer may do with a reply, onto its
// Authorized, from the rules for its thread.
func SetReplyFlags(thread *permissions.ReplyAccessContext, reply *entities.AnahitaNode) {
	if reply.Authorized == nil {
		reply.Authorized = &entities.AnahitaNodeAuthorized{}
	}

	ctx := ForReply(thread, reply)

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
	// What is said in a group's thread is the group's to keep someone from.
	blockFromOwner := !removed && permissions.CanBlockFromOwner(ctx.Viewer, ctx.Root.Owner, reply.GetAuthorID(), ctx.RootOwnerViewerRel)
	reply.Authorized.BlockFromOwner = &blockFromOwner
}

// ReplyRuling answers a request about one reply by the rules for its thread:
// which of read, edit and delete the viewer may do. Used by the routes a
// reply shares with every other post of its kind, which would otherwise
// judge it by its author's profile.
//
// Returns the thread, and 0 when the answer is yes, or the status to refuse
// with.
func ReplyRuling(c echo.Context, r ThreadReaders, reply *entities.AnahitaNode, viewer *entities.Viewer, may func(*permissions.ReplyAccessContext) bool) (*permissions.ReplyAccessContext, int) {
	thread, err := Thread(c, r, reply.RootID, viewer)
	if err != nil {
		log.Println(err)
		return nil, http.StatusNotFound
	}

	if !may(ForReply(thread, reply)) {
		return thread, http.StatusForbidden
	}

	return thread, 0
}

// BlankRemoved takes out of a removed reply what was not already emptied
// when it was removed: who wrote it, and what it showed. The placeholder
// says that something was here, and nothing about what or whose.
func BlankRemoved(reply *entities.AnahitaNode) {
	if reply.DeletedAt == nil {
		return
	}

	reply.Body = ""
	reply.Name = ""
	// With its words went the card for a link in them, and a video's
	// files, which are described in its meta.
	reply.Meta = nil
	// And a photo reply's pictures.
	reply.Files = nil
	reply.Filename = ""
	reply.Portrait = nil
	reply.Author = nil
	reply.Owner = nil
	reply.Editor = nil
	reply.LikesCount = 0
	reply.IsLikedByViewer = false
}

// MayReplyTo says whether the viewer may answer the post parentID now: it is
// there, it is not a removed reply, and its thread lets them. For what makes
// a reply later than it was asked for, as the video worker does once a
// video is ready. An error is one of reading, which may pass; false is an
// answer.
func MayReplyTo(r ThreadReaders, parentID int64, viewer *entities.Viewer) (bool, error) {
	parent, err := r.Media.Read(parentID, viewer)
	if err != nil {
		return false, err
	}
	if parent == nil || (parent.RootID > 0 && parent.DeletedAt != nil) {
		return false, nil
	}
	thread, err := ThreadOf(nil, r, parent, viewer)
	if err != nil {
		return false, err
	}
	return permissions.CanReply(thread), nil
}
