package main

import (
	"net/http"

	"actor/cmd/api/handlers"
	"actor/responses"

	"anahita-libs/apidoc"
	"anahita-libs/requests"
	libs_responses "anahita-libs/responses"
)

// --- People ---
// peopleRoutes registers the routes at /people/: somebody with an account,
// at their username. Listing people, reading one, and who is tagged in a
// photo or a video are public: no token is needed, and when one is sent the
// caller is known (r.setViewer). Every other route needs a token
// (r.authorize), and whether the viewer may is decided in the handler.
func (r *Routes) peopleRoutes(api *apidoc.Router, s *shared) {
	people := api.In("people").Group("people")

	people.GET("/", r.handler.Person.Browse, r.setViewer).
		Summary("People").
		Describe("Those the viewer may see. "+s.someOrLimited).
		Viewer(apidoc.Anyone).
		Takes(s.personReq.Browse).
		Sends(http.StatusOK, libs_responses.List[any]{}).
		Refuses(http.StatusForbidden, "the viewer may not list people")

	people.GET("/:username", r.handler.Person.Read, r.setViewer).
		Summary("One person").
		Describe("By their username. To somebody they are not open to, but who may ask to follow "+
			"them, what a stranger is shown of them.").
		Viewer(apidoc.Anyone).
		Takes(s.personReq.Read).
		Sends(http.StatusOK, responses.PersonNode{},
			"them, or what a stranger is shown of them (LimitedActor)").
		Refuses(http.StatusForbidden, "the viewer may not see them").
		Refuses(http.StatusNotFound, "there is no such person")

	people.PATCH("/:id", r.handler.Person.Edit, r.authorize).
		Summary("Change a person's profile").
		Viewer(apidoc.SignedIn).
		Takes(s.personReq.Edit).
		Sends(http.StatusOK, responses.PersonNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such person")

	// --- Deleting, purging and archiving a person ---
	// Every route needs a token (r.authorize). What deleting would take with
	// it, for the Danger zone, has the same permission as deleting: see the
	// handler. Purging is for super administrators, removing as spam for
	// administrators, and archiving for the person or a super
	// administrator; the handlers check.
	//
	// The delete is at :id, not :username. The client sends node.id here
	// (api/create.js deleteItem builds /people/{id}), so this route matched
	// the username against a number and answered 404 to every delete — the
	// flow could never have run. PATCH above already keys on the id, and a
	// group's delete keys on the id too, so this is the consistent shape as
	// well as the working one.
	people.DELETE("/:id", r.handler.Person.Delete, r.authorize).
		With(s.remove("person", s.personReq.Delete))

	people.GET("/:id/deletion-counts", r.handler.Person.DeletionCounts, r.authorize).
		With(s.counts("person", handlers.PersonDeletionCounts{}))

	people.DELETE("/:id/purge", r.handler.Person.Purge, r.authorize).With(s.purge("person"))

	people.POST("/:id/spam", r.handler.Person.DeleteAsSpam, r.authorize).With(s.spam("person"))

	people.POST("/purge-batch", r.handler.Person.PurgeBatch, r.authorize).With(s.purgeBatch("person"))

	people.GET("/purge-batch/:batch_id", r.handler.Person.PurgeProgress, r.authorize).
		With(s.purgeProgress)

	people.POST("/:id/archive", r.handler.Person.Archive, r.authorize).With(s.archive("person"))

	// --- Turning a person off, and featuring them ---
	// For administrators (enabled) and super administrators (featured);
	// the handlers check. Every route needs a token (r.authorize).
	people.PATCH("/:id/enabled", r.handler.Person.SetEnabled, r.authorize).
		With(s.enabled).
		Sends(http.StatusConflict, libs_responses.Refusal{}, "`last_super_administrator`")

	people.PATCH("/:id/featured", r.handler.Person.SetFeatured, r.authorize).With(s.featured)

	// --- A person's settings ---
	// Who may see them and their followers, what their profile shows, who
	// may quote them, their choices for content warnings (roadmap row 122,
	// phase 2), which are theirs alone, and what their new photos and videos
	// start with about being downloaded. Every route needs a token
	// (r.authorize).
	people.PATCH("/:id/access", r.handler.Person.EditAccess, r.authorize).
		With(s.access(s.personReq.EditAccess, responses.PersonNode{}))

	people.PATCH("/:id/graph-access", r.handler.Person.EditGraphAccess, r.authorize).
		With(s.graphAccess(responses.PersonNode{}))

	people.PATCH("/:id/reposts-on-profile", r.handler.Person.EditRepostsOnProfile, r.authorize).
		Summary("Show reposts among a person's posts, or not").
		Describe("They are always in the Reposts tab of the profile.").
		Viewer(apidoc.SignedIn).
		Takes(&requests.ActorEditRepostsOnProfileRequest{}).
		Sends(http.StatusOK, responses.PersonNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such person")

	people.PATCH("/:id/quote-policy", r.handler.Person.EditQuotePolicy, r.authorize).
		Summary("Change who may quote a person's posts").
		Describe("For the posts that do not say.").
		Viewer(apidoc.SignedIn).
		Takes(&requests.PersonEditQuotePolicyRequest{}).
		Sends(http.StatusOK, responses.PersonNode{}).
		Sends(http.StatusBadRequest, libs_responses.Refusal{},
			"`quote_policy_invalid`, or not well formed").
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such person")

	people.GET("/:id/content-preferences", r.handler.Person.ReadContentPreferences, r.authorize).
		Summary("A person's choices for content warnings").
		Describe("Show, Warn or Hide for each kind: whether a post of that kind is shown at once, "+
			"behind its warning, or as one line that opens it. The viewer's own alone.").
		Viewer(apidoc.SignedIn).
		Takes(&requests.PersonContentPreferencesRequest{}).
		Sends(http.StatusOK, responses.ContentPreferences{}).
		Refuses(http.StatusForbidden, "they are not the viewer's own")

	people.PATCH("/:id/content-preferences", r.handler.Person.EditContentPreferences, r.authorize).
		Summary("Change a person's choices for content warnings").
		Describe("The kinds named change; the others stay as they were.").
		Viewer(apidoc.SignedIn).
		Takes(&requests.PersonEditContentPreferencesRequest{}).
		Sends(http.StatusOK, responses.ContentPreferences{}).
		Sends(http.StatusBadRequest, libs_responses.Refusal{},
			"`content_preference_invalid`, or not well formed").
		Refuses(http.StatusForbidden, "they are not the viewer's own")

	people.PATCH("/:id/download-default", r.handler.Person.EditDownloadDefault, r.authorize).
		With(s.downloads(responses.PersonNode{}))

	// --- Tags ---
	// People in a picture or a video (roadmap row 103): tagged by the post's
	// author, approved by the person unless they chose otherwise. Who is
	// tagged is public: no token is needed, and when one is sent the caller
	// is known (r.setViewer). Tagging, approving and taking a tag off need a
	// token (r.authorize).
	people.GET("/tagged-in/:post_id", r.handler.Person.TaggedIn, r.setViewer).
		Summary("Who is in a photo or a video").
		Describe("In the order they were tagged. A tag waiting for the person's approval is "+
			"listed only for them and the post's author, with `pending`.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.TaggedInPath{}).
		Sends(http.StatusOK, libs_responses.Unpaged[handlers.TaggedPerson]{}).
		Refuses(http.StatusNotFound, "there is no such photo or video the viewer can see")

	people.POST("/:id/tags/:post_id", r.handler.Person.AddTag, r.authorize).
		Summary("Tag somebody in a photo or a video").
		Describe("For the post's author. Only a person here may be tagged, and as they chose: at "+
			"once, each approved first (the default), or not at all. Anybody may tag themselves. "+
			"At most 20 people in one post.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.TagPath{}).
		SendsNothing(http.StatusCreated, "tagged").
		SendsNothing(http.StatusAccepted, "tagged, waiting for their approval").
		Refuses(http.StatusForbidden, "the viewer did not write the post").
		Refuses(http.StatusNotFound,
			"no such person, or no such photo or video the viewer can see").
		Sends(http.StatusConflict, libs_responses.Refusal{}, "`not_taggable`, they do not allow "+
			"it or are not here; `too_many`, the post has as many people as it may")

	people.PUT("/:id/tags/:post_id", r.handler.Person.ApproveTag, r.authorize).
		Summary("Approve being tagged").
		Describe("For the person tagged.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.TagPath{}).
		SendsNothing(http.StatusOK, "approved, or it was already").
		Refuses(http.StatusForbidden, "the viewer is not the person tagged").
		Refuses(http.StatusNotFound, "no such tag, or no such photo or video the viewer can see")

	people.DELETE("/:id/tags/:post_id", r.handler.Person.DeleteTag, r.authorize).
		Summary("Take somebody out of a photo or a video").
		Describe("For the person, declining or changing their mind, and for the post's author.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.TagPath{}).
		SendsNothing(http.StatusNoContent, "removed, or there was nothing to remove").
		Refuses(http.StatusForbidden, "the viewer is neither the person nor the post's author").
		Refuses(http.StatusNotFound, "there is no such photo or video the viewer can see")

	// --- What may be posted on a person's profile ---
	// Needs a token (r.authorize).
	people.PATCH("/:id/features", r.handler.Person.EditFeatures, r.authorize).
		With(s.features(s.personReq.EditFeatures, responses.PersonNode{}))
}
