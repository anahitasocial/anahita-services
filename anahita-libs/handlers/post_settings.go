package handlers

import (
	"log"
	"net/http"
	"strconv"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_mw "anahita-libs/middleware"
	"anahita-libs/permissions"
	"anahita-libs/requests"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// PostSettings answers the requests that change one setting of a post: who
// may see it, whether and who may reply, who may quote it, its content
// warning, and its pin. The same for every kind of post; text-service,
// photo-service and video-service each give it their clients and how they
// answer with a post (roadmap row 170: each kept a copy, about a thousand
// lines, which had begun to differ).
//
// Each setting has a route of its own, not a field of the post's edit: an
// edit form that did not send it must not change it.
type PostSettings struct {
	Media       services.MediaGRPCInterface
	Socialgraph services.SocialgraphGRPCInterface
	Perms       permissions.MediumPermissionsInterface
	// Thread judges a reply by the post at the top of its thread.
	Thread ThreadReaders
	// Answer writes the post, as its service answers with one, after it
	// was changed: what the viewer may do with it, its pictures signed.
	Answer func(c echo.Context, post *entities.AnahitaNode) error
}

// AccessContext is who the viewer is to a post's owner and its author, for
// the permissions that decide what they may do with it.
func AccessContext(socialgraph services.SocialgraphGRPCInterface, post *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.MediumAccessContext, error) {
	ctx := &permissions.MediumAccessContext{
		Viewer: viewer,
		Medium: post,
	}
	relationships, err := socialgraph.Browse([]int64{post.GetOwnerID(), post.GetAuthorID()}, viewer.ID)
	if err != nil {
		return nil, err
	}
	ctx.OwnerViewerRel = relationships[post.GetOwnerID()]
	ctx.AuthorViewerRel = relationships[post.GetAuthorID()]
	return ctx, nil
}

// replyFollowsItsThread is the answer to a setting asked of a reply, which
// has none of its own: it follows the post at the top of its thread, and a
// reply is not quoted.
const replyFollowsItsThread = "reply_follows_its_thread"

// bound reads a request into req, and answers 400 when it cannot.
func bound(c echo.Context, req any) bool {
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return false
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return false
	}
	return true
}

// read reads the post a request is about. When it cannot, it has answered
// the request, and the post is nil.
func (s PostSettings) read(c echo.Context, id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	post, err := s.Media.Read(id, viewer)
	if err != nil {
		log.Println(err)
		return nil, c.NoContent(http.StatusNotFound)
	}
	return post, nil
}

