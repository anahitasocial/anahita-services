package main

import (
	"net/http"

	"text/cmd/api/handlers"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
	libs_requests "anahita-libs/requests"
	"anahita-libs/responses"
)

// postDocs holds what is described the same way for articles, notes and
// topics. Each description is given to a route with With, so that a
// kind's own routes say only what is theirs. The ones that take `kind`
// name the kind of post in what they say.
type postDocs struct {
	// write is the scope every change to a post needs.
	write string

	// media is the requests every kind of post takes.
	media *libs_requests.MediumRequest

	browse, read             func(kind string) func(*apidoc.Route)
	edit, add                func(kind string, takes any) func(*apidoc.Route)
	remove                   func(kind string) func(*apidoc.Route)
	access, publish          func(*apidoc.Route)
	pin, save                func(what string) func(*apidoc.Route)
	commentStatus            func(*apidoc.Route)
	replyAccess, quotePolicy func(*apidoc.Route)
	warning                  func(*apidoc.Route)
}

// newPostDocs makes the descriptions postDocs holds.
func newPostDocs() postDocs {
	write := libs_constants.OAuthScopeMediaWrite
	media := libs_requests.NewMediumRequest()

	// The refusal of a setting changed on a reply, which has none of its
	// own.
	reply := "`reply_follows_its_thread`: a reply has no setting of its own; its thread decides"

	// The refusal of a new post that is not well formed.
	invalid := "`language_invalid`, `reply_access_invalid` or `quote_policy_invalid`; " +
		"or not well formed, without a body"

	// A profile's posts of one kind, and one of them.
	browse := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("A profile's "+kind+"s").
				Describe("Those the viewer may see.").
				Viewer(apidoc.Anyone).
				Takes(media.Browse).
				Sends(http.StatusOK, responses.List[*responses.MediumNodeItem]{}).
				Refuses(http.StatusForbidden, "the viewer may not see the profile")
		}
	}
	read := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("One "+kind).
				Describe("A reply is read by whoever may read the post at the top of its thread.").
				Viewer(apidoc.Anyone).
				Takes(media.Read).
				Sends(http.StatusOK, responses.MediumNode{}).
				Refuses(http.StatusForbidden, "the viewer may not see it").
				Refuses(http.StatusNotFound, "there is no such post").
				Refuses(http.StatusGone, "a reply that was removed")
		}
	}

	// Changing a post, posting one and deleting one. A note's routes add
	// to these what is a note's own.
	edit := func(kind string, takes any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Change a "+kind+"'s words").
				Viewer(apidoc.SignedIn).
				Scope(write).
				Takes(takes).
				Sends(http.StatusOK, responses.MediumNode{}).
				Sends(http.StatusBadRequest, responses.Refusal{}, "`language_invalid`; or not "+
					"well formed, without a body").
				Refuses(http.StatusForbidden, "the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such post")
		}
	}
	add := func(kind string, takes any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Post a "+kind).
				Viewer(apidoc.SignedIn).
				Scope(write).
				Takes(takes).
				Sends(http.StatusCreated, responses.MediumNode{}).
				Sends(http.StatusBadRequest, responses.Refusal{}, invalid).
				Refuses(http.StatusForbidden, "the viewer may not post here").
				Refuses(http.StatusNotFound, "there is no such profile").
				Sends(http.StatusUnprocessableEntity, responses.AudienceRefusal{},
					"`access_not_allowed`, with the audiences it may have")
		}
	}
	remove := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Delete a "+kind).
				Describe("A reply that others have answered is emptied and kept, so the thread "+
					"holds together.").
				Viewer(apidoc.SignedIn).
				Scope(write).
				Takes(media.Delete).
				SendsNothing(http.StatusOK, "it is gone").
				Refuses(http.StatusForbidden, "the viewer may not delete it").
				Refuses(http.StatusNotFound, "there is no such post")
		}
	}

	// One of a post's settings that a reply does not have, since its thread
	// decides: commentStatus, replyAccess and quotePolicy below. `bad`, when
	// given, is what a setting that is not well formed is refused with.
	setting := func(summary string, takes any, bad string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary(summary).
				Viewer(apidoc.SignedIn).
				Scope(write).
				Takes(takes).
				Sends(http.StatusOK, responses.MediumNode{}).
				Refuses(http.StatusForbidden, "the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such post").
				Sends(http.StatusUnprocessableEntity, responses.Refusal{}, reply)
			if bad != "" {
				route.Sends(http.StatusBadRequest, responses.Refusal{}, bad)
			}
		}
	}

	// Who may see a post. A reply has no audience of its own either.
	access := func(route *apidoc.Route) {
		route.Summary("Change who may see it").
			Viewer(apidoc.SignedIn).
			Scope(write).
			Takes(media.EditAccess).
			Sends(http.StatusOK, responses.MediumNode{}).
			Refuses(http.StatusForbidden, "the viewer may not change it").
			Refuses(http.StatusNotFound, "there is no such post").
			Sends(http.StatusUnprocessableEntity, responses.AudienceRefusal{},
				"`access_not_allowed`, with the audiences it may have; or "+reply+", without them")
	}

	// Publishing an article or a topic begun as a draft.
	publish := func(route *apidoc.Route) {
		route.Summary("Publish a draft").
			Describe("An article or a topic begun as a draft (`draft` when it was made) becomes a "+
				"post from now: dated now, seen by whoever its audience says, and announced as a "+
				"post made now. Its author's alone. `access` sets its audience; left out, the one "+
				"it was saved with.").
			Viewer(apidoc.SignedIn).
			Scope(write).
			Takes(&libs_requests.MediumPublishRequest{}).
			Sends(http.StatusOK, responses.MediumNode{}).
			Refuses(http.StatusForbidden, "the viewer may not publish it").
			Refuses(http.StatusNotFound, "no such draft of theirs").
			Refuses(http.StatusConflict, "it is published already")
	}

	// Pinning a post to its profile, and taking the pin off: `what` is the
	// summary, the rest is the same.
	pin := func(what string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary(what).
				Describe("A profile has one pin: pinning another post moves it. For the profile "+
					"and its administrators, and site administrators (permissions.md). Answers "+
					"with the post.").
				Viewer(apidoc.SignedIn).
				Scope(write).
				Takes(media.Read).
				Sends(http.StatusOK, responses.MediumNode{}).
				Refuses(http.StatusForbidden, "the viewer may not pin it").
				Refuses(http.StatusNotFound, "there is no such post").
				Sends(http.StatusUnprocessableEntity, responses.Refusal{},
					"`reply_cannot_be_pinned`")
		}
	}

	// The three settings made with setting.
	commentStatus := setting("Open or close replies", media.EditCommentStatus, "")
	replyAccess := setting("Change who may reply",
		&libs_requests.MediumEditReplyAccessRequest{},
		"`reply_access_invalid`, or not well formed")
	quotePolicy := setting("Change who may quote it",
		&libs_requests.MediumEditQuotePolicyRequest{},
		"`quote_policy_invalid`, or not well formed")

	// A post's content warning, given by whoever may edit it or by a
	// moderator.
	warning := func(route *apidoc.Route) {
		route.Summary("Give it a content warning, or take it off").
			Describe("Its kind, and for `other` its words: shown in place of the post until a "+
				"reader chooses to see it. An empty `warning` takes it off. Whoever may edit the "+
				"post may; a reply, its author alone. A site administrator who did not write it "+
				"gives or takes off a warning as a moderator, on any post, and its author is "+
				"told; its author may not change what a moderator gave.").
			Viewer(apidoc.SignedIn).
			Scope(write).
			Takes(&libs_requests.MediumEditWarningRequest{}).
			Sends(http.StatusOK, responses.MediumNode{}).
			Refuses(http.StatusForbidden, "the viewer may not change it").
			Sends(http.StatusForbidden, responses.Refusal{},
				"`warning_not_allowed`: a moderator gave it").
			Refuses(http.StatusNotFound, "there is no such post").
			Refuses(http.StatusGone, "a reply that was removed").
			Sends(http.StatusBadRequest, responses.Refusal{},
				"`warning_invalid`, or not well formed")
	}

	// Saving a post to the viewer's own list, and taking it off: `what` is
	// the summary, the rest is the same.
	save := func(what string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary(what).
				Describe("A post of any kind. Saving is the viewer's own: nobody is told.").
				Viewer(apidoc.SignedIn).
				Takes(media.Read).
				Sends(http.StatusOK, handlers.SavedAnswer{}).
				Refuses(http.StatusNotFound, "there is no such post that the viewer may see")
		}
	}

	return postDocs{
		write:         write,
		media:         media,
		browse:        browse,
		read:          read,
		edit:          edit,
		add:           add,
		remove:        remove,
		access:        access,
		publish:       publish,
		pin:           pin,
		save:          save,
		commentStatus: commentStatus,
		replyAccess:   replyAccess,
		quotePolicy:   quotePolicy,
		warning:       warning,
	}
}
