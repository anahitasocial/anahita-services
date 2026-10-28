package main

import (
	"net/http"

	"actor/cmd/api/handlers"
	"actor/responses"

	"anahita-libs/apidoc"
	libs_handlers "anahita-libs/handlers"
	"anahita-libs/requests"
	libs_responses "anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	authorize echo.MiddlewareFunc
	setViewer echo.MiddlewareFunc
}

func NewRoutes(
	handler *handlers.Handlers,
	authorize echo.MiddlewareFunc,
	setViewer echo.MiddlewareFunc,
) *Routes {
	return &Routes{
		handler:   handler,
		authorize: authorize,
		setViewer: setViewer,
	}
}

// Init registers the routes, each with its description, and answers with
// the descriptions: docs/api/actor-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("actor-service", "Profiles",
		"People, groups and events: reading and changing them, who may see them and what may be posted on them, their administrators, and deleting, archiving and purging them (account-lifecycle.md).")
	api := doc.On(e)
	actorReq := requests.NewActorRequest()
	personReq := requests.NewPersonRequest()
	adminsReq := requests.NewActorAdminsRequest()
	stepUp := "`step_up_required`: the viewer must prove who they are again first; or the viewer may not, without a body"
	someOrLimited := "Each a full profile, or, for one the viewer may only ask to follow, what a stranger is shown of it (`restricted`)."

	// What each kind of profile has alike. `kind` is "person", "group" or
	// "event".
	changed := func(summary, describe string, takes any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary(summary).
				Describe(describe).
				Viewer(apidoc.SignedIn).
				Takes(takes).
				SendsNothing(http.StatusOK, "done").
				SendsNothing(http.StatusNoContent, "it was so already").
				Refuses(http.StatusForbidden, "the viewer may not").
				Refuses(http.StatusNotFound, "there is no such profile")
		}
	}
	remove := func(kind string, takes any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Delete a "+kind).
				Describe("It is out of sight at once and destroyed after thirty days, unless it is restored before then.").
				Viewer(apidoc.SignedIn).
				StepUp().
				Takes(takes).
				SendsNothing(http.StatusOK, "it will be deleted").
				Sends(http.StatusForbidden, libs_responses.Refusal{}, stepUp).
				Refuses(http.StatusNotFound, "there is no such profile")
		}
	}
	counts := func(kind string, sends any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("What deleting a "+kind+" would take with it").
				Describe("For the Danger zone, for whoever may delete it.").
				Viewer(apidoc.SignedIn).
				Takes(actorReq.Delete).
				Sends(http.StatusOK, sends).
				Refuses(http.StatusForbidden, "the viewer may not delete it").
				Refuses(http.StatusNotFound, "there is no such profile")
		}
	}
	purge := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Destroy a "+kind+" for good").
				Describe("Everything of it, at once and with no thirty days. Super administrators only, never on their own profile. Queued, and done by the sweeper.").
				Viewer(apidoc.SuperAdmin).
				StepUp().
				Takes(actorReq.Delete).
				Sends(http.StatusAccepted, libs_handlers.PurgeQueued{}, "queued").
				Sends(http.StatusOK, libs_handlers.PurgeQueued{}, "purged or queued already").
				Sends(http.StatusForbidden, libs_responses.Refusal{}, stepUp).
				Sends(http.StatusConflict, libs_handlers.PurgeRefused{}, "it cannot be purged; nothing was queued")
		}
	}
	// A group's and an event's rules (roadmap row 102, part 5).
	readRules := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("The rules of a "+kind).
				Describe("What somebody agrees to, each ticked, before following it or answering it: at most ten, in order, with the version they are at and the version the viewer last agreed to. For whoever may see it, and for whoever it would let ask to follow.").
				Viewer(apidoc.Anyone).
				Takes(actorReq.Read).
				Sends(http.StatusOK, handlers.Rules{}).
				Refuses(http.StatusForbidden, "the viewer may not see it").
				Refuses(http.StatusNotFound, "there is no such "+kind)
		}
	}
	setRules := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Set the rules of a "+kind).
				Describe("The rules as they are to be from now on, in order, at most ten; an empty list takes them away. Every change raises the version, so that whoever agreed before is asked again. For whoever may change it.").
				Viewer(apidoc.SignedIn).
				Takes(&handlers.RulesSetRequest{}).
				Sends(http.StatusOK, handlers.Rules{}).
				Sends(http.StatusBadRequest, libs_responses.Refusal{}, "`rules_invalid`: more than ten, or one too long").
				Refuses(http.StatusForbidden, "the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such "+kind)
		}
	}
	agreeRules := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Agree to the rules of a "+kind).
				Describe("The viewer agreeing, each rule ticked, to the version they were shown.").
				Viewer(apidoc.SignedIn).
				Takes(&handlers.RulesAgreeRequest{}).
				Sends(http.StatusNoContent, nil).
				Sends(http.StatusConflict, libs_responses.Refusal{}, "`rules_not_current`: the rules changed since; read them again").
				Refuses(http.StatusNotFound, "there is no such "+kind)
		}
	}
	spam := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Remove a "+kind+" as spam").
				Describe("A moderator's decision (roadmap row 102): everything of it destroyed at once, with no thirty days, and recorded as `removed-as-spam` with who decided. Administrators and super administrators; never an administrator's or a super administrator's account, nor one's own. Queued, and done by the sweeper.").
				Viewer(apidoc.Admin).
				Takes(actorReq.Delete).
				Sends(http.StatusAccepted, libs_handlers.PurgeQueued{}, "queued").
				Sends(http.StatusOK, libs_handlers.PurgeQueued{}, "purged or queued already").
				Refuses(http.StatusForbidden, "the viewer is not an administrator").
				Sends(http.StatusConflict, libs_handlers.PurgeRefused{}, "it cannot be removed (`administrator`, `yourself`, `not_found`, `wrong_type`); nothing was queued")
		}
	}
	purgeBatch := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Destroy many "+kind+"s for good").
				Describe("As purging one, for up to 500 at once.").
				Viewer(apidoc.SuperAdmin).
				StepUp().
				Takes(&requests.ActorPurgeBatchRequest{}).
				Sends(http.StatusAccepted, libs_handlers.PurgeQueued{}, "queued").
				Sends(http.StatusOK, libs_handlers.PurgeQueued{}, "purged or queued already").
				Sends(http.StatusBadRequest, libs_responses.Refusal{}, "`confirmation_mismatch`: `confirm` does not name how many there are; or not well formed, without a body").
				Sends(http.StatusForbidden, libs_responses.Refusal{}, stepUp).
				Sends(http.StatusConflict, libs_handlers.PurgeRefused{}, "some cannot be purged; nothing was queued")
		}
	}
	purgeProgress := func(route *apidoc.Route) {
		route.Summary("How far a purge has got").
			Describe("Asked every few seconds. Changes nothing, and needs no proof.").
			Viewer(apidoc.SuperAdmin).
			Takes(&requests.ActorPurgeBatchReadRequest{}).
			Sends(http.StatusOK, libs_handlers.PurgeProgress{}).
			Refuses(http.StatusForbidden, "the viewer is not a super administrator").
			Refuses(http.StatusNotFound, "there is no such purge")
	}
	archive := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Archive a "+kind).
				Describe("For good, and nothing is destroyed (account-lifecycle.md).").
				Viewer(apidoc.SignedIn).
				StepUp().
				Takes(actorReq.Delete).
				SendsNothing(http.StatusOK, "archived").
				SendsNothing(http.StatusNoContent, "archived already, or being deleted").
				Sends(http.StatusForbidden, libs_responses.Refusal{}, stepUp).
				Refuses(http.StatusNotFound, "there is no such profile")
		}
	}
	enabled := changed("Turn a profile on or off", "Reversible, site administrators only. Off, it is out of sight and may not act; nothing is deleted.", &handlers.EnabledRequest{})
	featured := func(route *apidoc.Route) {
		route.With(changed("Feature a profile, or stop", "What onboarding offers new members to follow. Super administrators only.", &handlers.FeaturedRequest{})).
			Sends(http.StatusConflict, libs_responses.Refusal{}, "`not_featurable`: it is disabled, archived or deleted")
	}
	restore := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Restore a deleted "+kind).
				Describe("Within the thirty days. Answers with it.").
				Viewer(apidoc.SignedIn).
				Takes(actorReq.Delete).
				Sends(http.StatusOK, libs_responses.ActorNode{}).
				Refuses(http.StatusForbidden, "the viewer may not").
				Refuses(http.StatusNotFound, "there is no such profile to restore")
		}
	}
	access := func(takes any, sends any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Change who may see it").
				Describe("And whether people it is not open to may ask to follow it.").
				Viewer(apidoc.SignedIn).
				Takes(takes).
				Sends(http.StatusOK, sends).
				Refuses(http.StatusForbidden, "the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such profile")
		}
	}
	features := func(takes any, sends any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Change what may be posted on it").
				Describe("Each kind of post, on or off, and who may add it (permissions.md).").
				Viewer(apidoc.SignedIn).
				Takes(takes).
				Sends(http.StatusOK, sends).
				Refuses(http.StatusForbidden, "the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such profile")
		}
	}
	downloads := func(sends any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Change whether new photos and videos may be downloaded").
				Describe("What a post starts with; each post can say otherwise.").
				Viewer(apidoc.SignedIn).
				Takes(&requests.ActorEditDownloadDefaultRequest{}).
				Sends(http.StatusOK, sends).
				Refuses(http.StatusForbidden, "the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such profile")
		}
	}
	admins := func(kind string) (func(*apidoc.Route), func(*apidoc.Route), func(*apidoc.Route)) {
		return func(route *apidoc.Route) {
				route.Summary("A "+kind+"'s administrators").
					Viewer(apidoc.SignedIn).
					Takes(adminsReq.Browse).
					Sends(http.StatusOK, libs_responses.Unpaged[*libs_responses.ActorNodeItem]{}).
					Refuses(http.StatusForbidden, "the viewer may not see them").
					Refuses(http.StatusNotFound, "there is no such "+kind)
			}, func(route *apidoc.Route) {
				route.Summary("Make somebody an administrator of a "+kind).
					Describe("By one of its administrators. Answers with them.").
					Viewer(apidoc.SignedIn).
					Takes(adminsReq.Add).
					Sends(http.StatusCreated, libs_responses.ActorNodeItem{}).
					Refuses(http.StatusForbidden, "the viewer may not").
					Refuses(http.StatusNotFound, "there is no such "+kind+" or person")
			}, func(route *apidoc.Route) {
				route.Summary("Remove an administrator of a "+kind).
					Viewer(apidoc.SignedIn).
					Takes(adminsReq.Delete).
					SendsNothing(http.StatusNoContent, "removed").
					Refuses(http.StatusForbidden, "the viewer may not").
					Refuses(http.StatusNotFound, "there is no such "+kind+" or administrator")
			}
	}
	readGroup := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("One "+kind).
				Describe("To somebody it is not open to, but who may ask to follow it, what a stranger is shown of it.").
				Viewer(apidoc.Anyone).
				Takes(actorReq.Read).
				Sends(http.StatusOK, libs_responses.ActorNode{}, "it, or what a stranger is shown of it (LimitedActor)").
				Refuses(http.StatusForbidden, "the viewer may not see it").
				Refuses(http.StatusNotFound, "there is no such "+kind)
		}
	}
	groupAdmins, addGroupAdmin, removeGroupAdmin := admins("group")
	eventAdmins, addEventAdmin, removeEventAdmin := admins("event")
	websiteAdmins, addWebsiteAdmin, removeWebsiteAdmin := admins("Website")

	// Each kind of actor keeps its own address: /people/ and /groups/. The
	// gateway sends both here.

	// people — Browse and Read are public but still identify the caller
	// when a token is present, so the viewer drives access filtering.
	// Everything else requires a valid OAuth access token.
	// Which profile an alias is, of any kind: see handlers/handle.go.
	api.GET("/actors/:alias", r.handler.Person.Handle, r.setViewer).
		Summary("What profile an alias is").
		Describe("Of any kind: what its own address begins with, and its id. The web app finds `/@ana` by it.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.HandleRequest{}).
		Unbound().
		Sends(http.StatusOK, handlers.HandleResponse{}).
		Refuses(http.StatusNotFound, "there is no such profile that the viewer may see")

	people := api.Group("people")
	people.GET("/", r.handler.Person.Browse, r.setViewer).
		Summary("People").
		Describe("Those the viewer may see. "+someOrLimited).
		Viewer(apidoc.Anyone).
		Takes(personReq.Browse).
		Sends(http.StatusOK, libs_responses.List[any]{}).
		Refuses(http.StatusForbidden, "the viewer may not list people")
	people.GET("/:username", r.handler.Person.Read, r.setViewer).
		Summary("One person").
		Describe("By their username. To somebody they are not open to, but who may ask to follow them, what a stranger is shown of them.").
		Viewer(apidoc.Anyone).
		Takes(personReq.Read).
		Sends(http.StatusOK, responses.PersonNode{}, "them, or what a stranger is shown of them (LimitedActor)").
		Refuses(http.StatusForbidden, "the viewer may not see them").
		Refuses(http.StatusNotFound, "there is no such person")
	people.PATCH("/:id", r.handler.Person.Edit, r.authorize).
		Summary("Change a person's profile").
		Viewer(apidoc.SignedIn).
		Takes(personReq.Edit).
		Sends(http.StatusOK, responses.PersonNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such person")
	// :id, not :username. The client sends node.id here (api/create.js
	// deleteItem builds /people/{id}), so this route matched the username
	// against a number and answered 404 to every delete — the flow could never
	// have run. PATCH above already keys on the id, and a group's delete keys
	// on the id too, so this is the consistent shape as well as the working
	// one.
	people.DELETE("/:id", r.handler.Person.Delete, r.authorize).With(remove("person", personReq.Delete))
	// What the deletion would cost, for the Danger zone. Same permission as
	// the deletion itself — see the handler.
	people.GET("/:id/deletion-counts", r.handler.Person.DeletionCounts, r.authorize).With(counts("person", handlers.PersonDeletionCounts{}))
	// Hard delete. A separate route from DELETE /people/:id because the
	// difference between recoverable and gone should not be a parameter.
	// Super administrators only — enforced in the handler.
	people.DELETE("/:id/purge", r.handler.Person.Purge, r.authorize).With(purge("person"))
	people.POST("/:id/spam", r.handler.Person.RemoveAsSpam, r.authorize).With(spam("person"))
	// The same for many at once, and how far a request has got.
	people.POST("/purge-batch", r.handler.Person.PurgeBatch, r.authorize).With(purgeBatch("person"))
	people.GET("/purge-batch/:batch_id", r.handler.Person.PurgeProgress, r.authorize).With(purgeProgress)
	// Archive: permanent, destroys nothing. Self or a super administrator.
	people.POST("/:id/archive", r.handler.Person.Archive, r.authorize).With(archive("person"))
	// Enable/disable: reversible, administrators only.
	people.PATCH("/:id/enabled", r.handler.Person.SetEnabled, r.authorize).
		With(enabled).
		Sends(http.StatusConflict, libs_responses.Refusal{}, "`last_super_administrator`")
	// Featuring: what onboarding offers to follow. Super administrators only.
	people.PATCH("/:id/featured", r.handler.Person.SetFeatured, r.authorize).With(featured)

	// Edit access
	people.PATCH("/:id/access", r.handler.Person.EditAccess, r.authorize).With(access(personReq.EditAccess, responses.PersonNode{}))
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
		Sends(http.StatusBadRequest, libs_responses.Refusal{}, "`quote_policy_invalid`, or not well formed").
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such person")
	// What their new photos and videos start with, about being downloaded.
	people.PATCH("/:id/download-default", r.handler.Person.EditDownloadDefault, r.authorize).With(downloads(responses.PersonNode{}))

	// People in a picture or a video (roadmap row 103): tagged by the post's
	// author, approved by the person unless they chose otherwise.
	people.GET("/tagged-in/:post_id", r.handler.Person.TaggedIn, r.setViewer).
		Summary("Who is in a photo or a video").
		Describe("In the order they were tagged. A tag waiting for the person's approval is listed only for them and the post's author, with `pending`.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.TaggedInPath{}).
		Sends(http.StatusOK, libs_responses.Unpaged[handlers.TaggedPerson]{}).
		Refuses(http.StatusNotFound, "there is no such photo or video the viewer can see")
	people.POST("/:id/tags/:post_id", r.handler.Person.AddTag, r.authorize).
		Summary("Tag somebody in a photo or a video").
		Describe("For the post's author. Only a person here may be tagged, and as they chose: at once, each approved first (the default), or not at all. Anybody may tag themselves. At most 20 people in one post.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.TagPath{}).
		SendsNothing(http.StatusCreated, "tagged").
		SendsNothing(http.StatusAccepted, "tagged, waiting for their approval").
		Refuses(http.StatusForbidden, "the viewer did not write the post").
		Refuses(http.StatusNotFound, "no such person, or no such photo or video the viewer can see").
		Sends(http.StatusConflict, libs_responses.Refusal{}, "`not_taggable`, they do not allow it or are not here; `too_many`, the post has as many people as it may")
	people.PUT("/:id/tags/:post_id", r.handler.Person.ApproveTag, r.authorize).
		Summary("Approve being tagged").
		Describe("For the person tagged.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.TagPath{}).
		SendsNothing(http.StatusOK, "approved, or it was already").
		Refuses(http.StatusForbidden, "the viewer is not the person tagged").
		Refuses(http.StatusNotFound, "no such tag, or no such photo or video the viewer can see")
	people.DELETE("/:id/tags/:post_id", r.handler.Person.RemoveTag, r.authorize).
		Summary("Take somebody out of a photo or a video").
		Describe("For the person, declining or changing their mind, and for the post's author.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.TagPath{}).
		SendsNothing(http.StatusNoContent, "removed, or there was nothing to remove").
		Refuses(http.StatusForbidden, "the viewer is neither the person nor the post's author").
		Refuses(http.StatusNotFound, "there is no such photo or video the viewer can see")

	// Edit features
	people.PATCH("/:id/features", r.handler.Person.EditFeatures, r.authorize).With(features(personReq.EditFeatures, responses.PersonNode{}))

	// groups — Browse and Read are public but still identify the caller
	// when a token is present, so the viewer drives access filtering.
	// Everything else requires a valid OAuth access token.
	groups := api.Group("groups")
	groups.GET("/", r.handler.Group.Browse, r.setViewer).
		Summary("Groups").
		Describe("Those the viewer may see. "+someOrLimited).
		Viewer(apidoc.Anyone).
		Takes(actorReq.Browse).
		Sends(http.StatusOK, libs_responses.List[any]{}).
		Refuses(http.StatusForbidden, "the viewer may not list groups")
	groups.GET("/:id", r.handler.Group.Read, r.setViewer).With(readGroup("group"))
	groups.POST("/", r.handler.Group.Add, r.authorize).
		Summary("Make a group").
		Describe("Whoever makes it is its first administrator.").
		Viewer(apidoc.SignedIn).
		Takes(actorReq.Add).
		Sends(http.StatusCreated, libs_responses.ActorNode{})
	groups.PATCH("/:id", r.handler.Group.Edit, r.authorize).
		Summary("Change a group").
		Viewer(apidoc.SignedIn).
		Takes(actorReq.Edit).
		Sends(http.StatusOK, libs_responses.ActorNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such group")
	groups.DELETE("/:id", r.handler.Group.Delete, r.authorize).With(remove("group", actorReq.Delete))
	// What the deletion would cost, for the Danger zone. Same permission as
	// the deletion itself — see the handler.
	groups.GET("/:id/deletion-counts", r.handler.Group.DeletionCounts, r.authorize).With(counts("group", handlers.GroupDeletionCounts{}))
	// Hard delete. Super administrators only — enforced in the handler.
	groups.DELETE("/:id/purge", r.handler.Group.Purge, r.authorize).With(purge("group"))
	groups.POST("/:id/spam", r.handler.Group.RemoveAsSpam, r.authorize).With(spam("group"))
	groups.POST("/purge-batch", r.handler.Group.PurgeBatch, r.authorize).With(purgeBatch("group"))
	groups.GET("/purge-batch/:batch_id", r.handler.Group.PurgeProgress, r.authorize).With(purgeProgress)
	// Archive: permanent, destroys nothing. A group administrator may.
	groups.POST("/:id/archive", r.handler.Group.Archive, r.authorize).With(archive("group"))
	// Enable/disable: reversible, site administrators only.
	groups.PATCH("/:id/enabled", r.handler.Group.SetEnabled, r.authorize).With(enabled)
	// Featuring: what onboarding offers to follow. Super administrators only.
	groups.PATCH("/:id/featured", r.handler.Group.SetFeatured, r.authorize).With(featured)
	// Undo, within the grace period. Groups cannot sign in, so this is their
	// equivalent of the person restore-at-login path.
	groups.POST("/:id/restore", r.handler.Group.Restore, r.authorize).With(restore("group"))

	// group edit access
	groups.PATCH("/:id/access", r.handler.Group.EditAccess, r.authorize).With(access(actorReq.EditAccess, libs_responses.ActorNode{}))
	groups.PATCH("/:id/download-default", r.handler.Group.EditDownloadDefault, r.authorize).With(downloads(libs_responses.ActorNode{}))

	// group edit features
	groups.PATCH("/:id/features", r.handler.Group.EditFeatures, r.authorize).With(features(actorReq.EditFeatures, libs_responses.ActorNode{}))
	groups.GET("/:id/rules", r.handler.Group.ReadRules, r.setViewer).With(readRules("group"))
	groups.PUT("/:id/rules", r.handler.Group.SetRules, r.authorize).With(setRules("group"))
	groups.POST("/:id/rules/agree", r.handler.Group.AgreeRules, r.authorize).With(agreeRules("group"))

	// The events a group hosts.
	groups.GET("/:id/events/", r.handler.Event.HostedBy, r.setViewer).
		Summary("The events a group hosts").
		Describe("Those the viewer may see. Each an event, or what is shown of one the viewer may only answer as a follower of the group.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.EventHostedRequest{Limit: 20}).
		Sends(http.StatusOK, libs_responses.List[any]{}).
		Refuses(http.StatusForbidden, "the viewer may not see the group").
		Refuses(http.StatusNotFound, "there is no such group")

	groups.GET("/:id/admins/", r.handler.Admin.Browse, r.authorize).With(groupAdmins)
	groups.POST("/:id/admins/", r.handler.Admin.Add, r.authorize).With(addGroupAdmin)
	groups.DELETE("/:id/admins/:admin_id", r.handler.Admin.Delete, r.authorize).With(removeGroupAdmin)

	// Websites — a site's feed as a profile, read and looked after as a
	// group is. Made by syndication-service once its domain is proved
	// (POST /websites/_connect), so there is no POST /websites/ here; nobody
	// posts on one, so it has no features to choose and no audience to
	// change: it is public.
	websites := api.Group("websites")
	websites.GET("/", r.handler.Website.Browse, r.setViewer).
		Summary("Websites").
		Describe("Those the viewer may see. "+someOrLimited).
		Viewer(apidoc.Anyone).
		Takes(actorReq.Browse).
		Sends(http.StatusOK, libs_responses.List[any]{}).
		Refuses(http.StatusForbidden, "the viewer may not list Websites")
	websites.GET("/:id", r.handler.Website.Read, r.setViewer).With(readGroup("Website"))
	websites.PATCH("/:id", r.handler.Website.Edit, r.authorize).
		Summary("Change a Website").
		Describe("Its name and its description. Its handle and its site stay as they were connected.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.WebsiteEditRequest{}).
		Sends(http.StatusOK, libs_responses.ActorNode{}).
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such Website")
	websites.DELETE("/:id", r.handler.Website.Delete, r.authorize).With(remove("Website", actorReq.Delete))
	websites.GET("/:id/deletion-counts", r.handler.Website.DeletionCounts, r.authorize).With(counts("Website", handlers.GroupDeletionCounts{}))
	websites.DELETE("/:id/purge", r.handler.Website.Purge, r.authorize).With(purge("Website"))
	websites.PATCH("/:id/enabled", r.handler.Website.SetEnabled, r.authorize).With(enabled)
	websites.PATCH("/:id/featured", r.handler.Website.SetFeatured, r.authorize).With(featured)
	websites.POST("/:id/restore", r.handler.Website.Restore, r.authorize).With(restore("Website"))
	websites.PATCH("/:id/download-default", r.handler.Website.EditDownloadDefault, r.authorize).With(downloads(libs_responses.ActorNode{}))

	websites.GET("/:id/admins/", r.handler.WebsiteAdmin.Browse, r.authorize).With(websiteAdmins)
	websites.POST("/:id/admins/", r.handler.WebsiteAdmin.Add, r.authorize).With(addWebsiteAdmin)
	websites.DELETE("/:id/admins/:admin_id", r.handler.WebsiteAdmin.Delete, r.authorize).With(removeWebsiteAdmin)

	// events — an actor like a group, with a time and a place. Everything
	// a group has, an event has, from the same handler told a different
	// kind. What only an event has follows.
	//
	// There is no GET /events/: no list of every event on the site, as
	// there is none of every note or photo. An event is found from an
	// invitation, from whoever hosts it, or by its address.
	events := api.Group("events")
	// The viewer's own: what they answered, look after, or were invited
	// to. Before /:id, though a word is never mistaken for an id.
	events.GET("/mine", r.handler.Event.Mine, r.authorize).
		Summary("The viewer's events").
		Describe("The ones they said going or maybe to, look after, or were invited to.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.EventMineRequest{Limit: 20}).
		Sends(http.StatusOK, libs_responses.List[any]{})
	events.GET("/:id", r.handler.Event.Read, r.setViewer).With(readGroup("event"))
	// Who is going, and who said maybe.
	events.GET("/:id/attendees/", r.handler.Event.Attendees, r.setViewer).
		Summary("Who is going to an event, or may").
		Viewer(apidoc.Anyone).
		Takes(&handlers.EventAttendeesRequest{Limit: 20}).
		Sends(http.StatusOK, libs_responses.List[*libs_responses.ActorNodeItem]{}).
		Refuses(http.StatusForbidden, "the viewer may not see who is going").
		Refuses(http.StatusNotFound, "there is no such event")
	// "Add to calendar".
	events.GET("/:id/ics", r.handler.Event.ICS, r.setViewer).
		Summary("An event for a calendar").
		Describe("As iCalendar (RFC 5545), `text/calendar`, to add to one's own calendar.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.EventIDRequest{}).
		SendsNothing(http.StatusOK, "the event, as `text/calendar`").
		Refuses(http.StatusForbidden, "the viewer may not see it").
		Refuses(http.StatusNotFound, "there is no such event")
	events.POST("/", r.handler.Event.Add, r.authorize).
		Summary("Make an event").
		Describe("Its own, or hosted by a group whoever makes it administers.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.EventAddRequest{}).
		Sends(http.StatusCreated, libs_responses.ActorNode{}).
		Sends(http.StatusBadRequest, libs_responses.Refusal{}, "`invalid_timezone`, `invalid_time`, `ends_before_start`, `too_long` or `already_over`; or not well formed, without a body").
		Sends(http.StatusForbidden, libs_responses.Refusal{}, "`host_not_allowed`: the viewer does not administer the group").
		Sends(http.StatusNotFound, libs_responses.Refusal{}, "`host_not_allowed`: there is no such group")
	events.PATCH("/:id", r.handler.Event.Edit, r.authorize).
		Summary("Change an event").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.EventEditRequest{}).
		Sends(http.StatusOK, libs_responses.ActorNode{}).
		Sends(http.StatusBadRequest, libs_responses.Refusal{}, "`invalid_timezone`, `invalid_time`, `ends_before_start` or `too_long`; or not well formed, without a body").
		Refuses(http.StatusForbidden, "the viewer may not change it").
		Refuses(http.StatusNotFound, "there is no such event")
	events.DELETE("/:id", r.handler.Event.Delete, r.authorize).With(remove("event", actorReq.Delete))
	events.GET("/:id/deletion-counts", r.handler.Event.DeletionCounts, r.authorize).With(counts("event", handlers.GroupDeletionCounts{}))
	events.DELETE("/:id/purge", r.handler.Event.Purge, r.authorize).With(purge("event"))
	events.POST("/:id/archive", r.handler.Event.Archive, r.authorize).With(archive("event"))
	events.PATCH("/:id/enabled", r.handler.Event.SetEnabled, r.authorize).With(enabled)
	events.POST("/:id/restore", r.handler.Event.Restore, r.authorize).With(restore("event"))
	events.PATCH("/:id/access", r.handler.Event.EditAccess, r.authorize).With(access(actorReq.EditAccess, libs_responses.ActorNode{}))
	events.GET("/:id/rules", r.handler.Event.ReadRules, r.setViewer).With(readRules("event"))
	events.PUT("/:id/rules", r.handler.Event.SetRules, r.authorize).With(setRules("event"))
	events.POST("/:id/rules/agree", r.handler.Event.AgreeRules, r.authorize).With(agreeRules("event"))
	events.PATCH("/:id/features", r.handler.Event.EditFeatures, r.authorize).With(features(actorReq.EditFeatures, libs_responses.ActorNode{}))

	events.GET("/:id/admins/", r.handler.EventAdmin.Browse, r.authorize).With(eventAdmins)
	events.POST("/:id/admins/", r.handler.EventAdmin.Add, r.authorize).With(addEventAdmin)
	events.DELETE("/:id/admins/:admin_id", r.handler.EventAdmin.Delete, r.authorize).With(removeEventAdmin)

	// Going, maybe, and not after all.
	events.PUT("/:id/rsvp", r.handler.Event.Rsvp, r.authorize).
		Summary("Say going, or maybe").
		Describe("Answers with the event.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.EventRsvpRequest{}).
		Sends(http.StatusOK, libs_responses.ActorNode{}).
		Refuses(http.StatusForbidden, "the viewer may not go to it").
		Refuses(http.StatusNotFound, "there is no such event").
		Sends(http.StatusConflict, libs_responses.Refusal{}, "`event_full`: every place is taken; maybe is still open; or `rules_not_agreed`: it is a group or an event with rules the viewer has not agreed to; tick each and agree (`POST /groups|events/:id/rules/agree`), then ask again").
		Sends(http.StatusGone, libs_responses.Refusal{}, "`event_closed`: it is over or called off")
	events.DELETE("/:id/rsvp", r.handler.Event.Unrsvp, r.authorize).
		Summary("Not going after all").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.EventIDRequest{}).
		SendsNothing(http.StatusNoContent, "done").
		Refuses(http.StatusForbidden, "the viewer may not").
		Refuses(http.StatusNotFound, "there is no such event")
	// Calling it off. It is kept, saying so.
	events.POST("/:id/cancel", r.handler.Event.Cancel, r.authorize).
		Summary("Call an event off").
		Describe("It is kept, saying so. Answers with it.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.EventIDRequest{}).
		Sends(http.StatusOK, libs_responses.ActorNode{}).
		Refuses(http.StatusForbidden, "the viewer does not look after it").
		Refuses(http.StatusNotFound, "there is no such event")

	return doc
}
