package permissions

import (
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

const quoteAuthorID = 2

// A public note by quoteAuthorID on their own public profile.
func quotable() *entities.AnahitaNode {
	post := postOn(ownerWith(quoteAuthorID, "public"), "public")
	post.CreatedBy = quoteAuthorID
	return post
}

func quoting(post *entities.AnahitaNode, viewer *entities.Viewer, rel *entities.ActorRelation, authorPolicy string) (bool, string) {
	return CanQuote(&QuoteAccessContext{
		Viewer:          viewer,
		Post:            post,
		AuthorPolicy:    authorPolicy,
		OwnerViewerRel:  rel,
		AuthorViewerRel: rel,
	})
}

func TestWhoMayQuote(t *testing.T) {
	var (
		stranger = &entities.ActorRelation{}
		follower = &entities.ActorRelation{IsLeading: true}
		followed = &entities.ActorRelation{IsFollowing: true}
		blocked  = &entities.ActorRelation{IsBlocking: true}
		somebody = replyViewer(bystanderID, constants.PersonTypeRegistered)
	)

	cases := []struct {
		name         string
		postPolicy   string
		authorPolicy string
		rel          *entities.ActorRelation
		want         bool
		reason       string
	}{
		{"nothing said, a stranger", "", "", stranger, true, ""},
		{"anyone, a stranger", "anyone", "", stranger, true, ""},
		{"nobody", "nobody", "", follower, false, QuoteRefusedPolicy},
		{"followers, a follower", "followers", "", follower, true, ""},
		{"followers, a stranger", "followers", "", stranger, false, QuoteRefusedPolicy},
		{"followers, somebody the author follows", "followers", "", followed, false, QuoteRefusedPolicy},

		// The author's own policy is for posts that say nothing.
		{"the author says nobody", "", "nobody", stranger, false, QuoteRefusedPolicy},
		{"the author says followers", "", "followers", follower, true, ""},
		{"the post says anyone over an author who says nobody", "anyone", "nobody", stranger, true, ""},
		{"the post says nobody over an author who says anyone", "nobody", "anyone", stranger, false, QuoteRefusedPolicy},

		// Blocked by the author, on the author's own profile: they cannot
		// see the post, and are told no more than that.
		{"blocked by the author", "anyone", "", blocked, false, QuoteRefusedUnavailable},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			post := quotable()
			post.QuotePolicy = c.postPolicy

			got, reason := quoting(post, somebody, c.rel, c.authorPolicy)
			if got != c.want || reason != c.reason {
				t.Errorf("CanQuote = %v, %q; want %v, %q", got, reason, c.want, c.reason)
			}
		})
	}
}

// The policy is about other people.
func TestAnAuthorQuotesTheirOwnPostWhateverItsPolicy(t *testing.T) {
	post := quotable()
	post.QuotePolicy = "nobody"

	author := replyViewer(quoteAuthorID, constants.PersonTypeRegistered)
	if ok, reason := quoting(post, author, &entities.ActorRelation{}, "nobody"); !ok {
		t.Errorf("refused: %s", reason)
	}
}

