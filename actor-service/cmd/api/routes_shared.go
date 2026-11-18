package main

import (
	"net/http"
	"strings"

	"actor/cmd/api/handlers"
	"actor/responses"

	"anahita-libs/apidoc"
	libs_handlers "anahita-libs/handlers"
	"anahita-libs/requests"
	libs_responses "anahita-libs/responses"
)

// shared holds what the routes of every kind of profile describe alike:
// the request shapes they take, and the descriptions of the routes that
// each kind has at its own address. Each description is a function that
// fills in an *apidoc.Route, used with Route.With. Most take the kind,
// "person", "group", "channel" or "event", to name it in the summary, and
// they mark the route Shared("profiles"), so that the reference lists it
// under each kind as every profile's.
type shared struct {
	// The request shapes of the routes on any kind of profile, and of a
	// person's own.
	actorReq  *requests.ActorRequest
	personReq *requests.PersonRequest

	// How a list of profiles says what is in it: the end of the Describe
	// of each route that lists profiles.
	someOrLimited string

	// Deleting, for thirty days, and what deleting would take with it.
	remove func(kind string, takes any) func(*apidoc.Route)
	counts func(kind string, sends any) func(*apidoc.Route)

	// Destroying for good, by a super administrator: one, many at once,
	// and how far a purge of many has got. A route apart from deleting,
	// so that the difference between recoverable and gone is never a
	// parameter.
	purge         func(kind string) func(*apidoc.Route)
	purgeBatch    func(kind string) func(*apidoc.Route)
	purgeProgress func(*apidoc.Route)

	// Removing as spam, a moderator's decision.
	spam func(kind string) func(*apidoc.Route)

	// A group's and an event's rules (roadmap row 102, part 5): reading
	// them, setting them, and agreeing to them.
	readRules  func(kind string) func(*apidoc.Route)
	setRules   func(kind string) func(*apidoc.Route)
	agreeRules func(kind string) func(*apidoc.Route)

	// Archiving, for good with nothing destroyed; turning a profile on or
	// off, reversibly, for site administrators; featuring it for
	// onboarding; and restoring one deleted within the thirty days, for
	// the kinds that cannot sign in to restore themselves.
	archive  func(kind string) func(*apidoc.Route)
	enabled  func(*apidoc.Route)
	featured func(*apidoc.Route)
	restore  func(kind string) func(*apidoc.Route)

	// Who may see a profile, and may ask to follow it; who may see its
	// followers and whom it follows (roadmap row 151); what may be posted
	// on it; and what its new photos and videos start with about being
	// downloaded. Each answers with the profile, in the shape `sends`.
	access      func(takes any, sends any) func(*apidoc.Route)
	graphAccess func(sends any) func(*apidoc.Route)
	features    func(takes any, sends any) func(*apidoc.Route)
	downloads   func(sends any) func(*apidoc.Route)

	// Reading one group, channel or event, or what a stranger is shown of
	// it.
	readGroup func(kind string) func(*apidoc.Route)

	// Each kind's admins: listing them, adding one, and removing one.
	groupAdmins, addGroupAdmin, removeGroupAdmin       func(*apidoc.Route)
	eventAdmins, addEventAdmin, removeEventAdmin       func(*apidoc.Route)
	channelAdmins, addChannelAdmin, removeChannelAdmin func(*apidoc.Route)
}

