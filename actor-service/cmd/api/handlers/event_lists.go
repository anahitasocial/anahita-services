package handlers

import (
	"actor/constants"
	"log"
	"net/http"

	an_handlers "anahita-libs/cmd/api/handlers"
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// EventMineRequest asks for the viewer's own events.
type EventMineRequest struct {
	// upcoming, past, hosting or invited. Upcoming when left out.
	Filter string `query:"filter" validate:"omitempty,oneof=upcoming past hosting invited"`
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send: up to 100.
	Limit uint64 `query:"limit" validate:"omitempty,max=100"`
}

// EventAttendeesRequest asks who is going to an event, or may.
type EventAttendeesRequest struct {
	// The event's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// going or maybe. Going when left out.
	Rsvp string `query:"rsvp" validate:"omitempty,oneof=going maybe"`
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send: up to 100.
	Limit uint64 `query:"limit" validate:"omitempty,max=100"`
}

// Mine lists the events that are the viewer's: the ones they said going or
// maybe to, the ones they look after, and the ones they were invited to.
//
// GET /events/mine?filter=upcoming|past|hosting|invited
//
// There is no list of every event on the site. An event is found by being
// invited to it, from the group or person hosting it, or by its address.
//
// An event the viewer was invited to and may not yet see is sent as its
// name, picture and time, marked restricted, like the limited profile.
func (h *eventHandler) Mine(c echo.Context) error {
	var (
		req    = &EventMineRequest{Limit: an_constants.RequestLimitDefault}
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	filter := req.Filter
	if filter == "" {
		filter = an_constants.EventListUpcoming
	}

	resp := &responses.List[any]{
		Data: []any{},
		Pagination: responses.Pagination{
			Limit:  req.Limit,
			Offset: req.Offset,
		},
	}

	ids, total, err := h.Srv.EventsGRPC.ListFor(viewer.ID, filter, req.Limit, req.Offset)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total

	if len(ids) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	readReq := services.NewActorGRPCRequest().BatchRead
	readReq.IDs = ids
	events, err := h.Srv.ActorsGRPC.BatchRead(readReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	byID := make(map[int64]*entities.AnahitaNode, len(events))
	for _, event := range events {
		byID[event.ID] = event
	}

	rels, err := h.Srv.SocialgraphGRPC.Browse(ids, viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// In the order the list gave them, which is by when each is.
	ordered := make([]*entities.AnahitaNode, 0, len(ids))
	for _, id := range ids {
		if event, ok := byID[id]; ok {
			ordered = append(ordered, event)
		}
	}

	h.setEventDetails(ordered, viewer)
	batchPresignPaths(h.Env, ordered)

	for _, event := range ordered {
		// An event that is no longer theirs to see, in any way, is left
		// out.
		if item := eventListItem(event, rels[event.ID], viewer); item != nil {
			resp.Data = append(resp.Data, item)
		}
	}

	return c.JSON(http.StatusOK, resp)
}

// Attendees lists who is going to an event, or who said maybe, to anybody
// who can see the event. Each of them only if the viewer may see them, as
// with a profile's followers.
//
// GET /events/:id/attendees/?rsvp=going|maybe
func (h *eventHandler) Attendees(c echo.Context) error {
	var (
		req      = &EventAttendeesRequest{Limit: an_constants.RequestLimitDefault}
		graphReq = services.NewGraphRequest().BrowseConnectionNodes
		viewer   = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanRead(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	resp := &responses.List[*responses.ActorNodeItem]{
		Data: []*responses.ActorNodeItem{},
		Pagination: responses.Pagination{
			Limit:  req.Limit,
			Offset: req.Offset,
		},
	}

	graphReq.NodeId = actor.ID
	graphReq.EdgeType = an_constants.EdgeFollowEntityType
	graphReq.Direction = an_constants.GraphQueryDirectionInbound
	graphReq.Offset = req.Offset
	graphReq.Limit = req.Limit
	graphReq.Ordering = an_constants.OrderingConnected
	graphReq.Viewer = viewer
	graphReq.AccessCheck = true
	// Going is the follows that say so. Maybe is every other follow, one
	// with nothing said included.
	if req.Rsvp == an_constants.EventRsvpMaybe {
		graphReq.EdgeMetaNot = an_constants.EventRsvpGoingMeta
	} else {
		graphReq.EdgeMeta = an_constants.EventRsvpGoingMeta
	}

	people, total, err := h.Srv.GraphGRPC.BrowseConnectionNodes(graphReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total

	if len(people) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	an_handlers.SetViewerRelations(h.Srv.SocialgraphGRPC, people, viewer)
	batchPresignPaths(h.Env, people)

	actorResp := &responses.ActorResponse{}
	resp.Data = actorResp.MapNodeItems(people)

	return c.JSON(http.StatusOK, resp)
}

type EventHostedRequest struct {
	// The group.
	ID int64 `param:"id" validate:"required,numeric"`
	// upcoming or past. Upcoming when left out.
	Filter string `query:"filter" validate:"omitempty,oneof=upcoming past"`
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send: up to 100.
	Limit uint64 `query:"limit" validate:"omitempty,max=100"`
}

// HostedBy lists the events a group hosts, to anybody who can see the
// group: the ones still to come, or the ones that are over. Each event only
// if the viewer may see it, so an event for its own followers is not shown
// to somebody who merely follows the group.
//
// GET /groups/:id/events/?filter=upcoming|past
func (h *eventHandler) HostedBy(c echo.Context) error {
	var (
		req    = &EventHostedRequest{Limit: an_constants.RequestLimitDefault}
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	filter := req.Filter
	if filter == "" {
		filter = an_constants.EventListUpcoming
	}

	group, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, constants.NodeActorGroupType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	groupCtx, err := h.getActorAccessContext(group, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanRead(groupCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	// Each an event, or what is shown of one the viewer may only answer as a
	// follower of this group.
	resp := &responses.List[any]{
		Data: []any{},
		Pagination: responses.Pagination{
			Limit:  req.Limit,
			Offset: req.Offset,
		},
	}

	ids, total, err := h.Srv.EventsGRPC.ListHostedBy(group.ID, viewer, filter, req.Limit, req.Offset)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total

	if len(ids) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	readReq := services.NewActorGRPCRequest().BatchRead
	readReq.IDs = ids
	events, err := h.Srv.ActorsGRPC.BatchRead(readReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	byID := make(map[int64]*entities.AnahitaNode, len(events))
	for _, event := range events {
		byID[event.ID] = event
	}
	ordered := make([]*entities.AnahitaNode, 0, len(ids))
	for _, id := range ids {
		if event, ok := byID[id]; ok {
			ordered = append(ordered, event)
		}
	}

	rels, err := h.Srv.SocialgraphGRPC.Browse(ids, viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.setEventDetails(ordered, viewer)
	batchPresignPaths(h.Env, ordered)

	for _, event := range ordered {
		if item := eventListItem(event, rels[event.ID], viewer); item != nil {
			resp.Data = append(resp.Data, item)
		}
	}

	return c.JSON(http.StatusOK, resp)
}
