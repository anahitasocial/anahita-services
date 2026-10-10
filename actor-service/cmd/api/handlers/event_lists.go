package handlers

import (
	"log"
	"net/http"

	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

type eventMineRequest struct {
	// upcoming, past, hosting or invited. Upcoming when left out.
	Filter string `query:"filter" validate:"omitempty,oneof=upcoming past hosting invited"`
	Offset uint64 `query:"start"`
	Limit  uint64 `query:"limit" validate:"omitempty,max=100"`
}

type eventAttendeesRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
	// going or maybe. Going when left out.
	Rsvp   string `query:"rsvp" validate:"omitempty,oneof=going maybe"`
	Offset uint64 `query:"start"`
	Limit  uint64 `query:"limit" validate:"omitempty,max=100"`
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
		req    = &eventMineRequest{Limit: an_constants.RequestLimitDefault}
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

	resp := &struct {
		Data       []any                `json:"data"`
		Pagination responses.Pagination `json:"pagination"`
	}{
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

	actorPerms := permissions.NewActorPermissions()
	actorResp := &responses.ActorResponse{}

	for _, event := range ordered {
		rel := rels[event.ID]
		if rel == nil {
			rel = &entities.ActorRelation{}
		}

		canRead, _ := actorPerms.CanRead(&permissions.ActorAccessContext{
			Viewer:         viewer,
			Actor:          event,
			ActorViewerRel: rel,
		})
		if !canRead {
			// Invited to it and not otherwise let in. An event that is
			// neither is left out: it has stopped being theirs.
			if !permissions.CanSeeLimitedProfile(viewer, event, rel) {
				continue
			}
			limited := responses.NewLimitedActor(event, rel.HasRequested)
			limited.IsInvited = rel.HasInvite
			if rel.HasInvite {
				event.EventShowsOnlineURL = false
				limited.Event = responses.NewEvent(event)
			}
			resp.Data = append(resp.Data, limited)
			continue
		}

		event.IsLeadingViewer = rel.IsLeading
		event.IsAdminedByViewer = rel.HasAdmin
		event.IsInvitedViewer = rel.HasInvite
		resp.Data = append(resp.Data, actorResp.MapNodeItem(event))
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
		req      = &eventAttendeesRequest{Limit: an_constants.RequestLimitDefault}
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

	resp := &struct {
		Data       []*responses.ActorNodeItem `json:"data"`
		Pagination responses.Pagination       `json:"pagination"`
	}{
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
