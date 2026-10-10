package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/grpctls"
	"anahita-libs/proto/graphpb"
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
)

var socialgraphFollowGRPCConnectCounts int

type SocialgraphFollowGRPCInterface interface {
	// Edit adds a new follower
	Edit(req *SocialgraphFollowEditRequest) error

	// Add adds a new follower
	Add(req *SocialgraphFollowAddRequest) error

	// Delete removes a follower
	Delete(req *SocialgraphFollowDeleteRequest) error

	// Invitations to follow an actor. See the methods.
	InviteAdd(actorID int64, inviteeIDs []int64, viewer *entities.Viewer) (map[int64]string, bool, error)
	InvitesForActor(actorID int64, limit uint64, offset uint64) ([]*Invite, uint64, error)
	InvitesForPerson(personID int64, limit uint64, offset uint64) ([]*Invite, uint64, error)
	InviteRead(actorID int64, personID int64) (*Invite, error)
	InviteAnswer(actorID int64, accept bool, viewer *entities.Viewer) (bool, error)
	InviteDelete(actorID int64, personID int64, viewer *entities.Viewer) (bool, error)
}

type SocialgraphFollowGRPC struct {
	config GraphGRPCConfigInterface
}

func NewSocialgraphFollowGRPC(config GraphGRPCConfigInterface) SocialgraphFollowGRPCInterface {
	return &SocialgraphFollowGRPC{
		config: config,
	}
}

// getClient returns the grpc client
func (srv *SocialgraphFollowGRPC) getClient() (graphpb.SocialgraphFollowServiceClient, error) {
	var opts []grpc.DialOption
	transport, err := grpctls.DialOption()
	if err != nil {
		return nil, err
	}
	opts = append(opts, transport)
	opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`))

	for {
		cc, err := grpc.NewClient(srv.config.GetGraphServiceAddr(), opts...)

		if err != nil {
			log.Println("Socialgraph Follow GRPC not ready...")
			socialgraphFollowGRPCConnectCounts++
		} else {
			log.Println("Connected to socialgraph follower GRPC!")
			return graphpb.NewSocialgraphFollowServiceClient(cc), nil
		}

		if socialgraphFollowGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// Edit adds a new follower
func (srv *SocialgraphFollowGRPC) Edit(req *SocialgraphFollowEditRequest) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Edit(context.Background(), &graphpb.SocialgraphFollowEditRequest{
		Source: actorNodeToSocialgraphSource(req.Source),
		Target: actorNodeToSocialgraphTarget(req.Target),
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
		AcceptFollowRequest: req.AcceptFollowRequest,
	})
	if err != nil {
		return err
	}

	return nil
}

// Add adds a new follower
func (srv *SocialgraphFollowGRPC) Add(req *SocialgraphFollowAddRequest) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Add(context.Background(), &graphpb.SocialgraphFollowAddRequest{
		Source: actorNodeToSocialgraphSource(req.Source),
		Target: actorNodeToSocialgraphTarget(req.Target),
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
		IsFollowRequest: req.IsFollowRequest,
	})
	if err != nil {
		return err
	}

	return nil
}

// Delete removes a follower
func (srv *SocialgraphFollowGRPC) Delete(req *SocialgraphFollowDeleteRequest) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Delete(context.Background(), &graphpb.SocialgraphFollowDeleteRequest{
		Source: actorNodeToSocialgraphSource(req.Source),
		Target: actorNodeToSocialgraphTarget(req.Target),
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
		IsFollowRequest: req.IsFollowRequest,
	})
	if err != nil {
		return err
	}

	return nil
}

// Invite is an invitation to follow an actor that is waiting for an answer.
type Invite struct {
	ActorID   int64
	InviteeID int64
	InviterID int64
	Status    string
	CreatedAt time.Time
	EndsAt    time.Time
}

func grpcToInvite(invite *graphpb.Invite) *Invite {
	if invite == nil {
		return nil
	}

	return &Invite{
		ActorID:   invite.ActorId,
		InviteeID: invite.InviteeId,
		InviterID: invite.InviterId,
		Status:    invite.Status,
		CreatedAt: time.Unix(invite.CreatedAt, 0).UTC(),
		EndsAt:    time.Unix(invite.EndsAt, 0).UTC(),
	}
}

// InviteAdd invites people to follow an actor, and says what became of
// each. The second answer is true when the inviter has made too many
// lately, and then nobody was invited.
func (srv *SocialgraphFollowGRPC) InviteAdd(actorID int64, inviteeIDs []int64, viewer *entities.Viewer) (map[int64]string, bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, false, err
	}

	resp, err := client.InviteAdd(context.Background(), &graphpb.InviteAddRequest{
		ActorId:    actorID,
		InviteeIds: inviteeIDs,
		Viewer:     viewerToGrpc(viewer),
	})
	if err != nil {
		return nil, false, err
	}

	return resp.Results, resp.OverDailyLimit, nil
}

// InvitesForActor lists the invitations waiting to an actor.
func (srv *SocialgraphFollowGRPC) InvitesForActor(actorID int64, limit uint64, offset uint64) ([]*Invite, uint64, error) {
	return srv.inviteBrowse(&graphpb.InviteBrowseRequest{ActorId: actorID, Limit: limit, Offset: offset})
}

// InvitesForPerson lists the invitations waiting for a person.
func (srv *SocialgraphFollowGRPC) InvitesForPerson(personID int64, limit uint64, offset uint64) ([]*Invite, uint64, error) {
	return srv.inviteBrowse(&graphpb.InviteBrowseRequest{InviteeId: personID, Limit: limit, Offset: offset})
}

func (srv *SocialgraphFollowGRPC) inviteBrowse(req *graphpb.InviteBrowseRequest) ([]*Invite, uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, 0, err
	}

	resp, err := client.InviteBrowse(context.Background(), req)
	if err != nil {
		return nil, 0, err
	}

	invites := make([]*Invite, 0, len(resp.Invites))
	for _, invite := range resp.Invites {
		invites = append(invites, grpcToInvite(invite))
	}

	return invites, resp.Total, nil
}

// InviteRead returns the invitation waiting for a person to an actor, or
// nil.
func (srv *SocialgraphFollowGRPC) InviteRead(actorID int64, personID int64) (*Invite, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.InviteRead(context.Background(), &graphpb.InviteReadRequest{ActorId: actorID, InviteeId: personID})
	if err != nil {
		return nil, err
	}

	return grpcToInvite(resp.Invite), nil
}

// InviteAnswer accepts or declines the viewer's invitation to an actor. It
// answers false when there was none waiting.
func (srv *SocialgraphFollowGRPC) InviteAnswer(actorID int64, accept bool, viewer *entities.Viewer) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}

	resp, err := client.InviteAnswer(context.Background(), &graphpb.InviteAnswerRequest{
		ActorId:   actorID,
		InviteeId: viewer.ID,
		Accept:    accept,
		Viewer:    viewerToGrpc(viewer),
	})
	if err != nil {
		return false, err
	}

	return resp.Found, nil
}

// InviteDelete takes back a person's invitation to an actor. It answers
// false when there was none waiting.
func (srv *SocialgraphFollowGRPC) InviteDelete(actorID int64, personID int64, viewer *entities.Viewer) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}

	resp, err := client.InviteDelete(context.Background(), &graphpb.InviteDeleteRequest{
		ActorId:   actorID,
		InviteeId: personID,
		Viewer:    viewerToGrpc(viewer),
	})
	if err != nil {
		return false, err
	}

	return resp.Found, nil
}