func TestWhatCannotBeQuotedAtAll(t *testing.T) {
	somebody := replyViewer(bystanderID, constants.PersonTypeRegistered)
	follower := &entities.ActorRelation{IsLeading: true}

	// Not public, though the viewer can see it.
	forFollowers := quotable()
	forFollowers.Access = "followers"
	if ok, reason := quoting(forFollowers, somebody, follower, ""); ok || reason != QuoteRefusedNotPublic {
		t.Errorf("a followers-only post: %v, %q", ok, reason)
	}

	// Public, on a profile that is not.
	onPrivateProfile := postOn(ownerWith(quoteAuthorID, "followers"), "public")
	onPrivateProfile.CreatedBy = quoteAuthorID
	if ok, reason := quoting(onPrivateProfile, somebody, follower, ""); ok || reason != QuoteRefusedNotPublic {
		t.Errorf("a post on a followers-only profile: %v, %q", ok, reason)
	}

	// One the viewer cannot see at all is not told apart from one that is
	// not there.
	if ok, reason := quoting(onPrivateProfile, somebody, &entities.ActorRelation{}, ""); ok || reason != QuoteRefusedUnavailable {
		t.Errorf("a post the viewer cannot see: %v, %q", ok, reason)
	}

	reply := quotable()
	reply.RootID = 99
	if ok, reason := quoting(reply, somebody, follower, ""); ok || reason != QuoteRefusedUnavailable {
		t.Errorf("a reply: %v, %q", ok, reason)
	}

	removed := quotable()
	when := time.Now()
	removed.DeletedAt = &when
	if ok, reason := quoting(removed, somebody, follower, ""); ok || reason != QuoteRefusedUnavailable {
		t.Errorf("a removed post: %v, %q", ok, reason)
	}

	repost := quotable()
	repost.Type = "node.base.repost-service.repost.v1"
	if ok, reason := quoting(repost, somebody, follower, ""); ok || reason != QuoteRefusedUnavailable {
		t.Errorf("a repost: %v, %q", ok, reason)
	}

	// A token that may not write posts, and nobody at all.
	likesOnly := replyViewer(bystanderID, constants.PersonTypeRegistered, constants.OAuthScopeLikesWrite)
	if ok, reason := quoting(quotable(), likesOnly, follower, ""); ok || reason != QuoteRefusedNotAllowed {
		t.Errorf("a token without media:write: %v, %q", ok, reason)
	}
	if ok, _ := quoting(quotable(), entities.NewViewer(), nil, ""); ok {
		t.Error("a visitor may quote")
	}
}

// What a list answers with: exact on the author's own profile, and on the
// careful side for followers elsewhere.
func TestCanQuotePostFromAList(t *testing.T) {
	somebody := replyViewer(bystanderID, constants.PersonTypeRegistered)
	follower := &entities.ActorRelation{IsLeading: true}

	own := quotable()
	own.GetOwner().QuotePolicy = "followers"
	if !CanQuotePost(somebody, own, follower) {
		t.Error("refused a follower of an author who lets followers")
	}
	if CanQuotePost(somebody, own, &entities.ActorRelation{}) {
		t.Error("let a stranger past an author who lets followers")
	}

	// Written by somebody else in a public group, for the author's
	// followers. Following the group is not following the author.
	inGroup := postOn(ownerWith(groupOwnerID, "public"), "public")
	inGroup.CreatedBy = quoteAuthorID
	inGroup.QuotePolicy = "followers"
	if CanQuotePost(somebody, inGroup, follower) {
		t.Error("took a follower of the group for a follower of the author")
	}
}

// A post in a public group, by somebody who has blocked the viewer. The
// viewer can see the group, and the post is still not theirs to quote.
func TestSomebodyBlockedByTheAuthorDoesNotQuoteThemInAGroup(t *testing.T) {
	inGroup := postOn(ownerWith(groupOwnerID, "public"), "public")
	inGroup.CreatedBy = quoteAuthorID

	ok, reason := CanQuote(&QuoteAccessContext{
		Viewer:          replyViewer(bystanderID, constants.PersonTypeRegistered),
		Post:            inGroup,
		OwnerViewerRel:  &entities.ActorRelation{IsLeading: true},
		AuthorViewerRel: &entities.ActorRelation{IsBlocking: true},
	})
	if ok || reason != QuoteRefusedBlocked {
		t.Errorf("CanQuote = %v, %q; want false, blocked", ok, reason)
	}
}

func TestWhoTakesAPostOutOfAQuote(t *testing.T) {
	quoted := quotable()

	if !CanDetachQuote(replyViewer(quoteAuthorID, constants.PersonTypeRegistered), quoted) {
		t.Error("the quoted post's author cannot take it out")
	}
	if !CanDetachQuote(replyViewer(bystanderID, constants.PersonTypeAdmin), quoted) {
		t.Error("a site administrator cannot take it out")
	}
	if CanDetachQuote(replyViewer(bystanderID, constants.PersonTypeRegistered), quoted) {
		t.Error("somebody else took a post out of a quote")
	}
	if CanDetachQuote(entities.NewViewer(), quoted) {
		t.Error("a visitor took a post out of a quote")
	}
}
