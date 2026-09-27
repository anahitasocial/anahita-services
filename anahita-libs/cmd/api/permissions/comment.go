package permissions

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"errors"
)

const (
	ErrCommentInsufficientScope = "insufficient scope"
	ErrCommentEditNotAllowed    = "you are not allowed to edit this comment"
	ErrCommentAddNotAllowed     = "you are not allowed to add a comment"
	ErrCommentDeleteNotAllowed  = "you are not allowed to delete this comment"
	ErrCommentAdminNotAllowed   = "you are not allowed to administrate this comment"
)

// CommentParentNodeInterface
type CommentParentNodeInterface interface {
	GetID() int64
	GetAuthorID() int64
	GetOwnerID() int64
	GetAccess() string
	GetOwner() *entities.AnahitaNode
	GetAuthor() *entities.AnahitaNode
	GetService() string
	GetEntityName() string
}

// CommentPermissionsInterface
type CommentPermissionsInterface interface {
	CanBrowse(ctx *MediumAccessContext) (bool, error)
	CanRead(ctx *CommentAccessContext) (bool, error)
	CanEdit(ctx *CommentAccessContext) (bool, error)
	CanAdd(ctx *CommentAccessContext) (bool, error)
	CanDelete(ctx *CommentAccessContext) (bool, error)
	CanAdministrate(ctx *CommentAccessContext) (bool, error)
}

// CommentPermissions
type CommentPermissions struct{}

// NewCommentPermissions creates a new CommentPermissions service
func NewCommentPermissions() CommentPermissionsInterface {
	return &CommentPermissions{}
}

// CanBrowse checks if the viewer can browse the comments
// Comments are visible if the parent media is readable
func (c *CommentPermissions) CanBrowse(ctx *MediumAccessContext) (bool, error) {
	mediumPerm := NewMediumPermissions()
	ok, err := mediumPerm.CanRead(ctx)
	if ok {
		return true, nil
	}

	return false, err
}

// CanRead checks if the viewer can read the comment
// Comments are readable if the parent media is readable
func (c *CommentPermissions) CanRead(ctx *CommentAccessContext) (bool, error) {
	mediumPerm := NewMediumPermissions()
	mediumPermCtx := &MediumAccessContext{
		Viewer:         ctx.Viewer,
		Medium:         ctx.Comment.GetParent(),
		OwnerViewerRel: ctx.ParentOwnerViewerRel,
	}
	ok, err := mediumPerm.CanRead(mediumPermCtx)
	if ok {
		return true, nil
	}

	return false, err
}

// CanEdit checks if the viewer can edit the comment.
//
// The author only. Not site administrators, not the profile's owner or
// administrators: an edited comment still carries its author's name, so
// anybody else editing it puts words under that name, with no history to show
// who wrote them. Moderators remove comments; they do not rewrite them — see
// CanDelete.
//
// The author also has to be able to read the post still. Somebody the owner
// has since blocked, or a profile that has since narrowed its access, should
// not be able to keep rewriting what sits under it.
func (c *CommentPermissions) CanEdit(ctx *CommentAccessContext) (bool, error) {
	if !ctx.Viewer.RequireScope(constants.OAuthScopeCommentsWrite) {
		return false, errors.New(ErrCommentInsufficientScope)
	}

	if ctx.Viewer.ID <= 0 || ctx.Viewer.ID != ctx.Comment.GetAuthorID() {
		return false, errors.New(ErrCommentEditNotAllowed)
	}

	parent := ctx.Comment.GetParent()
	if parent == nil || parent.GetID() == 0 {
		return false, errors.New(ErrCommentEditNotAllowed)
	}

	mediumPermCtx := &MediumAccessContext{
		Viewer:         ctx.Viewer,
		Medium:         parent,
		OwnerViewerRel: relationOrEmpty(ctx.ParentOwnerViewerRel),
	}
	if ok, _ := NewMediumPermissions().CanRead(mediumPermCtx); !ok {
		return false, errors.New(ErrCommentEditNotAllowed)
	}

	return true, nil
}