// mayEdit says whether the viewer may change a setting of a post: not a
// reply's, and only whoever may edit the post. When not, it has answered
// the request, and the context is nil.
func (s PostSettings) mayEdit(c echo.Context, post *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.MediumAccessContext, error) {
	if post.RootID > 0 {
		return nil, c.JSON(http.StatusUnprocessableEntity, responses.Refusal{Error: replyFollowsItsThread})
	}
	ctx, err := AccessContext(s.Socialgraph, post, viewer)
	if err != nil {
		log.Println(err)
		return nil, c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := s.Perms.CanEdit(ctx); !ok {
		log.Println(err)
		return nil, c.NoContent(http.StatusForbidden)
	}
	return ctx, nil
}

// editable is read, then mayEdit.
func (s PostSettings) editable(c echo.Context, id int64, viewer *entities.Viewer) (*entities.AnahitaNode, *permissions.MediumAccessContext, error) {
	post, answered := s.read(c, id, viewer)
	if post == nil {
		return nil, nil, answered
	}
	ctx, answered := s.mayEdit(c, post, viewer)
	if ctx == nil {
		return nil, nil, answered
	}
	return post, ctx, nil
}

// saved keeps a changed post and answers with it.
func (s PostSettings) saved(c echo.Context, post *entities.AnahitaNode, viewer *entities.Viewer) error {
	// A change of a setting is a modification, and the edit's validator
	// requires a modifier.
	post.ModifiedBy = viewer.ID
	post, err := s.Media.Edit(post, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return s.Answer(c, post)
}

// EditAccess changes who may see a post.
//
// PATCH /{kind}/:id/access
func (s PostSettings) EditAccess(c echo.Context) error {
	req, viewer := requests.NewMediumRequest().EditAccess, an_mw.GetViewer(c)
	if !bound(c, req) {
		return c.NoContent(http.StatusBadRequest)
	}

	post, answered := s.read(c, req.ID, viewer)
	if post == nil {
		return answered
	}
	// A channel's own post is public, whoever asks. A reply is answered
	// that it follows its thread first.
	if post.RootID == 0 && permissions.IsChannelFeedItem(post) {
		return c.JSON(http.StatusForbidden, responses.Refusal{Error: permissions.ChannelFeedItemRefusal})
	}
	ctx, answered := s.mayEdit(c, post, viewer)
	if ctx == nil {
		return answered
	}

	// The same rule as when a post is created, so it cannot be given by
	// editing an audience it could not have been posted with. Keeping the
	// audience it already has is always allowed: a rule that came later
	// does not reach back.
	if !permissions.CanChangeMediumAudience(ctx, post.Access, req.Access) {
		log.Printf("refused: audience %q is not allowed on this profile", req.Access)
		return RefuseAudience(c, ctx)
	}
	post.Access = DetermineMediumNodeAccess(req.Access, post.Owner.Access)
	return s.saved(c, post, viewer)
}

// EditCommentStatus opens a post to replies or closes it.
//
// PATCH /{kind}/:id/comment-status
func (s PostSettings) EditCommentStatus(c echo.Context) error {
	req, viewer := requests.NewMediumRequest().EditCommentStatus, an_mw.GetViewer(c)
	if !bound(c, req) {
		return c.NoContent(http.StatusBadRequest)
	}
	post, _, answered := s.editable(c, req.ID, viewer)
	if post == nil {
		return answered
	}
	SetRepliesOpen(post, req.Status)
	return s.saved(c, post, viewer)
}

// EditReplyAccess changes who may reply to a post: anyone, nobody, or any
// of the people who follow its author, the people its author follows, and
// the people it mentions.
//
// PATCH /{kind}/:id/reply-access
func (s PostSettings) EditReplyAccess(c echo.Context) error {
	req, viewer := &requests.MediumEditReplyAccessRequest{}, an_mw.GetViewer(c)
	if !bound(c, req) {
		return c.NoContent(http.StatusBadRequest)
	}
	replyAccess, valid := RequestReplyAccess(req.ReplyAccess)
	if !valid || replyAccess == "" {
		return RefuseReplyAccess(c)
	}
	post, _, answered := s.editable(c, req.ID, viewer)
	if post == nil {
		return answered
	}
	SetReplyAccess(post, replyAccess)
	return s.saved(c, post, viewer)
}

// EditQuotePolicy changes who may quote a post: anyone, the people who
// follow its author, or nobody. It decides who may quote from now on: a
// note that quotes the post already stays as it is, and the post's author
// takes theirs out of it one at a time.
//
// PATCH /{kind}/:id/quote-policy
func (s PostSettings) EditQuotePolicy(c echo.Context) error {
	req, viewer := &requests.MediumEditQuotePolicyRequest{}, an_mw.GetViewer(c)
	if !bound(c, req) {
		return c.NoContent(http.StatusBadRequest)
	}
	quotePolicy, valid := RequestQuotePolicy(req.QuotePolicy)
	if !valid || quotePolicy == "" {
		return RefuseQuotePolicy(c)
	}
	post, _, answered := s.editable(c, req.ID, viewer)
	if post == nil {
		return answered
	}
	post.QuotePolicy = quotePolicy
	return s.saved(c, post, viewer)
}

// EditWarning gives a post a content warning, changes it, or takes it off
// (roadmap row 122). Whoever may edit the post may; a reply is changed by
// its author alone, as its words are. A warning a moderator gave stays as
// it is: its author is answered `warning_not_allowed`.
//
// PATCH /{kind}/:id/warning
func (s PostSettings) EditWarning(c echo.Context) error {
	req, viewer := &requests.MediumEditWarningRequest{}, an_mw.GetViewer(c)
	if !bound(c, req) {
		return c.NoContent(http.StatusBadRequest)
	}

	post, answered := s.read(c, req.ID, viewer)
	if post == nil {
		return answered
	}
	if post.RootID > 0 && post.DeletedAt != nil {
		return c.NoContent(http.StatusGone)
	}

	// A site administrator warns as a moderator what they did not write,
	// on any post; anybody else as whoever may edit it.
	moderator := IsWarningModerator(post, viewer)
	if !moderator {
		if post.RootID > 0 {
			if _, refused := ReplyRuling(c, s.Thread, post, viewer, permissions.CanEditReply); refused != 0 {
				return c.NoContent(refused)
			}
		} else {
			if permissions.IsChannelFeedItem(post) {
				return c.JSON(http.StatusForbidden, responses.Refusal{Error: permissions.ChannelFeedItemRefusal})
			}
			if ctx, answered := s.mayEdit(c, post, viewer); ctx == nil {
				return answered
			}
		}
	}

	valid, allowed := SetWarningBy(post, moderator, req.Warning, req.WarningText)
	if !valid {
		return RefuseWarning(c)
	}
	if !allowed {
		return RefuseWarningNotAllowed(c)
	}
	return s.saved(c, post, viewer)
}

// Pin pins a post to the profile it is on, so that it leads the profile's
// lists; Unpin takes the pin off. For whoever the profile belongs to: a
// person on their own profile, a group's admins in the group
// (permissions.CanPin). A profile has one pin (constants.MaxPinnedPosts),
// so pinning a post moves the pin to it from whichever had it.
//
// PUT and DELETE /{kind}/:id/pin
func (s PostSettings) Pin(c echo.Context) error {
	return s.setPinned(c, true)
}

func (s PostSettings) Unpin(c echo.Context) error {
	return s.setPinned(c, false)
}

func (s PostSettings) setPinned(c echo.Context, pinned bool) error {
	viewer := an_mw.GetViewer(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	post, answered := s.read(c, id, viewer)
	if post == nil {
		return answered
	}
	// A reply is read in its thread. It is not pinned.
	if post.RootID > 0 {
		return c.JSON(http.StatusUnprocessableEntity, responses.Refusal{Error: "reply_cannot_be_pinned"})
	}
	ctx, err := AccessContext(s.Socialgraph, post, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if !permissions.CanPin(viewer, post, ctx.OwnerViewerRel) {
		return c.NoContent(http.StatusForbidden)
	}

	if err := s.Media.SetPinned(post.ID, pinned, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if post, err = s.Media.Read(id, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return s.Answer(c, post)
}
