package handlers

import (
	"log"
	"net/http"

	"anahita-libs/data/services"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// ErrOff is the error a request gets for a kind of profile or of post the
// instance does not offer (roadmap row 139, docs/instance-features.md).
const ErrOff = "off"

// ProfileOffered is whether the instance offers a kind of profile ("group",
// "event", "website"), asked of the graph. Where it cannot be asked, it is:
// whatever made it unreachable stops the request anyway.
func ProfileOffered(graph services.GraphGRPCInterface, kind string) bool {
	if graph == nil {
		return true
	}
	offered, err := graph.ReadInstance()
	if err != nil {
		log.Printf("what the instance offers: %v", err)
		return true
	}
	return offered.OffersProfile(kind)
}

// PostOffered is whether the instance offers a kind of post ("article",
// "video"), asked of the graph; as ProfileOffered, it is where the graph
// cannot be asked, and the graph refuses it all the same when it saves one.
func PostOffered(graph services.GraphGRPCInterface, kind string) bool {
	if graph == nil {
		return true
	}
	offered, err := graph.ReadInstance()
	if err != nil {
		log.Printf("what the instance offers: %v", err)
		return true
	}
	return offered.OffersPost(kind)
}

// RefuseOff answers a request for what the instance does not offer: 403.
func RefuseOff(c echo.Context, kind string) error {
	return c.JSON(http.StatusForbidden, responses.Refusal{
		Error:       ErrOff,
		Description: "this site does not offer " + kind + "s",
	})
}
