package permissions

import (
	"strings"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// The rules for replies.
//
// A reply is a note with a parent. It is owned by its author and has no
// audience of its own, so NOTHING about it is decided the way it is for an
// ordinary post, by its owner and its access. Everything is decided by the
// post at the top of its thread, the root:
//
//   - who can see it: whoever can see the root;
//   - who can reply: whoever could comment on the root, narrowed to the
//     people the root's author chose, when they chose;
//   - who can remove it: its author, and whoever looks after the thread;
//   - who can hide it from the thread: whoever looks after the thread.
//
// "Whoever looks after the thread" is the author of the root, the profile the
// root is on (the person, or a group's administrators), and site
// administrators. The root's author is in that list wherever the post is, so
// somebody who posts in a group can deal with what is said under their post
// without being one of the group's administrators.
//
// These are functions over a context, like the capabilities, and not methods
// of MediumPermissions: handing a reply to MediumPermissions judges it by its
// author's profile, which is exactly the mistake these exist to prevent.

// ReplyAccessContext is a reply and the thread it is in.
type ReplyAccessContext struct {
	Viewer *entities.Viewer
	// The reply. Nil when asking about a reply that does not exist yet.
	Reply *entities.AnahitaNode
	// The post at the top of the thread, with its Owner loaded as the actor
	// read returns it: its access, and its permissions with the defaults
	// merged in.
	Root *entities.AnahitaNode
	// The root's owner's relationship to the viewer.
	RootOwnerViewerRel *entities.ActorRelation
	// The root's author's relationship to the viewer. The same as the
	// owner's for a post on its author's own profile, and not for one in a
	// group. Only who may reply reads it, and only when the author limited
	// replies to people they follow or who follow them.
	RootAuthorViewerRel *entities.ActorRelation
}

func (ctx *ReplyAccessContext) rootLoaded() bool {
	return ctx != nil && ctx.Viewer != nil && ctx.Root != nil && ctx.Root.GetID() > 0 &&
		ctx.Root.GetOwner() != nil && ctx.Root.GetOwner().GetID() > 0
}

// CanSeeThread reports whether the viewer may read the root, and so every
// reply under it.
func CanSeeThread(ctx *ReplyAccessContext) bool {
	if !ctx.rootLoaded() {
		return false
	}

	ok, _ := NewMediumPermissions().CanRead(&MediumAccessContext{
		Viewer:         ctx.Viewer,
		Medium:         ctx.Root,
		OwnerViewerRel: relationOrEmpty(ctx.RootOwnerViewerRel),
	})
	return ok
}

// LooksAfterThread reports whether the viewer is one of the people who may
// remove or hide what is said in this thread.
func LooksAfterThread(ctx *ReplyAccessContext) bool {
	if !ctx.rootLoaded() || ctx.Viewer.ID <= 0 {
		return false
	}

	if ctx.Viewer.IsAdmin() {
		return true
	}

	if ctx.Viewer.ID == ctx.Root.GetAuthorID() || ctx.Viewer.ID == ctx.Root.GetOwnerID() {
		return true
	}

	return relationOrEmpty(ctx.RootOwnerViewerRel).HasAdmin
}

// RepliesLimitedTo is who the root's author lets reply, as the stored value
// is read: "anyone", "nobody", or the groups. A post whose switch for
// replies is off reads as "nobody" whatever else it says, which is every
// post closed before authors could choose.
//
// A post from another server reads as "anyone", as CanReply has it: who
// its author lets reply is their server's to hold to, and nothing of it is
// kept here. Its switch is never set, and read as off it said "Replies are
// closed" over a box anybody here could reply in.
func RepliesLimitedTo(root *entities.AnahitaNode) string {
	if root == nil {
		return constants.ReplyAccessNobody
	}
	if root.IsRemote() {
		return constants.ReplyAccessAnyone
	}

	return constants.EffectiveReplyAccess(root.ReplyAccess, root.CommentStatus)
}

// chosenToReply reports whether the viewer is in one of the groups the
// root's author limited replies to. Any one group is enough.
func chosenToReply(ctx *ReplyAccessContext, limitedTo string) bool {
	rel := relationOrEmpty(ctx.RootAuthorViewerRel)

	// The relation is the author's to the viewer: "leading" is the viewer
	// following the author, "following" is the author following the viewer.
	if constants.ReplyAccessHas(limitedTo, constants.ReplyAccessFollowers) && rel.IsLeading {
		return true
	}

	if constants.ReplyAccessHas(limitedTo, constants.ReplyAccessFollowing) && rel.IsFollowing {
		return true
	}

	if constants.ReplyAccessHas(limitedTo, constants.ReplyAccessMentioned) && ctx.Viewer.Username != "" {
		for _, mention := range ctx.Root.GetMentions() {
			if strings.EqualFold(mention, ctx.Viewer.Username) {
				return true
			}
		}
	}

	return false
}

// CanReply reports whether the viewer may add a reply to this thread: the
// rule for commenting on the root, and then whoever the root's author
// limited replies to.
//
// The author's choice narrows and never widens. Somebody the profile does
// not let comment cannot reply because the post mentions them.
//
// Whoever looks after the thread can reply to it whatever was chosen.
// Closing it is how they stop other people, not themselves.
func CanReply(ctx *ReplyAccessContext) bool {
	if !ctx.rootLoaded() || !ctx.Viewer.Authenticated {
		return false
	}

	// A reply is a post. A token that may not write posts may not write
	// this one.
	if !ctx.Viewer.HasScope(constants.OAuthScopeMediaWrite) {
		return false
	}

	if !CanSeeThread(ctx) {
		return false
	}

	// A thread that began on another server. Whoever can read it here may
	// answer it here: who its author lets reply is theirs to say and their
	// server's to hold to, and nothing of that is kept here. The answer
	// reaches them when its writer is on the fediverse, and is otherwise
	// a conversation among the people of this installation.
	if ctx.Root.IsRemote() {
		return true
	}

	if LooksAfterThread(ctx) {
		return true
	}

	limitedTo := RepliesLimitedTo(ctx.Root)
	if limitedTo == constants.ReplyAccessNobody {
		return false
	}

	rel := relationOrEmpty(ctx.RootOwnerViewerRel)
	access := CommentAccess(ctx.Root.GetOwner(), ctx.Root.GetService())
	if access != "" && !AddAccessAllows(access, rel, ctx.Viewer) {
		return false
	}

	return limitedTo == constants.ReplyAccessAnyone || chosenToReply(ctx, limitedTo)
}

// CanEditReply reports whether the viewer may change a reply: its author
// only, and only while they can still see the thread.
//
// Not a moderator of any kind. An edited reply still carries its author's
// name, so anybody else editing it puts words under that name. Whoever looks
// after a thread removes or hides replies; they do not rewrite them.
func CanEditReply(ctx *ReplyAccessContext) bool {
	if !ctx.rootLoaded() || ctx.Reply == nil {
		return false
	}

	if !ctx.Viewer.HasScope(constants.OAuthScopeMediaWrite) {
		return false
	}

	if ctx.Viewer.ID <= 0 {
		return false
	}

	// The website's own replies are changed by whoever replies as it.
	if ctx.Viewer.ID != ctx.Reply.GetAuthorID() && !isWebsiteReply(ctx) {
		return false
	}

	return CanSeeThread(ctx)
}

// RepliesAs is the profile a reply the viewer writes in this thread belongs
// to, when it is not the viewer: the website, in a thread under one of its own
// posts, for one of its administrators (docs/plans/website.md). The site
// answers its readers as itself. Nil anywhere else: a reply is its
// writer's.
func RepliesAs(ctx *ReplyAccessContext) *entities.AnahitaNode {
	if !ctx.rootLoaded() || ctx.Viewer.ID <= 0 {
		return nil
	}
	owner := ctx.Root.GetOwner()
	if owner.GetType() != constants.NodeActorWebsiteEntityType {
		return nil
	}
	if !relationOrEmpty(ctx.RootOwnerViewerRel).HasAdmin {
		return nil
	}
	return owner
}

// isWebsiteReply says whether the reply is the website's own, in its own
// thread, and the viewer one of those who reply as it.
func isWebsiteReply(ctx *ReplyAccessContext) bool {
	website := RepliesAs(ctx)
	return website != nil && ctx.Reply.GetAuthorID() == website.GetID()
}

// CanDeleteReply reports whether the viewer may remove a reply for good: its
// author, and whoever looks after the thread.
//
// No check that the author can still see the thread. Taking your own words
// down works even after you have lost sight of what they were said under.
func CanDeleteReply(ctx *ReplyAccessContext) bool {
	if ctx == nil || ctx.Viewer == nil || ctx.Reply == nil || ctx.Viewer.ID <= 0 {
		return false
	}

	if !ctx.Viewer.HasScope(constants.OAuthScopeMediaWrite) {
		return false
	}

	if ctx.Viewer.ID == ctx.Reply.GetAuthorID() {
		return true
	}

	return LooksAfterThread(ctx)
}

// CanHideReply reports whether the viewer may hide a reply from its thread,
// and show it again: whoever looks after the thread.
//
// Not the reply's own author, who has no use for it: they can remove it.
func CanHideReply(ctx *ReplyAccessContext) bool {
	if ctx == nil || ctx.Viewer == nil || ctx.Reply == nil {
		return false
	}

	if !ctx.Viewer.HasScope(constants.OAuthScopeMediaWrite) {
		return false
	}

	return LooksAfterThread(ctx)
}

// CanLikeReply reports whether the viewer may like a reply: the rule for
// liking the root.
func CanLikeReply(ctx *ReplyAccessContext) bool {
	if !ctx.rootLoaded() {
		return false
	}

	return CanLike(ctx.Viewer, ctx.Root, ctx.RootOwnerViewerRel)
}

// relationOrEmpty stands in for a missing relationship. A viewer with none to
// the owner is ordinary, and ActorPermissions.CanRead dereferences it.
func relationOrEmpty(rel *entities.ActorRelation) *entities.ActorRelation {
	if rel == nil {
		return &entities.ActorRelation{}
	}
	return rel
}
