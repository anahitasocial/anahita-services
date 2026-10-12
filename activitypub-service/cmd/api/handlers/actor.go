package handlers

import (
	"log"
	"net/http"
	"regexp"

	"activitypub/ap"

	"anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

var uuidShape = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// actorOf is the person an address names, if they are on the fediverse.
func (h *Handlers) actorOf(c echo.Context) *entities.AnahitaNode {
	return h.federating(h.personByUUID(c.Param("uuid")))
}

// Actor describes a person here to another server.
//
// It is what a server fetches when it first hears of somebody, and again
// whenever it wants to check a signature: the public half of their key is
// in it.
func (h *Handlers) Actor(c echo.Context) error {
	person := h.actorOf(c)
	if person == nil {
		return c.NoContent(http.StatusNotFound)
	}

	publicKey, err := h.publicKeyOf(person.ID)
	if err != nil {
		log.Println("activitypub key:", err)
		return c.NoContent(http.StatusServiceUnavailable)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=180")
	return activityJSON(c, ap.NewActor(h.addresses, person, publicKey))
}

// Followers says how many follow a person, and not who.
func (h *Handlers) Followers(c echo.Context) error {
	person := h.actorOf(c)
	if person == nil {
		return c.NoContent(http.StatusNotFound)
	}
	return activityJSON(c, ap.NewCollection(h.addresses.Followers(person.UUID), person.FollowerCount))
}

// Following says how many a person follows, and not who.
func (h *Handlers) Following(c echo.Context) error {
	person := h.actorOf(c)
	if person == nil {
		return c.NoContent(http.StatusNotFound)
	}
	return activityJSON(c, ap.NewCollection(h.addresses.Following(person.UUID), person.LeaderCount))
}
