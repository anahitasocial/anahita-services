package handlers

import (
	"log"
	"net/http"
	"strings"

	"actor/constants"

	an_handlers "anahita-libs/cmd/api/handlers"
	an_constants "anahita-libs/constants"
	"anahita-libs/data/services"

	"github.com/labstack/echo/v4"
)

// HandleResponse is which profile an alias is: where the web app reads it,
// and by what.
type HandleResponse struct {
	// "people", "groups" or "events": the profile's own address begins so.
	Kind string `json:"kind"`
	// Its id.
	ID int64 `json:"id"`
	// Its alias, as it is kept.
	Alias string `json:"alias"`
}

// Handle says which profile an alias is, of any kind: GET /actors/:alias.
//
// A profile's address in the web app is its alias, /@ana, whatever kind of
// profile it is: a person, somebody on another server (/@ana@mastodon.social,
// /@ana.bsky.social), a group or an event. Aliases are one namespace across
// every kind, so at most one is found. Answered only when the viewer may see
// it; anybody else is told there is none, as reading it would tell them.
func (h *personHandler) Handle(c echo.Context) error {
	viewer := getViewer(c)
	alias := strings.TrimSpace(strings.TrimPrefix(c.Param("alias"), "@"))
	if alias == "" || len(alias) > 100 {
		return c.NoContent(http.StatusNotFound)
	}

	kinds := []struct {
		kind     string
		nodeType string
	}{
		{"people", personTypeOf(alias)},
		{"groups", constants.NodeActorGroupType},
		{"events", an_constants.NodeActorEventEntityType},
		{"pulses", an_constants.NodeActorPulseEntityType},
	}
	for _, each := range kinds {
		req := services.NewGraphNodeRequest().ReadByAlias
		req.Alias = alias
		req.NodeType = each.nodeType
		node, err := h.Srv.GraphNodeGRPC.ReadByAlias(req)
		if err != nil || node == nil || node.ID == 0 {
			continue
		}
		if !an_handlers.MaySee(h.Srv.GraphNodeGRPC, node.ID, viewer) {
			log.Printf("handle %q: not for this viewer", alias)
			return c.NoContent(http.StatusNotFound)
		}
		return c.JSON(http.StatusOK, HandleResponse{Kind: each.kind, ID: node.ID, Alias: node.Alias})
	}
	return c.NoContent(http.StatusNotFound)
}