// newShared makes the descriptions in shared. They are written here once,
// as closures over the request shapes and the wording they share.
func newShared() *shared {
	actorReq := requests.NewActorRequest()
	personReq := requests.NewPersonRequest()

	// The request shapes of the admin routes, for admins below.
	adminsReq := requests.NewActorAdminsRequest()

	// What a route that asks for proof again says when it refuses.
	stepUp := "`step_up_required`: the viewer must prove who they are again first; or the viewer " +
		"may not, without a body"
	someOrLimited := "Each a full profile, or, for one the viewer may only ask to follow, what a " +
		"stranger is shown of it (`restricted`)."

	// "a group", "an event": the kind's name after its article, in a
	// summary.
	a := func(kind string) string {
		if strings.ContainsRune("aeiou", rune(kind[0])) {
			return "an " + kind
		}
		return "a " + kind
	}

	// "A group", at the start of one.
	A := func(kind string) string {
		phrase := a(kind)
		return strings.ToUpper(phrase[:1]) + phrase[1:]
	}

	// A change to a profile that answers with nothing: done, or so
	// already. enabled and featured are made from it.
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
			route.Shared("profiles").
				Summary("Delete "+a(kind)).
				Describe("It is out of sight at once and destroyed after thirty days, unless it "+
					"is restored before then.").
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
			route.Shared("profiles").
				Summary("What deleting "+a(kind)+" would take with it").
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
			route.Shared("profiles").
				Summary("Destroy "+a(kind)+" for good").
				Describe("Everything of it, at once and with no thirty days. Super administrators "+
					"only, never on their own profile. Queued, and done by the sweeper.").
				Viewer(apidoc.SuperAdmin).
				StepUp().
				Takes(actorReq.Delete).
				Sends(http.StatusAccepted, libs_handlers.PurgeQueued{}, "queued").
				Sends(http.StatusOK, libs_handlers.PurgeQueued{}, "purged or queued already").
				Sends(http.StatusForbidden, libs_responses.Refusal{}, stepUp).
				Sends(http.StatusConflict, libs_handlers.PurgeRefused{},
					"it cannot be purged; nothing was queued")
		}
	}

	readRules := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("The rules of "+a(kind)).
				Describe("What somebody agrees to, each ticked, before following it or answering "+
					"it: at most ten, in order, with the version they are at and the version the "+
					"viewer last agreed to. For whoever may see it, and for whoever it would let "+
					"ask to follow.").
				Viewer(apidoc.Anyone).
				Takes(actorReq.Read).
				Sends(http.StatusOK, handlers.Rules{}).
				Refuses(http.StatusForbidden, "the viewer may not see it").
				Refuses(http.StatusNotFound, "there is no such "+kind)
		}
	}

	setRules := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Set the rules of "+a(kind)).
				Describe("The rules as they are to be from now on, in order, at most ten; an "+
					"empty list takes them away. Every change raises the version, so that whoever "+
					"agreed before is asked again. For whoever may change it.").
				Viewer(apidoc.SignedIn).
				Takes(&handlers.RulesSetRequest{}).
				Sends(http.StatusOK, handlers.Rules{}).
				Sends(http.StatusBadRequest, libs_responses.Refusal{}, "`rules_invalid`: more "+
					"than ten, or one too long").
				Refuses(http.StatusForbidden, "the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such "+kind)
		}
	}

	agreeRules := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Agree to the rules of "+a(kind)).
				Describe("The viewer agreeing, each rule ticked, to the version they were shown.").
				Viewer(apidoc.SignedIn).
				Takes(&handlers.RulesAgreeRequest{}).
				Sends(http.StatusNoContent, nil).
				Sends(http.StatusConflict, libs_responses.Refusal{}, "`rules_not_current`: the "+
					"rules changed since; read them again").
				Refuses(http.StatusNotFound, "there is no such "+kind)
		}
	}

	spam := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("Remove "+a(kind)+" as spam").
				Describe("A moderator's decision (roadmap row 102): everything of it destroyed at "+
					"once, with no thirty days, and recorded as `removed-as-spam` with who "+
					"decided. Administrators and super administrators; never an administrator's "+
					"or a super administrator's account, nor one's own. Queued, and done by the "+
					"sweeper.").
				Viewer(apidoc.Admin).
				Takes(actorReq.Delete).
				Sends(http.StatusAccepted, libs_handlers.PurgeQueued{}, "queued").
				Sends(http.StatusOK, libs_handlers.PurgeQueued{}, "purged or queued already").
				Refuses(http.StatusForbidden, "the viewer is not an administrator").
				Sends(http.StatusConflict, libs_handlers.PurgeRefused{}, "it cannot be removed "+
					"(`administrator`, `yourself`, `not_found`, `wrong_type`); nothing was queued")
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
				Sends(http.StatusBadRequest, libs_responses.Refusal{},
					"`confirmation_mismatch`: `confirm` does not name how many there are; or not "+
						"well formed, without a body").
				Sends(http.StatusForbidden, libs_responses.Refusal{}, stepUp).
				Sends(http.StatusConflict, libs_handlers.PurgeRefused{},
					"some cannot be purged; nothing was queued")
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
			route.Shared("profiles").
				Summary("Archive "+a(kind)).
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

	// enabled marks as Shared what changed describes.
	enabledChange := changed("Turn a profile on or off", "Reversible, site administrators only. "+
		"Off, it is out of sight and may not act; nothing is deleted.", &handlers.EnabledRequest{})
	enabled := func(route *apidoc.Route) {
		route.Shared("profiles").With(enabledChange)
	}

	featured := func(route *apidoc.Route) {
		route.Shared("profiles").
			With(changed("Feature a profile, or stop", "What onboarding offers new members to "+
				"follow. Super administrators only.", &handlers.FeaturedRequest{})).
			Sends(http.StatusConflict, libs_responses.Refusal{}, "`not_featurable`: it is "+
				"disabled, archived or deleted")
	}

	// Each kind's shape, for what answers with one: restore and readGroup.
	shapes := map[string]any{
		"group":   responses.GroupNode{},
		"event":   responses.EventNode{},
		"channel": responses.ChannelNode{},
	}

	restore := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Shared("profiles").
				Summary("Restore a deleted "+kind).
				Describe("Within the thirty days. Answers with it.").
				Viewer(apidoc.SignedIn).
				Takes(actorReq.Delete).
				Sends(http.StatusOK, shapes[kind]).
				Refuses(http.StatusForbidden, "the viewer may not").
				Refuses(http.StatusNotFound, "there is no such profile to restore")
		}
	}

	graphAccess := func(sends any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Shared("profiles").
				Summary("Change who may see its followers and whom it follows").
				Describe("An access level, or empty for whoever may see the profile; never wider "+
					"than the profile. Groups and events have no `leaders` or `mutuals`.").
				Viewer(apidoc.SignedIn).
				Takes(&requests.ActorEditGraphAccessRequest{}).
				Sends(http.StatusOK, sends).
				Sends(http.StatusBadRequest, libs_responses.Refusal{}, "`graph_access_invalid`, "+
					"or a level that is not one").
				Refuses(http.StatusForbidden, "the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such profile")
		}
	}

	access := func(takes any, sends any) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Shared("profiles").
				Summary("Change who may see it").
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
			route.Shared("profiles").
				Summary("Change what may be posted on it").
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
			route.Shared("profiles").
				Summary("Change whether new photos and videos may be downloaded").
				Describe("What a post starts with; each post can say otherwise.").
				Viewer(apidoc.SignedIn).
				Takes(&requests.ActorEditDownloadDefaultRequest{}).
				Sends(http.StatusOK, sends).
				Refuses(http.StatusForbidden, "the viewer may not change it").
				Refuses(http.StatusNotFound, "there is no such profile")
		}
	}

	// The three admin routes of a kind, in the order they are registered.
	admins := func(kind string) (func(*apidoc.Route), func(*apidoc.Route), func(*apidoc.Route)) {
		return func(route *apidoc.Route) {
				route.Shared("profiles").
					Summary(A(kind)+"'s admins").
					Viewer(apidoc.SignedIn).
					Takes(adminsReq.Browse).
					Sends(http.StatusOK, libs_responses.Unpaged[*libs_responses.ActorNodeItem]{}).
					Refuses(http.StatusForbidden, "the viewer may not see them").
					Refuses(http.StatusNotFound, "there is no such "+kind)
			}, func(route *apidoc.Route) {
				route.Shared("profiles").
					Summary("Make somebody an admin of "+a(kind)).
					Describe("By one of its admins. Answers with them.").
					Viewer(apidoc.SignedIn).
					Takes(adminsReq.Add).
					Sends(http.StatusCreated, libs_responses.ActorNodeItem{}).
					Refuses(http.StatusForbidden, "the viewer may not").
					Refuses(http.StatusNotFound, "there is no such "+kind+" or person")
			}, func(route *apidoc.Route) {
				route.Shared("profiles").
					Summary("Remove an admin of "+a(kind)).
					Viewer(apidoc.SignedIn).
					Takes(adminsReq.Delete).
					SendsNothing(http.StatusNoContent, "removed").
					Refuses(http.StatusForbidden, "the viewer may not").
					Refuses(http.StatusNotFound, "there is no such "+kind+" or admin")
			}
	}

	readGroup := func(kind string) func(*apidoc.Route) {
		return func(route *apidoc.Route) {
			route.Summary("One "+kind).
				Describe("To somebody it is not open to, but who may ask to follow it, what a "+
					"stranger is shown of it.").
				Viewer(apidoc.Anyone).
				Takes(actorReq.Read).
				Sends(http.StatusOK, shapes[kind],
					"it, or what a stranger is shown of it (LimitedActor)").
				Refuses(http.StatusForbidden, "the viewer may not see it").
				Refuses(http.StatusNotFound, "there is no such "+kind)
		}
	}

	groupAdmins, addGroupAdmin, removeGroupAdmin := admins("group")
	eventAdmins, addEventAdmin, removeEventAdmin := admins("event")
	channelAdmins, addChannelAdmin, removeChannelAdmin := admins("channel")

	return &shared{
		actorReq:      actorReq,
		personReq:     personReq,
		someOrLimited: someOrLimited,

		remove:        remove,
		counts:        counts,
		purge:         purge,
		readRules:     readRules,
		setRules:      setRules,
		agreeRules:    agreeRules,
		spam:          spam,
		purgeBatch:    purgeBatch,
		purgeProgress: purgeProgress,
		archive:       archive,
		enabled:       enabled,
		featured:      featured,
		restore:       restore,
		graphAccess:   graphAccess,
		access:        access,
		features:      features,
		downloads:     downloads,
		readGroup:     readGroup,

		groupAdmins:        groupAdmins,
		addGroupAdmin:      addGroupAdmin,
		removeGroupAdmin:   removeGroupAdmin,
		eventAdmins:        eventAdmins,
		addEventAdmin:      addEventAdmin,
		removeEventAdmin:   removeEventAdmin,
		channelAdmins:      channelAdmins,
		addChannelAdmin:    addChannelAdmin,
		removeChannelAdmin: removeChannelAdmin,
	}
}
