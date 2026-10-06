package permissions

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// The rule for quoting a post: writing a note of your own that carries it.
//
// A quote sits between a repost, which has no words of your own, and a
// reply, which stays in the other person's thread. It puts somebody's post
// in front of your audience under your words, so the person quoted keeps
// the say: who may quote is theirs to set, on a post and for all their
// posts, and they can take their post out of a quote afterwards.

// Why a quote was refused, for the app to say.
const (
	// Not signed in, or with a token that may not write posts.
	QuoteRefusedNotAllowed = "not_allowed"
	// Not something that can be quoted: a reply, a repost, something
	// removed, or a post the viewer cannot see.
	QuoteRefusedUnavailable = "unavailable"
	// Not public. Only a public post can be quoted.
	QuoteRefusedNotPublic = "not_public"
	// Its author has blocked the viewer.
	QuoteRefusedBlocked = "blocked"
	// Its author does not let the viewer quote it.
	QuoteRefusedPolicy = "policy"
)

// QuoteAccessContext is a post somebody wants to quote.
type QuoteAccessContext struct {
	Viewer *entities.Viewer
	// The post, with its Owner loaded as the actor read returns it.
	Post *entities.AnahitaNode
	// The post's author's own policy, for a post that has none. Empty
	// when they have not said, or when it is not known.
	AuthorPolicy string
	// The relationships of the post's owner and of its author to the
	// viewer. The same for a post on its author's own profile.
	OwnerViewerRel  *entities.ActorRelation
	AuthorViewerRel *entities.ActorRelation
}

// CanQuote reports whether the viewer may quote the post, and when not,
// why.
//
//   - Only a post at the top level, that is still there. A reply is part of
//     its thread and is seen by whoever sees the thread; it is not quoted.
//   - Only one the viewer can see.
//   - Only a public one, on a profile that is public or for members. A
//     quote of a post with a narrower audience would be a note most of its
//     readers could not see the point of. What is carried is still shown to
//     each reader only if they may see it, so this is not what keeps a
//     private post private.
//   - Not when its author has blocked the viewer.
//   - Then the policy: the post's, or its author's, or anyone. "Followers"
//     are the people who follow the author.
//
// Somebody may always quote their own post, whatever its policy: the policy
// is about other people.
func CanQuote(ctx *QuoteAccessContext) (bool, string) {
	if ctx == nil || ctx.Viewer == nil || !ctx.Viewer.Authenticated ||
		!ctx.Viewer.HasScope(constants.OAuthScopeMediaWrite) {
		return false, QuoteRefusedNotAllowed
	}

	post := ctx.Post
	if post == nil || post.GetID() <= 0 || !post.IsMedium() || post.RootID > 0 || post.DeletedAt != nil {
		return false, QuoteRefusedUnavailable
	}

	owner := post.GetOwner()
	if owner == nil || owner.GetID() <= 0 {
		return false, QuoteRefusedUnavailable
	}

	if ok, _ := NewMediumPermissions().CanRead(&MediumAccessContext{
		Viewer:         ctx.Viewer,
		Medium:         post,
		OwnerViewerRel: relationOrEmpty(ctx.OwnerViewerRel),
	}); !ok {
		return false, QuoteRefusedUnavailable
	}

	if post.GetAccess() != constants.AnahitaAccessPublic {
		return false, QuoteRefusedNotPublic
	}
	switch owner.GetAccess() {
	case constants.AnahitaAccessPublic, constants.AnahitaAccessRegistered:
	default:
		return false, QuoteRefusedNotPublic
	}

	if ctx.Viewer.ID == post.GetAuthorID() {
		return true, ""
	}

	authorRel := relationOrEmpty(ctx.AuthorViewerRel)
	if authorRel.IsBlocking {
		return false, QuoteRefusedBlocked
	}

	switch constants.EffectiveQuotePolicy(post.QuotePolicy, ctx.AuthorPolicy) {
	case constants.QuotePolicyAnyone:
		return true, ""
	case constants.QuotePolicyFollowers:
		// The relation is the author's to the viewer: "leading" is the
		// viewer following the author.
		if authorRel.IsLeading {
			return true, ""
		}
	}

	return false, QuoteRefusedPolicy
}

// CanQuotePost answers the same with what a list of posts has to hand: the
// post with its owner, and the owner's relationship to the viewer.
//
// A list does not read the author as well. For a post on its author's own
// profile the owner is the author, and the answer is exact. For one that
// somebody wrote elsewhere, in a group or on another profile, the author's
// own policy and relationship are not known here: a post that names no
// policy of its own is answered as if its author let anyone, and one for
// followers as no. Making the quote asks again, with everything.
func CanQuotePost(viewer *entities.Viewer, post *entities.AnahitaNode, rel *entities.ActorRelation) bool {
	if viewer == nil || post == nil {
		return false
	}

	ctx := &QuoteAccessContext{
		Viewer:         viewer,
		Post:           post,
		OwnerViewerRel: rel,
	}

	if owner := post.GetOwner(); owner != nil && post.GetAuthorID() == post.GetOwnerID() {
		ctx.AuthorPolicy = owner.QuotePolicy
		ctx.AuthorViewerRel = rel
	}

	ok, _ := CanQuote(ctx)
	return ok
}

// CanDetachQuote reports whether the viewer may take a quoted post out of a
// note that quotes it: the post's author, and site administrators.
//
// Not the author of the note, who has no use for it: they can edit or
// remove their note.
func CanDetachQuote(viewer *entities.Viewer, quoted *entities.AnahitaNode) bool {
	if viewer == nil || quoted == nil || viewer.ID <= 0 {
		return false
	}

	if !viewer.HasScope(constants.OAuthScopeMediaWrite) {
		return false
	}

	return viewer.IsAdmin() || viewer.ID == quoted.GetAuthorID()
}
