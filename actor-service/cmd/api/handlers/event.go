package handlers

import (
	"actor/constants"
	"log"
	"net/http"
	"time"
	// Every zone's rules, built into the program. The image this runs in
	// has no zone database of its own, and without one no zone by name can
	// be found: every event was refused for its time zone, whichever it
	// was.
	_ "time/tzdata"

	"anahita-libs/cmd/api/permissions"
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/responses"
	"anahita-libs/validators"

	"github.com/labstack/echo/v4"
)

// An event is an actor like a group: followed, posted on, looked after by
// administrators. So it is served by the same handler, told a different
// kind, and everything a group can do an event can: its access, features,
// administrators, archive, delete.
//
// What only an event has is here: when it is, how many may go, where to
// join online, answering it, and calling it off.

type eventHandler struct {
	*groupHandler
}

// NewEventHandler serves events.
func NewEventHandler(env *Env) *eventHandler {
	return &eventHandler{
		groupHandler: &groupHandler{
			Env:      env,
			nodeType: an_constants.NodeActorEventEntityType,
			service:  eventService,
		},
	}
}

// The service name stored on an event's node. An event was born in this
// service, and is named for it.
const eventService = "actor-service"

// The longest an event may run.
const eventLongest = 366 * 24 * time.Hour

// eventFields is what is given when an event is made or changed, beside
// the name and description every actor has.
type eventFields struct {
	Name       string `form:"name" json:"name" validate:"required,max=100"`
	Body       string `form:"body" json:"body" validate:"omitempty,max=10000"`
	WebsiteURL string `form:"website_url" json:"website_url" validate:"omitempty,http_url,max=255"`
	// When, as RFC 3339 with an offset: 2026-11-05T19:00:00-05:00.
	StartsAt string `form:"starts_at" json:"starts_at" validate:"required,max=40"`
	EndsAt   string `form:"ends_at" json:"ends_at" validate:"required,max=40"`
	// The zone those times were chosen in, as IANA names it.
	TimezoneName string `form:"timezone_name" json:"timezone_name" validate:"required,max=64"`
	// How many may be going. 0, or left out, for no limit.
	Capacity uint32 `form:"capacity" json:"capacity" validate:"omitempty,max=100000"`
	// Where to join, when it is online.
	OnlineURL string `form:"online_url" json:"online_url" validate:"omitempty,http_url,max=512"`
}

type eventAddRequest struct {
	eventFields
	// Who can see it. Anybody, unless it says otherwise.
	Access string `form:"access" json:"access" validate:"omitempty,oneof=public registered followers"`
	// A group hosting it, which whoever makes the event must administer.
	// Left out for an event that is its maker's own.
	HostGroupID int64 `form:"host_group_id" json:"host_group_id" validate:"omitempty,numeric"`
}

type eventEditRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
	eventFields
}

type eventIDRequest struct {
	ID int64 `param:"id" validate:"required,numeric"`
}

type eventRsvpRequest struct {
	ID   int64  `param:"id" validate:"required,numeric"`
	Rsvp string `form:"rsvp" json:"rsvp" validate:"required,oneof=going maybe"`
}

// refuseEvent answers a request that cannot be done with the reason, as a
// word the web app has wording for.
func refuseEvent(c echo.Context, status int, reason string) error {
	return c.JSON(status, map[string]string{"error": reason})
}

// eventTimes reads when an event is, and says why not when it cannot be:
// "invalid_time", "ends_before_start", "too_long", "invalid_timezone".
func eventTimes(fields *eventFields) (time.Time, time.Time, string) {
	if _, err := time.LoadLocation(fields.TimezoneName); err != nil || fields.TimezoneName == "Local" {
		return time.Time{}, time.Time{}, "invalid_timezone"
	}

	startsAt, err := time.Parse(time.RFC3339, fields.StartsAt)
	if err != nil {
		return time.Time{}, time.Time{}, "invalid_time"
	}
	endsAt, err := time.Parse(time.RFC3339, fields.EndsAt)
	if err != nil {
		return time.Time{}, time.Time{}, "invalid_time"
	}

	if !endsAt.After(startsAt) {
		return time.Time{}, time.Time{}, "ends_before_start"
	}
	if endsAt.Sub(startsAt) > eventLongest {
		return time.Time{}, time.Time{}, "too_long"
	}

	return startsAt.UTC(), endsAt.UTC(), ""
}

