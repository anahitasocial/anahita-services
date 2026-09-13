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

var likesGRPCConnectCounts int

type LikesGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

type LikesGRPCInterface interface {
	Add(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) error
	Delete(source *entities.AnahitaNode, target *entities.AnahitaNode) error
	IsLiked(source int64, targetID int64) (bool, error)
	IsLikedMany(sourceID int64, targetIDs []int64) (map[int64]bool, error)
}

type LikesGRPC struct {
	config LikesGRPCConfigInterface
}

func NewLikesGRPC(config LikesGRPCConfigInterface) LikesGRPCInterface {
	return &LikesGRPC{
		config: config,
	}
}

// getClient
func (srv *LikesGRPC) getClient() (graphpb.LikeServiceClient, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`))

	for {
		cc, err := grpc.NewClient(srv.config.GetGraphServiceAddr(), opts...)

		if err != nil {
			log.Println("Like GRPC not ready...")
			likesGRPCConnectCounts++
		} else {
			log.Println("Connected to Like GRPC!")
			return graphpb.NewLikeServiceClient(cc), nil
		}

		if likesGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// Add
func (srv *LikesGRPC) Add(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Add(context.Background(), &graphpb.LikeAddRequest{
		Source: &graphpb.Source{
			Id:   source.ID,
			Type: source.Type,
		},
		Target: &graphpb.Target{
			Id:   target.ID,
			Type: target.Type,
		},
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// Delete
func (srv *LikesGRPC) Delete(source *entities.AnahitaNode, target *entities.AnahitaNode) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Delete(context.Background(), &graphpb.LikeDeleteRequest{
		Source: &graphpb.Source{
			Id:   source.ID,
			Type: source.Type,
		},
		Target: &graphpb.Target{
			Id:   target.ID,
			Type: target.Type,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// IsLiked
func (srv *LikesGRPC) IsLiked(sourceID int64, targetID int64) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}

	resp, err := client.IsLiked(context.Background(), &graphpb.LikeIsLikedRequest{
		SourceId: sourceID,
		TargetId: targetID,
	})
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}

	return resp.IsLiked, nil
}

// IsLikedMany
func (srv *LikesGRPC) IsLikedMany(sourceID int64, targetIDs []int64) (map[int64]bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.IsLikedMany(context.Background(), &graphpb.LikeIsLikedManyRequest{
		SourceId:  sourceID,
		TargetIds: targetIDs,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	results := make(map[int64]bool)
	for _, item := range resp.Results {
		results[item.Id] = item.IsLiked
	}

	return results, nil
}
