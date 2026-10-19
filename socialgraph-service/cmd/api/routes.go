package main

import (
	"net/http"

	"socialgraph/cmd/api/handlers"
	"socialgraph/requests"

	"anahita-libs/apidoc"
	an_mw "anahita-libs/middleware"
	"anahita-libs/responses"

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
// the descriptions: docs/api/socialgraph-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("socialgraph-service", "Following and blocking",
		"Who follows whom, asking to follow a profile that is not open to all, invitations to follow a group or an event, and blocks (permissions.md).")
	api := doc.On(e)
	sg := requests.NewSocialgraphRequest()
	actors := responses.List[*responses.ActorNodeItem]{}

	// socialgraph — followers and leaders are public but still identify
	// the caller when a token is present, so the viewer drives access
	// filtering. Everything else requires a valid OAuth access token.
	socialgraph := api.Group("socialgraph")
	// Who follows whom is not part of a preview: a visitor is shown the
	// counts on a profile, and has to sign in for the names.
	socialgraph.GET("/:actor_id/followers/", r.handler.Follower.Browse, r.setViewer, an_mw.MembersInPreview()).
		Summary("Who follows a profile").
		Describe("Those the viewer may see, each saying how they stand with the viewer. On an installation in preview, only for somebody signed in.").
		Viewer(apidoc.Anyone).
		Takes(sg.Browse).
		Sends(http.StatusOK, actors).
		Refuses(http.StatusUnauthorized, "the installation is in preview and the viewer is not signed in").
		Refuses(http.StatusForbidden, "the viewer may not see who follows it").
		Refuses(http.StatusNotFound, "there is no such profile")
	socialgraph.GET("/:actor_id/leaders/", r.handler.Leader.Browse, r.setViewer, an_mw.MembersInPreview()).
		Summary("Whom a profile follows").
		Describe("As who follows it.").
		Viewer(apidoc.Anyone).
		Takes(sg.Browse).
		Sends(http.StatusOK, actors).
		Refuses(http.StatusUnauthorized, "the installation is in preview and the viewer is not signed in").
		Refuses(http.StatusForbidden, "the viewer may not see whom it follows").
		Refuses(http.StatusNotFound, "there is no such profile")

	socialgraph.POST("/:target_id/followers/:source_id", r.handler.Follower.Add, r.authorize).
		Summary("Follow a profile").
		Describe("`source_id` follows `target_id`: the viewer, or a follower a profile's administrator adds. Answers with the profile followed. To a profile that is not open to the viewer, ask instead (`POST .../follow-requests/...`).").
		Viewer(apidoc.SignedIn).
		Takes(sg.Add).
		Sends(http.StatusCreated, responses.ActorNodeItem{}).
		Refuses(http.StatusForbidden, "the viewer may not make this follow").
		Sends(http.StatusConflict, responses.Refusal{}, "`rules_not_agreed`: it is a group or an event with rules the viewer has not agreed to; tick each and agree (`POST /groups|events/:id/rules/agree`), then ask again").
		Refuses(http.StatusNotFound, "there is no such profile")
	socialgraph.DELETE("/:target_id/followers/:source_id", r.handler.Follower.Delete, r.authorize).
		Summary("Stop following, or remove a follower").
		Describe("By the follower, or by whoever looks after the profile followed. Answers with the profile.").
		Viewer(apidoc.SignedIn).
		Takes(sg.Delete).
		Sends(http.StatusOK, responses.ActorNodeItem{}).
		Refuses(http.StatusForbidden, "the viewer may not end this follow").
		Refuses(http.StatusNotFound, "there is no such profile")

	socialgraph.GET("/:actor_id/mutuals/", r.handler.Mutual.Browse, r.authorize).
		Summary("Followers in common").
		Describe("The people the viewer follows who also follow the profile.").
		Viewer(apidoc.SignedIn).
		Takes(sg.Browse).
		Sends(http.StatusOK, actors).
		Refuses(http.StatusForbidden, "the viewer may not see its followers").
		Refuses(http.StatusNotFound, "there is no such profile")

	socialgraph.GET("/:actor_id/follow-requests/", r.handler.FollowRequest.Browse, r.authorize).
		Summary("Who asks to follow a profile").
		Describe("For whoever looks after it.").
		Viewer(apidoc.SignedIn).
		Takes(sg.Browse).
		Sends(http.StatusOK, actors).
		Refuses(http.StatusForbidden, "the viewer does not look after it").
		Refuses(http.StatusNotFound, "there is no such profile")
	socialgraph.PATCH("/:target_id/follow-requests/:source_id", r.handler.FollowRequest.Edit, r.authorize).
		Summary("Let somebody who asked follow").
		Describe("The request becomes a follow. Answers with the profile followed.").
		Viewer(apidoc.SignedIn).
		Takes(sg.Edit).
		Sends(http.StatusOK, responses.ActorNodeItem{}).
		Refuses(http.StatusForbidden, "the viewer does not look after the profile").
		Refuses(http.StatusNotFound, "there is no such profile or request")
	socialgraph.POST("/:target_id/follow-requests/:source_id", r.handler.FollowRequest.Add, r.authorize).
		Summary("Ask to follow a profile").
		Describe("For a profile that lets people ask. Answers with what a stranger is shown of it.").
		Viewer(apidoc.SignedIn).
		Takes(sg.Add).
		Sends(http.StatusCreated, responses.LimitedActor{}).
		Refuses(http.StatusForbidden, "the viewer may not ask: the profile does not let people ask, or has blocked them").
		Sends(http.StatusConflict, responses.Refusal{}, "`rules_not_agreed`: it is a group or an event with rules the viewer has not agreed to; tick each and agree (`POST /groups|events/:id/rules/agree`), then ask again").
		Refuses(http.StatusNotFound, "there is no such profile")
	socialgraph.DELETE("/:target_id/follow-requests/:source_id", r.handler.FollowRequest.Delete, r.authorize).
		Summary("Withdraw a request, or turn one down").
		Describe("By whoever asked, who is answered with what a stranger is shown of the profile; or by whoever looks after the profile, answered with it.").
		Viewer(apidoc.SignedIn).
		Takes(sg.Delete).
		Sends(http.StatusOK, responses.ActorNodeItem{}).
		Refuses(http.StatusForbidden, "the viewer may not").
		Refuses(http.StatusNotFound, "there is no such profile or request")

	// Invitations to follow a group. The viewer's own first: it has no
	// actor in its address.
	socialgraph.GET("/invites/", r.handler.Invite.Mine, r.authorize).
		Summary("The viewer's invitations").
		Describe("To follow a group or an event.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.InviteMineRequest{Limit: 20}).
		Sends(http.StatusOK, responses.List[*handlers.InviteItem]{})
	socialgraph.GET("/:actor_id/invites/", r.handler.Invite.Browse, r.authorize).
		Summary("Who is invited to follow a group or an event").
		Describe("For whoever looks after it.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.InviteActorRequest{Limit: 20}).
		Sends(http.StatusOK, responses.List[*handlers.InviteItem]{}).
		Refuses(http.StatusForbidden, "the viewer does not look after it")
	socialgraph.POST("/:actor_id/invites", r.handler.Invite.Add, r.authorize).
		Summary("Invite people to follow a group or an event").
		Describe("People who follow the viewer. Each is told, and chooses. Answers with what became of each, in the order they were named: somebody who cannot be invited does not stop the others.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.InviteAddRequest{}).
		Sends(http.StatusOK, responses.Unpaged[handlers.InviteResult]{}).
		Refuses(http.StatusForbidden, "the viewer may not invite people to it").
		Refuses(http.StatusTooManyRequests, "too many invitations lately")
	socialgraph.POST("/:actor_id/invites/accept", r.handler.Invite.Accept, r.authorize).
		Summary("Accept an invitation").
		Describe("The viewer follows it from now on.").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.InviteActorRequest{}).
		Sends(http.StatusOK, handlers.InviteAnswer{}).
		Refuses(http.StatusForbidden, "the viewer may not follow it").
		Sends(http.StatusConflict, responses.Refusal{}, "`rules_not_agreed`: it is a group or an event with rules the viewer has not agreed to; tick each and agree (`POST /groups|events/:id/rules/agree`), then ask again").
		Refuses(http.StatusNotFound, "the viewer has no invitation to it")
	socialgraph.POST("/:actor_id/invites/decline", r.handler.Invite.Decline, r.authorize).
		Summary("Turn an invitation down").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.InviteActorRequest{}).
		Sends(http.StatusOK, handlers.InviteAnswer{}).
		Refuses(http.StatusForbidden, "the viewer may not").
		Refuses(http.StatusNotFound, "the viewer has no invitation to it")
	socialgraph.DELETE("/:actor_id/invites/:person_id", r.handler.Invite.Delete, r.authorize).
		Summary("Take an invitation back").
		Viewer(apidoc.SignedIn).
		Takes(&handlers.InvitePersonRequest{}).
		SendsNothing(http.StatusNoContent, "taken back").
		Refuses(http.StatusForbidden, "the viewer does not look after it").
		Refuses(http.StatusNotFound, "there is no such invitation")

	socialgraph.GET("/:actor_id/blocks/", r.handler.Block.Browse, r.authorize).
		Summary("Whom a profile has blocked").
		Describe("A person's own, for them; a group's or an event's, for whoever looks after it, and site administrators.").
		Viewer(apidoc.SignedIn).
		Takes(sg.Browse).
		Sends(http.StatusOK, actors).
		Refuses(http.StatusForbidden, "the viewer may not see them").
		Refuses(http.StatusNotFound, "there is no such profile")
	socialgraph.POST("/:target_id/blocks/:source_id", r.handler.Block.Add, r.authorize).
		Summary("Block somebody").
		Describe("`source_id` blocks `target_id`: the viewer, or a group or an event they look after (permissions.md, what a block hides). Answers with whoever is blocked.").
		Viewer(apidoc.SignedIn).
		Takes(sg.Add).
		Sends(http.StatusCreated, responses.ActorNodeItem{}).
		Refuses(http.StatusForbidden, "the viewer may not make this block, or there is one either way already").
		Refuses(http.StatusNotFound, "there is no such profile")
	socialgraph.DELETE("/:target_id/blocks/:source_id", r.handler.Block.Delete, r.authorize).
		Summary("Unblock somebody").
		Describe("Answers with whoever was blocked.").
		Viewer(apidoc.SignedIn).
		Takes(sg.Delete).
		Sends(http.StatusOK, responses.ActorNodeItem{}).
		Refuses(http.StatusForbidden, "the viewer may not lift this block").
		Refuses(http.StatusNotFound, "there is no such profile")

	return doc
}