// CanAdd checks if the viewer can add a comment
func (c *CommentPermissions) CanAdd(ctx *CommentAccessContext) (bool, error) {
	if !ctx.Viewer.RequireScope(constants.OAuthScopeCommentsWrite) {
		return false, errors.New(ErrCommentInsufficientScope)
	}

	if ctx.Viewer.IsAdmin() {
		return true, nil
	}

	// The caller has to have loaded the post and its owner. Without them
	// every check below runs against an empty placeholder — which is how,
	// before the comment handler loaded them, a post nobody could read took
	// comments from anybody.
	parent := ctx.Comment.GetParent()
	if parent == nil || parent.GetID() == 0 {
		return false, errors.New(ErrCommentAddNotAllowed)
	}
	parentOwner := parent.GetOwner()
	if parentOwner == nil || parentOwner.GetID() == 0 {
		return false, errors.New(ErrCommentAddNotAllowed)
	}

	// The profile's owner, its administrators and the post's own author can
	// always comment — on a post they can see, which the owner and
	// administrators always can and the author, having written it, can.
	if ctx.Viewer.ID == parentOwner.GetID() || ctx.Viewer.ID == parent.GetAuthorID() {
		return true, nil
	}

	// ActorPermissions.CanRead dereferences the relationship, and a viewer
	// with none to the owner is an ordinary case, not an error.
	rel := relationOrEmpty(ctx.ParentOwnerViewerRel)

	if rel.HasAdmin {
		return true, nil
	}

	// Everybody else has to be able to read the post before anything else.
	// This used to be the last check and a sufficient one, which made the
	// comment setting unenforceable: any reader could comment whatever it
	// said.
	mediumPermCtx := &MediumAccessContext{
		Viewer:         ctx.Viewer,
		Medium:         parent,
		OwnerViewerRel: rel,
	}
	if ok, _ := NewMediumPermissions().CanRead(mediumPermCtx); !ok {
		return false, errors.New(ErrCommentAddNotAllowed)
	}

	access := CommentAccess(parentOwner, parent.GetService())
	if access == "" || AddAccessAllows(access, rel, ctx.Viewer) {
		return true, nil
	}

	return false, errors.New(ErrCommentAddNotAllowed)
}

// CanDelete checks if the viewer can delete the comment: its author, the
// profile the post is on — the person, or for a group its administrators —
// and site administrators.
//
// No read check for the author. Taking your own words down should work even
// after you have lost sight of the post they sit under.
func (c *CommentPermissions) CanDelete(ctx *CommentAccessContext) (bool, error) {
	if !ctx.Viewer.RequireScope(constants.OAuthScopeCommentsWrite) {
		return false, errors.New(ErrCommentInsufficientScope)
	}

	if canModerate(ctx) {
		return true, nil
	}

	return false, errors.New(ErrCommentDeleteNotAllowed)
}

// CanAdministrate checks if the viewer can administrate the comment. The same
// people as CanDelete: moderating a comment is removing it.
func (c *CommentPermissions) CanAdministrate(ctx *CommentAccessContext) (bool, error) {
	if !ctx.Viewer.RequireScope(constants.OAuthScopeCommentsWrite) {
		return false, errors.New(ErrCommentInsufficientScope)
	}

	if canModerate(ctx) {
		return true, nil
	}

	return false, errors.New(ErrCommentAdminNotAllowed)
}

// canModerate is who may take a comment down.
//
// The profile's owner and administrators are only recognised when the caller
// loaded the post: against the empty placeholder parent the owner id is 0,
// and the relationship is the viewer's to nobody.
func canModerate(ctx *CommentAccessContext) bool {
	if ctx.Viewer.IsAdmin() {
		return true
	}

	if ctx.Viewer.ID <= 0 {
		return false
	}

	if ctx.Viewer.ID == ctx.Comment.GetAuthorID() {
		return true
	}

	parent := ctx.Comment.GetParent()
	if parent == nil || parent.GetOwnerID() == 0 {
		return false
	}

	if ctx.Viewer.ID == parent.GetOwnerID() {
		return true
	}

	return relationOrEmpty(ctx.ParentOwnerViewerRel).HasAdmin
}

// relationOrEmpty stands in for a missing relationship. A viewer with none to
// the owner is ordinary, and ActorPermissions.CanRead dereferences it.
func relationOrEmpty(rel *entities.ActorRelation) *entities.ActorRelation {
	if rel == nil {
		return &entities.ActorRelation{}
	}
	return rel
}
