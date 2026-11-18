package main

import (
	"net/http"

	"activitypub/cmd/api/handlers"
	"activitypub/data/repositories"

	"anahita-libs/apidoc"
	"anahita-libs/federation"
	"anahita-libs/responses"
)

// --- Servers refused ---
// The servers this installation refuses. Every route needs a token
// (r.authorize, on the group), and the handlers check that the viewer is
// a super administrator.
func (r *Routes) refusalRoutes(activitypub *apidoc.Router) {
	activitypub.GET("/blocks", r.handler.BrowseBlocks).
		Summary("The servers refused").
		Viewer(apidoc.SuperAdmin).
		Takes(&handlers.BlocksBrowse{}).
		Unbound().
		Sends(http.StatusOK, responses.List[*federation.DomainBlock]{})

	activitypub.POST("/blocks", r.handler.AddBlock).
		Summary("Refuse a server, or change why").
		Describe("Nothing from it is taken, and nothing is sent to it.").
		Viewer(apidoc.SuperAdmin).
		Takes(&handlers.BlockRequest{}).
		Sends(http.StatusCreated, federation.DomainBlock{}).
		Sends(http.StatusBadRequest, responses.Refusal{},
			"`domain_required`: the request names no server; `domain_unreadable`: it is not a "+
				"server's name; `domain_is_here`: it is this installation")

	// --- Accounts refused ---
	// The accounts refused one by one, on the fediverse and on Bluesky
	// (docs/plans/done/refused-accounts.md), and who from other networks is
	// kept here to choose them from. Every route needs a token (r.authorize,
	// on the group), and the handlers check that the viewer is an
	// administrator.
	activitypub.GET("/known-profiles", r.handler.BrowseKnownProfiles).
		Summary("Who from other networks is kept here").
		Describe("Newest first. `q` narrows it to the names and handles it is found in. For "+
			"administrators: these people did not ask to be listed.").
		Viewer(apidoc.Admin).
		Takes(&handlers.BlocksBrowse{}).
		Unbound().
		Sends(http.StatusOK, responses.List[*repositories.KnownProfile]{})

	activitypub.GET("/refused-accounts", r.handler.BrowseRefusedAccounts).
		Summary("The accounts refused").
		Describe("Newest first. `q` narrows it to the names, handles and reasons it is found in.").
		Viewer(apidoc.Admin).
		Takes(&handlers.BlocksBrowse{}).
		Unbound().
		Sends(http.StatusOK, responses.List[*federation.RefusedAccount]{})

	activitypub.POST("/refused-accounts", r.handler.RefuseAccount).
		Summary("Refuse an account from another network").
		Describe("By the profile kept here (`actor_id`), or by a handle or an address "+
			"(`address`): `@user@server`, an actor's address, a Bluesky handle or a DID. Nothing "+
			"from it is taken, nothing is sent to it, it cannot be looked up or followed, and "+
			"what is here of it is removed: at once on the fediverse, within a minute on Bluesky.").
		Viewer(apidoc.Admin).
		Takes(&handlers.RefuseAccountRequest{}).
		Sends(http.StatusCreated, federation.RefusedAccount{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`account_required`: the request names "+
			"no account; `account_unreadable`: the address is not one").
		Sends(http.StatusNotFound, responses.Refusal{}, "`account_not_found`").
		Sends(http.StatusConflict, responses.Refusal{}, "`is_local`: the profile is this "+
			"installation's own; `already_refused`: it, or its server, is refused already")

	activitypub.DELETE("/refused-accounts/:id", r.handler.LetAccountBack).
		Summary("Stop refusing an account").
		Describe("What was removed does not come back: the account can be looked up and followed "+
			"again.").
		Viewer(apidoc.Admin).
		Takes(&handlers.RefusedAccountRef{}).
		Unbound().
		SendsNothing(http.StatusNoContent, "done").
		Refuses(http.StatusNotFound, "it is not refused")

	// Letting a server back belongs with the servers refused above. It is
	// registered last so that the API reference keeps its order.
	activitypub.DELETE("/blocks/:domain", r.handler.DeleteBlock).
		Summary("Stop refusing a server").
		Viewer(apidoc.SuperAdmin).
		Takes(&handlers.DomainRef{}).
		Unbound().
		SendsNothing(http.StatusNoContent, "done").
		Refuses(http.StatusNotFound, "it is not refused")
}
