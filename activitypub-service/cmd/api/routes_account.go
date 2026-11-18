package main

import (
	"net/http"

	"activitypub/cmd/api/handlers"

	"anahita-libs/apidoc"
	"anahita-libs/responses"
)

// --- On the fediverse ---
// A person's own choice to be on the fediverse, or a channel's, made by
// its administrators. Every route needs a token (r.authorize, on the
// group).
func (r *Routes) accountRoutes(activitypub *apidoc.Router) {
	activitypub.GET("/settings", r.handler.ReadSettings).
		Summary("Whether the viewer is on the fediverse, and could be").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.SettingsQuery{}).
		Unbound().
		Sends(http.StatusOK, handlers.Settings{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`person_unreadable`").
		Refuses(http.StatusForbidden, "somebody else's, and the viewer is not a site "+
			"administrator, nor an administrator of the channel asked about").
		Refuses(http.StatusNotFound, "there is no such person")

	activitypub.PUT("/settings", r.handler.EditSettings).
		Summary("Go on the fediverse, or leave it").
		Describe("It is each person's own choice, off until they make it, and only for somebody "+
			"whose profile is open to everybody. A channel's is its administrators' "+
			"(`?person_id=` with its id).").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.SettingsQuery{}).
		Takes(&handlers.SettingsRequest{}).
		Sends(http.StatusOK, handlers.Settings{}).
		Sends(http.StatusBadRequest, responses.Refusal{},
			"`enabled_required`: nothing to change; `person_unreadable`").
		Refuses(http.StatusForbidden, "somebody else's, and the viewer is not an administrator of "+
			"the channel asked about").
		Sends(http.StatusConflict, responses.Refusal{}, "why they cannot be: `site_private`, "+
			"`site_unreachable` or `profile_not_public`; `moved` for leaving an account that "+
			"moved away, which would take the way to the new one with it")

	// --- Moving an account ---
	// Moving one's own account out and in (roadmap row 124, part 3): the
	// old accounts this one also is, and the move itself.
	activitypub.GET("/aliases", r.handler.ReadAliases).
		Summary("The old accounts the viewer's account also is").
		Describe("Its `alsoKnownAs`: what another server checks before it lets somebody move from "+
			"there to here.").
		Viewer(apidoc.SignedIn).
		Unbound().
		Sends(http.StatusOK, handlers.Aliases{})

	activitypub.PUT("/aliases", r.handler.EditAliases).
		Summary("Name the old accounts the viewer's account also is").
		Describe("Each is found on its own server and kept, as a lookup is; the list takes the "+
			"place of the one before. For somebody on the fediverse; not an account on this site. "+
			"Their followers' servers are sent their description again.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.AliasesRequest{}).
		Sends(http.StatusOK, handlers.Aliases{}).
		Sends(http.StatusBadRequest, responses.Refusal{},
			"`also_known_as_required` or `too_many_aliases`").
		Sends(http.StatusNotFound, handlers.AliasRefusal{},
			"one could not be found, as for a lookup (`lookup_not_found`, `lookup_is_here` for "+
				"one on this site, …), with `alias` naming it").
		Sends(http.StatusConflict, responses.Refusal{}, "`not_on_fediverse` or `not_a_person`").
		Sends(http.StatusBadGateway, responses.Refusal{}, "`lookup_failed`")

	activitypub.GET("/move", r.handler.ReadMove).
		Summary("Whether the viewer's account moved").
		Describe("To which account and when, and, within thirty days of a move, when it may move "+
			"again.").
		Viewer(apidoc.SignedIn).
		Unbound().
		Sends(http.StatusOK, handlers.MoveStatus{})

	activitypub.POST("/move", r.handler.Move).
		Summary("Move the viewer's account to another").
		Describe("To an account elsewhere that names this one in its `alsoKnownAs`, read afresh. "+
			"Its description gains `movedTo`; its followers' servers are sent a `Move`, and its "+
			"followers here follow the new account; its profile says where it went. The account "+
			"can then no longer post, like, follow or tag, until it moves back. Not within thirty "+
			"days of the last move.").
		Viewer(apidoc.SignedIn).
		StepUp().
		Takes(&handlers.MoveRequest{}).
		Sends(http.StatusOK, handlers.MoveStatus{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`to_required`").
		Sends(http.StatusForbidden, responses.Refusal{}, "`step_up_required`: the viewer must "+
			"prove who they are again first (`/reauth`)").
		Sends(http.StatusNotFound, responses.Refusal{},
			"the account could not be found, as for a lookup; `lookup_is_here` for one on this "+
				"site, which is not moved to: a username is changed instead").
		Sends(http.StatusConflict, responses.Refusal{},
			"`not_on_fediverse`, `not_a_person`, `moved_already`, `moved_lately` or "+
				"`not_named_there`: the new account does not name this one").
		Sends(http.StatusBadGateway, responses.Refusal{}, "`lookup_failed`")

	activitypub.DELETE("/move", r.handler.ComeBack).
		Summary("Take the viewer's move back").
		Describe("The profile no longer points to the new account, its description no longer says "+
			"it moved, and the account may post, like and follow again. Its followers stay with "+
			"the new account; moving from there to here brings them back, which other servers "+
			"allow only after this. The thirty days before another move still count from the "+
			"move.").
		Viewer(apidoc.SignedIn).
		Sends(http.StatusOK, handlers.MoveStatus{}).
		Sends(http.StatusConflict, responses.Refusal{},
			"`not_moved`, `not_on_fediverse` or `not_a_person`")
}
