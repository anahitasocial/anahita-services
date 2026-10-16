package handlers

import (
	"log"
	"net/http"
	"time"

	"anahita-libs/cmd/api/permissions"
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// Invitations: asking people to follow a group. Whoever is invited is told,
// and says yes or no. Nobody is made a follower without saying yes.

type inviteHandler struct {
	*Env
}

func NewInviteHandler(env *Env) *inviteHandler {
	return &inviteHandler{env}
}

type InviteAddRequest struct {
	// The group's or the event's id.
	ActorID int64 `param:"actor_id" validate:"required,numeric"`
	// Who to invite: people who follow the viewer.
	PersonIDs []int64 `json:"person_ids" validate:"required,min=1,dive,required,numeric"`
}

type InviteActorRequest struct {
	// The group's or the event's id.
	ActorID int64 `param:"actor_id" validate:"required,numeric"`
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit" validate:"omitempty,max=100"`
}

type InvitePersonRequest struct {
	// The group's or the event's id.
	ActorID int64 `param:"actor_id" validate:"required,numeric"`
	// The person invited.
	PersonID int64 `param:"person_id" validate:"required,numeric"`
}

type InviteMineRequest struct {
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit" validate:"omitempty,max=100"`
}

// InviteResult is what became of one person invited.
type InviteResult struct {
	// Who.
	PersonID int64 `json:"person_id"`
	// `invited`; or why not: `already_follower`, `not_follower`, `blocked`,
	// `cooldown`, `pending`, `requested`, `unavailable`, `full` or
	// `closed`.
	Result string `json:"result"`
}

// InviteAnswer is the viewer's answer to an invitation.
type InviteAnswer struct {
	// Whether they accepted it: they follow now.
	Accepted bool `json:"accepted"`
}

// InviteItem is one invitation as it is sent: who, to what, by whom, and
// until when.
type InviteItem struct {
	// What they are invited to follow.
	Actor *responses.ActorBrief `json:"actor"`
	// Who is invited.
	Person *responses.ActorBrief `json:"person"`
	// Who invited them.
	Inviter *responses.ActorBrief `json:"inviter"`
	// When.
	CreatedAt time.Time `json:"created_at"`
	// When it lapses, unanswered.
	EndsAt time.Time `json:"ends_at"`
}

// Add invites people to follow an actor.
//
// POST /socialgraph/:actor_id/invites  {"person_ids": [..]}
//
// Answers with what became of each person, in the order they were named.
// Somebody who cannot be invited does not stop the others.
func (h *inviteHandler) Add(c echo.Context) error {
	var (
		req    = &InviteAddRequest{}
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
	if len(req.PersonIDs) > constants.InviteMaxPerRequest {
		return c.NoContent(http.StatusBadRequest)
	}

	if !viewer.HasScope(constants.OAuthScopeSocialgraphWrite) {
		return c.NoContent(http.StatusForbidden)
	}

	actor, rel, status := h.actorAndRelation(req.ActorID, viewer)
	if status != 0 {
		return c.NoContent(status)
	}

	if !permissions.CanInvite(viewer, actor, rel) {
		return c.NoContent(http.StatusForbidden)
	}

	results, overLimit, err := h.Srv.SocialgraphFollowGRPC.InviteAdd(actor.ID, req.PersonIDs, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if overLimit {
		return c.NoContent(http.StatusTooManyRequests)
	}

	resp := responses.Unpaged[InviteResult]{Data: []InviteResult{}}

	seen := map[int64]bool{}
	for _, id := range req.PersonIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		resp.Data = append(resp.Data, InviteResult{PersonID: id, Result: results[id]})
	}

	return c.JSON(http.StatusOK, resp)
}

// Browse lists who has been invited to an actor and is yet to answer, for
// whoever looks after it.
//
// GET /socialgraph/:actor_id/invites/
func (h *inviteHandler) Browse(c echo.Context) error {
	var (
		req    = &InviteActorRequest{Limit: constants.RequestLimitDefault}
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

	if !viewer.HasScope(constants.OAuthScopeSocialgraphRead) {
		return c.NoContent(http.StatusForbidden)
	}

	actor, rel, status := h.actorAndRelation(req.ActorID, viewer)
	if status != 0 {
		return c.NoContent(status)
	}

	if !permissions.CanSeeInvites(viewer, actor, rel) {
		return c.NoContent(http.StatusForbidden)
	}

	invites, total, err := h.Srv.SocialgraphFollowGRPC.InvitesForActor(actor.ID, req.Limit, req.Offset)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, h.list(invites, total, req.Limit, req.Offset, viewer))
}

// Mine lists the invitations waiting for the viewer.
//
// GET /socialgraph/invites/
func (h *inviteHandler) Mine(c echo.Context) error {
	var (
		req    = &InviteMineRequest{Limit: constants.RequestLimitDefault}
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

	if !viewer.HasScope(constants.OAuthScopeSocialgraphRead) {
		return c.NoContent(http.StatusForbidden)
	}

	invites, total, err := h.Srv.SocialgraphFollowGRPC.InvitesForPerson(viewer.ID, req.Limit, req.Offset)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, h.list(invites, total, req.Limit, req.Offset, viewer))
}

// Accept and Decline are the viewer answering their own invitation to an
// actor. Nobody answers for anybody else: the invitation looked for is the
// viewer's.
//
// POST /socialgraph/:actor_id/invites/accept
// POST /socialgraph/:actor_id/invites/decline
//
// 404 when there is none waiting: never made, answered already, taken back
// or lapsed.
func (h *inviteHandler) Accept(c echo.Context) error {
	return h.answer(c, true)
}

func (h *inviteHandler) Decline(c echo.Context) error {
	return h.answer(c, false)
}

func (h *inviteHandler) answer(c echo.Context, accept bool) error {
	var (
		req    = &InviteActorRequest{}
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

	if !viewer.HasScope(constants.OAuthScopeSocialgraphWrite) {
		return c.NoContent(http.StatusForbidden)
	}

	found, err := h.Srv.SocialgraphFollowGRPC.InviteAnswer(req.ActorID, accept, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if !found {
		return c.NoContent(http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, InviteAnswer{Accepted: accept})
}

// Delete takes an invitation back: whoever made it, or whoever looks after
// the actor.
//
// DELETE /socialgraph/:actor_id/invites/:person_id
func (h *inviteHandler) Delete(c echo.Context) error {
	var (
		req    = &InvitePersonRequest{}
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

	if !viewer.HasScope(constants.OAuthScopeSocialgraphWrite) {
		return c.NoContent(http.StatusForbidden)
	}

	actor, rel, status := h.actorAndRelation(req.ActorID, viewer)
	if status != 0 {
		return c.NoContent(status)
	}

	invite, err := h.Srv.SocialgraphFollowGRPC.InviteRead(actor.ID, req.PersonID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	// The same answer for an invitation that is not there and for one that
	// is not the viewer's to take back, so that who has been invited cannot
	// be found out by trying.
	if invite == nil || !permissions.CanWithdrawInvite(viewer, actor, rel, invite.InviterID) {
		return c.NoContent(http.StatusNotFound)
	}

	if _, err = h.Srv.SocialgraphFollowGRPC.InviteDelete(actor.ID, req.PersonID, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusNoContent)
}

// actorAndRelation reads the actor and how the viewer stands with it. The
// status is 0 when both were read.
func (h *inviteHandler) actorAndRelation(actorID int64, viewer *entities.Viewer) (*entities.AnahitaNode, *entities.ActorRelation, int) {
	actor, err := h.Srv.ActorsGRPC.Read(actorID, viewer)
	if err != nil || actor == nil {
		log.Println(err)
		return nil, nil, http.StatusNotFound
	}

	rel, err := h.Srv.SocialgraphGRPC.Read(actor.ID, viewer.ID)
	if err != nil {
		log.Println(err)
		return nil, nil, http.StatusInternalServerError
	}

	return actor, rel, 0
}

// list fills in who and what each invitation names.
//
// Each is sent as a picture and a name. The actor may be one the viewer
// cannot see yet, which is the point of having been invited to it, and
// that much of it they are shown.
func (h *inviteHandler) list(invites []*services.Invite, total uint64, limit uint64, offset uint64, viewer *entities.Viewer) any {
	resp := responses.List[*InviteItem]{
		Data: []*InviteItem{},
		Pagination: responses.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  total,
		},
	}

	known := map[int64]*entities.AnahitaNode{}
	read := func(id int64) *entities.AnahitaNode {
		if node, ok := known[id]; ok {
			return node
		}
		node, err := h.Srv.ActorsGRPC.Read(id, viewer)
		if err != nil {
			log.Println(err)
			node = nil
		}
		known[id] = node
		return node
	}

	for _, invite := range invites {
		read(invite.ActorID)
		read(invite.InviteeID)
		read(invite.InviterID)
	}

	nodes := make([]*entities.AnahitaNode, 0, len(known))
	for _, node := range known {
		if node != nil {
			nodes = append(nodes, node)
		}
	}
	onAfterActorFetched(h.Env, nodes)

	for _, invite := range invites {
		resp.Data = append(resp.Data, &InviteItem{
			Actor:     responses.NewActorBrief(known[invite.ActorID]),
			Person:    responses.NewActorBrief(known[invite.InviteeID]),
			Inviter:   responses.NewActorBrief(known[invite.InviterID]),
			CreatedAt: invite.CreatedAt,
			EndsAt:    invite.EndsAt,
		})
	}

	return resp
}
