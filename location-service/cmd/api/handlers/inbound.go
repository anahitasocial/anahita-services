package handlers

import (
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type inboundHandler struct {
	*Env
}

// NewInboundHandler creates a new inbound handler
func NewInboundHandler(env *Env) *inboundHandler {
	return &inboundHandler{
		env,
	}
}

// Browse answers a page of the posts it is on, as every kind of tag's is
// (anahita-libs/handlers, TaggedWith).
func (h *inboundHandler) Browse(c echo.Context) error {
	return an_handlers.TaggedWith(c, an_handlers.TagReaders{
		Perms: h.Perms.Taggables,
		Nodes: h.Srv.GraphNodeGRPC,
		Graph: h.Srv.GraphGRPC,
		Files: h.Srv.FilesGRPC,
		Media: h.Srv.MediaGRPC,
	}, an_constants.EdgeTagLocationType)
}

// Add adds a new association to a location
func (h *inboundHandler) Add(c echo.Context) error {
	var (
		err    error
		req    = requests.NewInboundRequest().Add
		viewer = getViewer(c)
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	location, err := h.Srv.GraphNodeGRPC.Read(req.TargetID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	source, err := h.Srv.GraphNodeGRPC.Read(req.SourceID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Taggables.CanAdd(viewer, source); !ok && !h.administers(viewer, source) {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	edge := entities.NewAnahitaEdge()
	edge.Type = an_constants.EdgeTagLocationType
	edge.NodeAID = location.ID
	edge.NodeAType = location.Type
	edge.NodeBID = source.ID
	edge.NodeBType = source.Type
	edge.CreatedBy = viewer.ID

	// Tied to it already: there is nothing to do.
	tied, err := h.Srv.GraphEdgeGRPC.CheckEdgeExists(edge)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if tied {
		return c.NoContent(http.StatusCreated)
	}

	// A post is tied to MaxLocationsOfAPost places at most.
	places, err := h.Srv.GraphEdgeGRPC.Count(&services.GraphEdgeCountRequest{
		Types:     []string{an_constants.EdgeTagLocationType},
		NodeBID:   source.ID,
		NodeBType: source.Type,
	})
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if places >= an_constants.MaxLocationsOfAPost {
		return c.JSON(http.StatusUnprocessableEntity, responses.Refusal{Error: "too_many_locations"})
	}

	_, err = h.Srv.GraphEdgeGRPC.Add(edge)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusCreated)
}

// Delete removes a tagged node from location
func (h *inboundHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewInboundRequest().Delete
		viewer = getViewer(c)
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	location, err := h.Srv.GraphNodeGRPC.Read(req.TargetID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	source, err := h.Srv.GraphNodeGRPC.Read(req.SourceID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Taggables.CanDelete(viewer, source); !ok && !h.administers(viewer, source) {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	gReq := &services.GraphEdgeBatchReadByFiltersRequest{
		NodeAID:   location.ID,
		NodeAType: location.Type,
		NodeBID:   source.ID,
		NodeBType: source.Type,
		Types:     []string{an_constants.EdgeTagLocationType},
	}
	edges, err := h.Srv.GraphEdgeGRPC.BatchReadByFilters(gReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, edge := range edges {
		err = h.Srv.GraphEdgeGRPC.Delete(edge.ID)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	return c.NoContent(http.StatusOK)
}

// administers says whether the viewer is one of the administrators of a
// profile that is not a person: a group, a channel or an event, whose
// locations its administrators look after as its owner would. Only the
// one who made it is its author; the others are known by the graph.
func (h *inboundHandler) administers(viewer *entities.Viewer, source *entities.AnahitaNode) bool {
	if viewer == nil || viewer.ID <= 0 || source == nil ||
		!strings.HasPrefix(source.Type, an_constants.NodeActorPrefixEntityType) ||
		strings.Contains(source.Type, ".person-service.person.") {
		return false
	}

	edge := entities.NewAnahitaEdge()
	edge.Type = an_constants.EdgeActorAdminEntityType
	edge.NodeAID = viewer.ID
	edge.NodeBID = source.ID
	found, err := h.Srv.GraphEdgeGRPC.CheckEdgeExists(edge)
	if err != nil {
		log.Println("is the viewer an administrator of", source.ID, err)
		return false
	}
	return found
}
