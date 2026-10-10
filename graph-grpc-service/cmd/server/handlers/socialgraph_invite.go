package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"anahita-libs/constants"
	"anahita-libs/event"
	"anahita-libs/proto/graphpb"
	"graph/data/repositories"
)

// Invitations to follow an actor. Whether the viewer may invite, answer or
// take one back is asked by socialgraph-service before it calls; what the
// data allows of each person is the repository's.

func inviteToGrpc(invite *repositories.Invite) *graphpb.Invite {
	if invite == nil {
		return nil
	}

	return &graphpb.Invite{
		ActorId:   invite.ActorID,
		InviteeId: invite.InviteeID,
		InviterId: invite.InviterID,
		Status:    invite.Status,
		CreatedAt: invite.CreatedAt.Unix(),
		EndsAt:    invite.EndsAt.Unix(),
	}
}

// InviteAdd invites people to follow an actor.
func (h *socialgraphFollowGrpcHandler) InviteAdd(ctx context.Context, req *graphpb.InviteAddRequest) (*graphpb.InviteAddResponse, error) {
	if req.ActorId == 0 || req.Viewer.GetId() == 0 {
		return nil, errors.New("an actor and a viewer are required")
	}
	if len(req.InviteeIds) > constants.InviteMaxPerRequest {
		return nil, errors.New("too many people in one request")
	}

	actor, err := h.env.Repos.Nodes.Read(req.ActorId)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	made, err := h.env.Repos.Invites.CountMadeLately(req.Viewer.Id)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	if made+len(req.InviteeIds) > constants.InviteMaxPerDay {
		return &graphpb.InviteAddResponse{OverDailyLimit: true}, nil
	}

	// An event takes fewer waiting invitations than a group when it has a
	// limit on places, and none once it is over or called off.
	maxWaiting := uint64(constants.InviteMaxPending)
	if actor.Type == constants.NodeActorEventEntityType {
		events, err := h.env.Repos.Events.Read([]int64{actor.ID}, 0)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		details := events[actor.ID]
		if details == nil || details.CancelledAt != nil || !actor.EndsAt.After(time.Now()) {
			closed := map[int64]string{}
			for _, id := range req.InviteeIds {
				closed[id] = constants.InviteResultClosed
			}
			return &graphpb.InviteAddResponse{Results: closed}, nil
		}
		maxWaiting = constants.InviteMaxPendingFor(details.Capacity)
	}

	results, err := h.env.Repos.Invites.Add(actor, req.Viewer.Id, req.InviteeIds, maxWaiting)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	for inviteeID, result := range results {
		if result == constants.InviteResultInvited {
			h.publishInvite(event.EventFormatEdgeInviteAdded, &repositories.Invite{
				ActorID:   actor.ID,
				InviteeID: inviteeID,
				InviterID: req.Viewer.Id,
			}, req.Viewer.Id)
		}
	}

	return &graphpb.InviteAddResponse{Results: results}, nil
}

// InviteBrowse lists the invitations waiting to an actor, or for a person.
func (h *socialgraphFollowGrpcHandler) InviteBrowse(ctx context.Context, req *graphpb.InviteBrowseRequest) (*graphpb.InviteBrowseResponse, error) {
	var (
		invites []*repositories.Invite
		total   uint64
		err     error
	)

	limit := req.Limit
	if limit == 0 || limit > constants.MaxPageSize {
		limit = constants.DefaultPageSize
	}

	switch {
	case req.ActorId > 0:
		invites, total, err = h.env.Repos.Invites.ListForActor(req.ActorId, limit, req.Offset)
	case req.InviteeId > 0:
		invites, total, err = h.env.Repos.Invites.ListForInvitee(req.InviteeId, limit, req.Offset)
	default:
		return nil, errors.New("an actor or a person is required")
	}
	if err != nil {
		log.Println(err)
		return nil, err
	}

	resp := &graphpb.InviteBrowseResponse{Total: total}
	for _, invite := range invites {
		resp.Invites = append(resp.Invites, inviteToGrpc(invite))
	}

	return resp, nil
}

// InviteRead returns the invitation waiting for a person to an actor.
func (h *socialgraphFollowGrpcHandler) InviteRead(ctx context.Context, req *graphpb.InviteReadRequest) (*graphpb.InviteReadResponse, error) {
	invite, err := h.env.Repos.Invites.Read(req.ActorId, req.InviteeId)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.InviteReadResponse{Invite: inviteToGrpc(invite)}, nil
}

