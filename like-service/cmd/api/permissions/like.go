package permissions

import (
	"anahita-libs/cmd/api/permissions"
	libs_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"errors"
	"fmt"
)

const ErrInsufficientScope = "insufficient scope"

type LikePermissionsInterface interface {
	CanBrowse(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error)
	CanAdd(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error)
	CanDelete(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error)
}

type LikePermissions struct {
	*Env
}

func NewLikePermissions(env *Env) LikePermissionsInterface {
	return &LikePermissions{
		Env: env,
	}
}

func (p *LikePermissions) CanBrowse(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error) {
	// Step 1: Super admins can always browse
	if viewer.IsAdmin() {
		return true, nil
	}

	// Case A: the node is a reply
	if judgedByParent(node) {
		parent := node.GetParent()

		commentParentCtx := &permissions.MediumAccessContext{
			Viewer: viewer,
			Medium: parent,
		}

		actorIds := []int64{parent.GetOwnerID(), parent.GetAuthorID()}
		relationships, err := p.Srv.SocialgraphGRPC.Browse(actorIds, viewer.ID)
		if err != nil {
			return false, err
		}

		commentParentCtx.OwnerViewerRel = relationships[parent.GetOwnerID()]
		commentParentCtx.AuthorViewerRel = relationships[parent.GetAuthorID()]

		// Step 2A: Ensure viewer can read the parent node
		mediumPerm := permissions.NewMediumPermissions()
		ok, err := mediumPerm.CanRead(commentParentCtx)
		if err != nil || !ok {
			return false, fmt.Errorf("viewer %d cannot read parent %d", viewer.ID, parent.GetID())
		}

		return true, nil
	}

	// Case B: The node is a medium
	if node.IsMedium() {
		mediumAccessCtx := &permissions.MediumAccessContext{
			Viewer: viewer,
			Medium: node,
		}

		relationships, err := p.Srv.SocialgraphGRPC.Browse([]int64{node.GetOwnerID(), node.GetAuthorID()}, viewer.ID)
		if err != nil {
			return false, err
		}

		mediumAccessCtx.OwnerViewerRel = relationships[node.GetOwnerID()]
		mediumAccessCtx.AuthorViewerRel = relationships[node.GetAuthorID()]

		// Step 2B: Ensure viewer can read the medium node
		mediumPerm := permissions.NewMediumPermissions()
		ok, err := mediumPerm.CanRead(mediumAccessCtx)
		if err != nil || !ok {
			return false, fmt.Errorf("viewer %d cannot read medium %d", viewer.ID, node.GetID())
		}

		return true, nil
	}

	// Unsupported node type
	return false, errors.New("you are not allowed to browse this content")
}

func (p *LikePermissions) CanAdd(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error) {
	// Before the admin short-circuit in CanLike: a role never bypasses the
	// scope gate, or a token deliberately narrowed to exclude likes would
	// still be able to like as soon as its holder is an admin.
	if !viewer.HasScope(libs_constants.OAuthScopeLikesWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	// Nothing from another server is liked from here yet, a reply under a
	// post here included: whoever wrote it would never be told, so the like
	// would be a lie to whoever made it.
	if node != nil && node.IsRemote() {
		return false, errors.New("what came from another server is not liked from here")
	}

	// A reply is liked under the rules of the post at the top of its thread.
	post := node
	if judgedByParent(node) {
		post = node.GetParent()
	} else if !node.IsMedium() {
		return false, errors.New("you are not allowed to like this content")
	}

	if post == nil || post.GetOwner() == nil {
		return false, fmt.Errorf("viewer %d cannot like node %d", viewer.ID, node.GetID())
	}

	rel, err := p.Srv.SocialgraphGRPC.Read(post.GetOwner().GetID(), viewer.ID)
	if err != nil {
		return false, err
	}

	// permissions.CanLike is also what answers authorized.like on the posts
	// and comments the web app shows, so the button and this check agree.
	if permissions.CanLike(viewer, post, rel) {
		return true, nil
	}

	return false, fmt.Errorf("viewer %d cannot like node %d", viewer.ID, node.GetID())
}

func (p *LikePermissions) CanDelete(viewer *entities.Viewer, node *entities.AnahitaNode) (bool, error) {
	if !viewer.HasScope(libs_constants.OAuthScopeLikesWrite) {
		return false, errors.New(ErrInsufficientScope)
	}

	if node.IsLikedByViewer {
		return true, nil
	}

	return false, fmt.Errorf("viewer %d has not liked the node %d", viewer.ID, node.GetID())
}

// judgedByParent reports whether a like on this node is decided by another
// post, the one the handler has loaded as its Parent, and not by the node
// itself: a reply, by the post at the top of its thread.
//
// A reply is a note owned by its author. Judged by itself it would be liked
// under the rules of its author's profile, by anybody who can see that
// profile, wherever the thread is.
func judgedByParent(node *entities.AnahitaNode) bool {
	return node.RootID > 0
}
