package services

import (
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
	})
	if err != nil {
		return err
	}

	return nil
}