// InviteAnswer is the person invited saying yes or no.
//
// Yes makes them a follower, whatever the actor's access says: the
// invitation is the actor's consent. The follow is announced like any
// other, so whoever looks after the actor hears of the new follower.
func (h *socialgraphFollowGrpcHandler) InviteAnswer(ctx context.Context, req *graphpb.InviteAnswerRequest) (*graphpb.InviteAnswerResponse, error) {
	if req.ActorId == 0 || req.InviteeId == 0 {
		return nil, errors.New("an actor and a person are required")
	}

	if !req.Accept {
		invite, err := h.env.Repos.Invites.Decline(req.ActorId, req.InviteeId)
		if errors.Is(err, repositories.ErrNoInvite) {
			return &graphpb.InviteAnswerResponse{Found: false}, nil
		}
		if err != nil {
			log.Println(err)
			return nil, err
		}

		h.publishInvite(event.EventFormatEdgeInviteDeclined, invite, req.InviteeId)

		return &graphpb.InviteAnswerResponse{Found: true}, nil
	}

	invite, err := h.env.Repos.Invites.Accept(req.ActorId, req.InviteeId)
	if errors.Is(err, repositories.ErrNoInvite) {
		return &graphpb.InviteAnswerResponse{Found: false}, nil
	}
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if err = h.env.Repos.SocialGraphCounts.UpdateLeaderCount(req.InviteeId); err != nil {
		log.Println(err)
	}
	if err = h.env.Repos.SocialGraphCounts.UpdateFollowerCount(req.ActorId); err != nil {
		log.Println(err)
	}

	// Accepting an invitation to an event is saying you are going. When
	// every place is taken the follow stands as it was made, which counts
	// as maybe.
	if target, err := h.env.Repos.Nodes.Read(req.ActorId); err == nil && target.Type == constants.NodeActorEventEntityType {
		if _, err := h.env.Repos.Events.Rsvp(req.ActorId, req.InviteeId, constants.EventRsvpGoing); err != nil {
			log.Println(err)
		}
	}

	h.publishInvite(event.EventFormatEdgeInviteAccepted, invite, req.InviteeId)

	source, err := h.env.Repos.Nodes.Read(req.InviteeId)
	if err == nil {
		if target, err := h.env.Repos.Nodes.Read(req.ActorId); err == nil {
			h.publish(event.EventFormatEdgeFollowAdded, source, target, req.Viewer)
		}
	}

	return &graphpb.InviteAnswerResponse{Found: true}, nil
}

// InviteDelete takes an invitation back.
func (h *socialgraphFollowGrpcHandler) InviteDelete(ctx context.Context, req *graphpb.InviteDeleteRequest) (*graphpb.InviteDeleteResponse, error) {
	invite, err := h.env.Repos.Invites.Delete(req.ActorId, req.InviteeId)
	if errors.Is(err, repositories.ErrNoInvite) {
		return &graphpb.InviteDeleteResponse{Found: false}, nil
	}
	if err != nil {
		log.Println(err)
		return nil, err
	}

	h.publishInvite(event.EventFormatEdgeInviteDeleted, invite, req.Viewer.GetId())

	return &graphpb.InviteDeleteResponse{Found: true}, nil
}

// publishInvite tells the other services what happened to an invitation.
func (h *socialgraphFollowGrpcHandler) publishInvite(format string, invite *repositories.Invite, by int64) {
	if h.env.Publisher == nil || invite == nil {
		return
	}

	payload := InviteEvent(format, invite, by)

	go func() {
		log.Println("Publishing event", payload.Name)
		h.env.Publisher.Publish(payload, payload.Name)
	}()
}

// InviteEvent is the event for something that happened to an invitation.
// The sweep uses it too, for the ones that lapse.
func InviteEvent(format string, invite *repositories.Invite, by int64) *event.EventPayload {
	routingKey := fmt.Sprintf(format, "socialgraph")

	return &event.EventPayload{
		Name:          routingKey,
		SourceService: "graph-grpc-service",
		SubjectID:     by,
		ObjectID:      invite.InviteeID,
		TargetID:      invite.ActorID,
		OwnerID:       invite.InviterID,
		ViewerID:      by,
	}
}
