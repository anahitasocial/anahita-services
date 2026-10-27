package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/handlers"
	"anahita-libs/permissions"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// A reply with pictures.
//
// Any post can answer another: a reply with pictures is a photo post in the
// thread, as a reply with words alone is a note. It is everything a reply is
// (owned by whoever wrote it, with no audience, no rule for who may answer
// and no rule for quoting of its own, since its thread decides all of
// those) and everything a photo post is: up to four pictures, each with
// what it shows, and words if there are any, which a reply with pictures
// need not have.
//
// The pictures are uploaded first, for the person replying, through
// POST /photos/:owner_id/uploads with their own id, as a photo post's are.

// PhotoReplyRequest is a reply with pictures to the post :id.
type PhotoReplyRequest struct {
	// The id of the post or the reply it answers. Read by the handler from
	// the path itself.
	ParentID int64 `param:"id"`
	// Its words, as Markdown. A reply with pictures need not have any.
	Content string `json:"content" form:"content" validate:"omitempty,max=8000"`
	// The language it is written in, as a BCP 47 tag.
	Language string `json:"language" form:"language" validate:"omitempty,max=12"`
	// Its pictures, each uploaded first for the person replying.
	Uploads []FileRef `json:"uploads" form:"-"`
}

// AddReply answers a post or a reply with pictures.
func (h *photoHandler) AddReply(c echo.Context) error {
	var (
		req    = &PhotoReplyRequest{}
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
	if len(req.Uploads) == 0 {
		return refuseFiles(c, errFilesRequired)
	}
	if len(req.Uploads) > an_constants.PhotoMaxFiles() {
		return refuseFiles(c, errTooManyFiles)
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

	thread, err := handlers.ThreadOf(c, h.readers(), parent, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}
	if !permissions.CanReply(thread) {
		return c.NoContent(http.StatusForbidden)
	}

	// A reply belongs to whoever wrote it, wherever the thread is, and its
	// pictures were uploaded for them.
	author, err := h.Srv.ActorsGRPC.Read(viewer.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	reply := placeholderPhoto(author, viewer)
	reply.Body = req.Content
	reply.Enabled = true
	reply.CommentStatus = true
	reply.AllowDownload = handlers.DefaultAllowDownload(nil, author)

	// In the language it is written in, or failing that the one of what it
	// answers.
	if language, valid := handlers.RequestLanguage(req.Language); !valid {
		return handlers.RefuseLanguage(c)
	} else if language != "" {
		reply.Language = language
	} else {
		reply.Language = parent.Language
	}

	uploads, err := h.Srv.MediaGRPC.ReadPhotoUploads(uploadIDs(req.Uploads))
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	plan, refused := planFiles(nil, uploads, req.Uploads, viewer.ID, author.ID)
	if refused != "" {
		return refuseFiles(c, refused)
	}
	reply.Filename = plan.Files[0].Filename
	reply.Mimetype = plan.Files[0].Mimetype

	// Only what it answers is sent. The graph works out the root from
	// that, and gives the reply no audience of its own.
	reply.ParentID = parent.ID

	reply, err = h.Srv.MediaGRPC.Add(reply, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if _, err := h.Srv.MediaGRPC.SetPhotoFiles(reply.ID, plan.Files, plan.UploadIDs); err != nil {
		log.Println(err)
		// A photo reply with no pictures is worse than none. The uploads
		// are still uploads, so the same request can be sent again.
		if err := h.Srv.MediaGRPC.Delete(reply.ID, viewer); err != nil {
			log.Println(err)
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	reply, err = h.Srv.MediaGRPC.Read(reply.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	handlers.SetReplyFlags(thread, reply)
	batchPresignPaths(h.Env, []*entities.AnahitaNode{reply})

	mediaResp := &responses.MediumNodeResponse{}
	return c.JSON(http.StatusCreated, mediaResp.MapMediumNode(reply))
}

// readers are what a thread is read with.
func (h *photoHandler) readers() handlers.ThreadReaders {
	return handlers.ThreadReaders{Media: h.Srv.MediaGRPC, Actors: h.Srv.ActorsGRPC, Socialgraph: h.Srv.SocialgraphGRPC}
}

// refuseReply answers a request about a photo that is a reply by the rules
// for its thread, as text-service does for a note: 0 when the viewer may.
func (h *photoHandler) refuseReply(c echo.Context, reply *entities.AnahitaNode, viewer *entities.Viewer, may func(*permissions.ReplyAccessContext) bool) int {
	_, status := handlers.ReplyRuling(c, h.readers(), reply, viewer, may)
	return status
}

// mayRead, mayEdit and mayDelete judge a photo post by the rule for photo
// posts, and one that is a reply by the rule for its thread.
func (h *photoHandler) mayRead(c echo.Context, medium *entities.AnahitaNode, viewer *entities.Viewer, ctx *permissions.MediumAccessContext) (bool, error) {
	if medium.RootID > 0 {
		return h.judgedByThread(c, medium, viewer, permissions.CanSeeThread)
	}
	return h.Perms.Media.CanRead(ctx)
}

func (h *photoHandler) mayEdit(c echo.Context, medium *entities.AnahitaNode, viewer *entities.Viewer, ctx *permissions.MediumAccessContext) (bool, error) {
	if medium.RootID > 0 {
		return h.judgedByThread(c, medium, viewer, permissions.CanEditReply)
	}
	return h.Perms.Media.CanEdit(ctx)
}

func (h *photoHandler) mayDelete(c echo.Context, medium *entities.AnahitaNode, viewer *entities.Viewer, ctx *permissions.MediumAccessContext) (bool, error) {
	if medium.RootID > 0 {
		return h.judgedByThread(c, medium, viewer, permissions.CanDeleteReply)
	}
	return h.Perms.Media.CanDelete(ctx)
}

func (h *photoHandler) judgedByThread(c echo.Context, reply *entities.AnahitaNode, viewer *entities.Viewer, may func(*permissions.ReplyAccessContext) bool) (bool, error) {
	if status := h.refuseReply(c, reply, viewer, may); status != 0 {
		return false, fmt.Errorf("reply %d: refused by its thread (%d)", reply.ID, status)
	}
	return true, nil
}
