package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var socialgraphBlocksGRPCConnectCounts int

type SocialgraphBlockGRPCInterface interface {
	// Add adds a new block edge
	Add(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) error

	// Delete removes a block edge
	Delete(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) error
}

type SocialgraphBlockGRPC struct {
	config GraphGRPCConfigInterface
}

func NewSocialgraphBlockGRPC(config GraphGRPCConfigInterface) SocialgraphBlockGRPCInterface {
	return &SocialgraphBlockGRPC{
		config: config,
	}
}

// getClient returns the grpc client
func (srv *SocialgraphBlockGRPC) getClient() (graphpb.SocialgraphBlockServiceClient, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`))

	for {
		cc, err := grpc.NewClient(srv.config.GetGraphServiceAddr(), opts...)

		if err != nil {
			log.Println("Socialgraph Block GRPC not ready...")
			socialgraphBlocksGRPCConnectCounts++
		} else {
			log.Println("Connected to socialgraph follower GRPC!")
			return graphpb.NewSocialgraphBlockServiceClient(cc), nil
		}

		if socialgraphBlocksGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// Add adds a new block
func (srv *SocialgraphBlockGRPC) Add(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Add(context.Background(), &graphpb.SocialgraphBlockAddRequest{
		Source: actorNodeToSocialgraphSource(source),
		Target: actorNodeToSocialgraphTarget(target),
		Viewer: &graphpb.Viewer{
			Id:       viewer.ID,
			UserType: viewer.UserType,
			Email:    viewer.Email,
			Username: viewer.Username,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// Delete removes a block
func (srv *SocialgraphBlockGRPC) Delete(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Delete(context.Background(), &graphpb.SocialgraphBlockDeleteRequest{
		Source: actorNodeToSocialgraphSource(source),
		Target: actorNodeToSocialgraphTarget(target),
		Viewer: &graphpb.Viewer{
			Id:       viewer.ID,
			UserType: viewer.UserType,
			Email:    viewer.Email,
			Username: viewer.Username,
		},
	})
	if err != nil {
		return err
	}

	return nil
}