// Add makes an event. Whoever makes it looks after it, and is going.
//
// POST /events/
func (h *eventHandler) Add(c echo.Context) error {
	var (
		req    = &eventAddRequest{}
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	req.WebsiteURL = validators.NormalizeWebsiteURL(req.WebsiteURL)
	req.OnlineURL = validators.NormalizeWebsiteURL(req.OnlineURL)

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	startsAt, endsAt, reason := eventTimes(&req.eventFields)
	if reason != "" {
		return refuseEvent(c, http.StatusBadRequest, reason)
	}
	// An event that is already over is not one anybody can go to.
	if !endsAt.After(time.Now()) {
		return refuseEvent(c, http.StatusBadRequest, "already_over")
	}

	// Hosted by a group: only by somebody who administers that group.
	var host *entities.AnahitaNode
	if req.HostGroupID > 0 {
		var status int
		if host, status = h.hostGroup(req.HostGroupID, viewer); status != 0 {
			return refuseEvent(c, status, "host_not_allowed")
		}
	}

	actor := entities.NewAnahitaNode()
	actor.Access = an_constants.AnahitaAccessPublic
	if req.Access != "" {
		actor.Access = req.Access
	}
	actor.Name = req.Name
	actor.Body = req.Body
	actor.WebsiteURL = req.WebsiteURL
	actor.Enabled = true
	actor.StartsAt = startsAt
	actor.EndsAt = endsAt

	actor, status := h.create(actor, viewer)
	if status != 0 {
		return c.NoContent(status)
	}

	details := &entities.EventDetails{
		NodeID:       actor.ID,
		TimezoneName: req.TimezoneName,
		Capacity:     req.Capacity,
		OnlineURL:    req.OnlineURL,
	}
	if host != nil {
		details.HostID = host.ID
	}
	if err := h.Srv.EventsGRPC.Save(details); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if host != nil {
		h.carryOverAdmins(host, actor, viewer)
	}

	// Whoever makes it is going. They follow it already; this says how.
	if _, err := h.Srv.EventsGRPC.Rsvp(actor.ID, viewer.ID, an_constants.EventRsvpGoing); err != nil {
		log.Println(err)
	}

	return h.respond(c, actor.ID, viewer, http.StatusCreated)
}

// Edit changes an event: what it is called, when it is, how many may go.
//
// PATCH /events/:id
func (h *eventHandler) Edit(c echo.Context) error {
	var (
		req    = &eventEditRequest{}
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	req.WebsiteURL = validators.NormalizeWebsiteURL(req.WebsiteURL)
	req.OnlineURL = validators.NormalizeWebsiteURL(req.OnlineURL)

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	startsAt, endsAt, reason := eventTimes(&req.eventFields)
	if reason != "" {
		return refuseEvent(c, http.StatusBadRequest, reason)
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
	if ok, err := h.Perms.Actors.CanEdit(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	// Whether when it is is being changed, which is what whoever answered
	// has to hear of. A new name or description is not.
	timesChanged := !actor.StartsAt.Equal(startsAt) || !actor.EndsAt.Equal(endsAt)

	// Its address is left as it was when it was made: an event is linked to
	// from invitations and calendars, and renaming it should not break them.
	actor.Name = req.Name
	actor.Body = req.Body
	actor.WebsiteURL = req.WebsiteURL
	actor.StartsAt = startsAt
	actor.EndsAt = endsAt
	actor.ModifiedBy = viewer.ID
	if _, err = h.Srv.ActorsGRPC.Edit(actor, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.Srv.EventsGRPC.Save(&entities.EventDetails{
		NodeID:       actor.ID,
		TimezoneName: req.TimezoneName,
		Capacity:     req.Capacity,
		OnlineURL:    req.OnlineURL,
	}); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if timesChanged {
		if err := h.Srv.EventsGRPC.AnnounceChange(actor.ID, viewer.ID); err != nil {
			log.Println(err)
		}
	}

	return h.respond(c, actor.ID, viewer, http.StatusOK)
}

// Rsvp is the viewer saying they are going, or that they may.
//
// PUT /events/:id/rsvp  {"rsvp": "going" | "maybe"}
//
// 409 with "event_full" when every place is taken, and 410 with
// "event_closed" when it is over or was called off.
func (h *eventHandler) Rsvp(c echo.Context) error {
	var (
		req    = &eventRsvpRequest{}
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

	if !viewer.HasScope(an_constants.OAuthScopeSocialgraphWrite) {
		return c.NoContent(http.StatusForbidden)
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
	rel := actorAccessCtx.ActorViewerRel

	// The same answer for an event that is not there and for one the viewer
	// may not answer, so that a private event cannot be found by trying.
	if !permissions.CanRsvp(viewer, actor, rel) {
		return c.NoContent(http.StatusNotFound)
	}

	result, err := h.Srv.EventsGRPC.Rsvp(actor.ID, viewer.ID, req.Rsvp)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	switch result {
	case an_constants.EventRsvpFull:
		return refuseEvent(c, http.StatusConflict, "event_full")
	case an_constants.EventRsvpClosed:
		return refuseEvent(c, http.StatusGone, "event_closed")
	case an_constants.EventRsvpMissing:
		return c.NoContent(http.StatusNotFound)
	}

	// An invitation that was waiting has been answered by this. Accepting
	// it removes it and tells whoever sent it.
	if rel != nil && rel.HasInvite {
		if _, err := h.Srv.SocialgraphFollowGRPC.InviteAnswer(actor.ID, true, viewer); err != nil {
			log.Println(err)
		}
	}

	return h.respond(c, actor.ID, viewer, http.StatusOK)
}

// Unrsvp takes the viewer's answer back: they are no longer going, or
// maybe going. Always theirs to do, whatever has become of the event.
//
// DELETE /events/:id/rsvp
func (h *eventHandler) Unrsvp(c echo.Context) error {
	var (
		req    = &eventIDRequest{}
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

	if !viewer.HasScope(an_constants.OAuthScopeSocialgraphWrite) {
		return c.NoContent(http.StatusForbidden)
	}

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if _, err := h.Srv.EventsGRPC.Rsvp(actor.ID, viewer.ID, ""); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusNoContent)
}

// Cancel calls an event off. It stays where it is, saying so.
//
// POST /events/:id/cancel
func (h *eventHandler) Cancel(c echo.Context) error {
	var (
		req    = &eventIDRequest{}
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
	if !permissions.CanCancelEvent(viewer, actor, actorAccessCtx.ActorViewerRel) {
		return c.NoContent(http.StatusForbidden)
	}

	if _, err := h.Srv.EventsGRPC.Cancel(actor.ID, viewer.ID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return h.respond(c, actor.ID, viewer, http.StatusOK)
}

// respond sends the event as it now is, the way reading it does.
func (h *eventHandler) respond(c echo.Context, id int64, viewer *entities.Viewer, status int) error {
	actor, err := h.Srv.ActorsGRPC.ReadOfType(id, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if err = h.onAfterFetched(c, actor); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{actor})
	batchPresignPaths(h.Env, actor.Administrators)

	actorResp := &responses.ActorResponse{}

	return c.JSON(status, actorResp.MapNode(actor))
}

// setEventDetails attaches to each event what only an event has, and says
// whether this viewer may be shown where to join it online.
//
// For one event the relation is read to answer that. A failure to read
// leaves the events without their details, logged: the page still shows
// what the event is called.
func (h *groupHandler) setEventDetails(events []*entities.AnahitaNode, viewer *entities.Viewer) {
	if len(events) == 0 {
		return
	}

	ids := make([]int64, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}

	details, err := h.Srv.EventsGRPC.Read(ids, viewer.ID)
	if err != nil {
		log.Println("event details:", err)
		return
	}

	rels, err := h.Srv.SocialgraphGRPC.Browse(ids, viewer.ID)
	if err != nil {
		log.Println("event details: relations:", err)
		rels = map[int64]*entities.ActorRelation{}
	}

	for _, event := range events {
		event.Event = details[event.ID]
		event.EventShowsOnlineURL = permissions.CanSeeOnlineURL(viewer, event, rels[event.ID], event.Event)
	}

	h.setEventHosts(events, viewer)
}

// setEventHosts names the group hosting each event, where the viewer may
// see that group. A host they may not see is left unnamed: the event does
// not give away a group that is not theirs to know about.
func (h *groupHandler) setEventHosts(events []*entities.AnahitaNode, viewer *entities.Viewer) {
	hosts := map[int64]*entities.AnahitaNode{}
	for _, event := range events {
		if event.Event != nil && event.Event.HostID > 0 {
			hosts[event.Event.HostID] = nil
		}
	}
	if len(hosts) == 0 {
		return
	}

	ids := make([]int64, 0, len(hosts))
	for id := range hosts {
		ids = append(ids, id)
	}

	rels, err := h.Srv.SocialgraphGRPC.Browse(ids, viewer.ID)
	if err != nil {
		log.Println("event hosts: relations:", err)
		return
	}

	actorPerms := permissions.NewActorPermissions()
	shown := []*entities.AnahitaNode{}
	for _, id := range ids {
		group, err := h.Srv.ActorsGRPC.Read(id, viewer)
		if err != nil || group == nil {
			continue
		}
		rel := rels[id]
		if rel == nil {
			rel = &entities.ActorRelation{}
		}
		if ok, _ := actorPerms.CanRead(&permissions.ActorAccessContext{
			Viewer:         viewer,
			Actor:          group,
			ActorViewerRel: rel,
		}); !ok {
			continue
		}
		hosts[id] = group
		shown = append(shown, group)
	}

	batchPresignPaths(h.Env, shown)

	for _, event := range events {
		if event.Event != nil {
			event.Event.Host = hosts[event.Event.HostID]
		}
	}
}

// hostGroup reads the group that is to host an event, for a viewer who
// administers it. The status is 0 when they do.
func (h *eventHandler) hostGroup(groupID int64, viewer *entities.Viewer) (*entities.AnahitaNode, int) {
	group, err := h.Srv.ActorsGRPC.ReadOfType(groupID, constants.NodeActorGroupType, viewer)
	if err != nil || group == nil {
		log.Println(err)
		return nil, http.StatusNotFound
	}

	rel, err := h.Srv.SocialgraphGRPC.Read(group.ID, viewer.ID)
	if err != nil {
		log.Println(err)
		return nil, http.StatusInternalServerError
	}

	if !viewer.IsAdmin() && (rel == nil || !rel.HasAdmin) {
		return nil, http.StatusForbidden
	}

	return group, 0
}

// carryOverAdmins makes the administrators of the hosting group
// administrators of the event as well.
//
// They are the event's own administrators from then on. Nothing is asked of
// the group afterwards: somebody who later joins or leaves the group's
// administrators is added to or taken off the event by hand, like anybody
// else. That was the user's choice, and it keeps "who looks after this
// actor" one question with one answer, everywhere it is asked.
//
// Whoever made the event is one already. A failure for one of them is
// logged and the others are still added: the event exists, and an
// administrator can add whoever is missing.
func (h *eventHandler) carryOverAdmins(group *entities.AnahitaNode, event *entities.AnahitaNode, viewer *entities.Viewer) {
	admins, err := h.Srv.ActorAdminsGRPC.Browse(group.ID)
	if err != nil {
		log.Println("event admins: reading the group's:", err)
		return
	}

	for _, admin := range admins {
		if admin == nil || admin.ID == viewer.ID {
			continue
		}
		if _, err := h.Srv.ActorAdminsGRPC.Add(event, admin, viewer); err != nil {
			log.Printf("event admins: adding %d to event %d: %v", admin.ID, event.ID, err)
		}
	}
}
