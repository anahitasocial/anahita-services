package main

import (
	"net/http"

	"like/cmd/api/handlers"
	"like/requests"

	"anahita-libs/apidoc"
	libs_constants "anahita-libs/constants"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

type Routes struct {
	handler   *handlers.Handlers
	authorize echo.MiddlewareFunc
}

func NewRoutes(
	handler *handlers.Handlers,
	authorize echo.MiddlewareFunc,
) *Routes {
	return &Routes{
		handler:   handler,
		authorize: authorize,
	}
}

// Init registers the routes, each with its description, and answers with
// the descriptions: docs/api/like-service.json is made of them.
func (r *Routes) Init(e *echo.Echo) *apidoc.Service {
	doc := apidoc.NewService("like-service", "Likes",
		"Liking a post or a reply, and who has liked it.")
	api := doc.On(e)

	// likes — every route requires a valid OAuth access token.
	likes := api.Group("likes", r.authorize)
	likes.GET("/:node_id/", r.handler.Like.Browse).
		Summary("Who likes it").
		Describe("The people who like a post or a reply that the viewer may see, each saying how they stand with the viewer. Somebody its owner or author has blocked is left out (permissions.md).").
		Viewer(apidoc.SignedIn).
		Takes(requests.NewLikeRequest().Browse).
		Sends(http.StatusOK, responses.List[*responses.ActorNodeItem]{}).
		Refuses(http.StatusForbidden, "the viewer may not see it").
		Refuses(http.StatusNotFound, "there is no such post or reply")
	likes.POST("/:node_id/", r.handler.Like.Add).
		Summary("Like it").
		Describe("A reply is liked by the rules of the post at the top of its thread. Answers with what was liked.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeLikesWrite).
		Takes(requests.NewLikeRequest().Add).
		Sends(http.StatusCreated, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer may not like it").
		Refuses(http.StatusNotFound, "there is no such post or reply").
		Refuses(http.StatusConflict, "the viewer likes it already")
	likes.DELETE("/:node_id/", r.handler.Like.Delete).
		Summary("Stop liking it").
		Describe("Answers with what was liked.").
		Viewer(apidoc.SignedIn).
		Scope(libs_constants.OAuthScopeLikesWrite).
		Takes(requests.NewLikeRequest().Delete).
		Sends(http.StatusOK, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer does not like it").
		Refuses(http.StatusNotFound, "there is no such post or reply")

	return doc
}
