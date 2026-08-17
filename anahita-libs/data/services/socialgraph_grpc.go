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

var socialgraphGRPCConnectCounts int

type SocialgraphGRPCInterface interface {
	// Browse gets relationships for a list of actorIds vs. an actorId
	Browse(actorAIds []int64, actorBId int64) (map[int64]*entities.ActorRelation, error)
	// Read gets a specific relationship between two actors
	Read(actorAId int64, actorBId int64) (*entities.ActorRelation, error)
}

type SocialgraphGRPC struct {
	config GraphGRPCConfigInterface
}

func NewSocialgraphGRPC(config GraphGRPCConfigInterface) SocialgraphGRPCInterface {
	return &SocialgraphGRPC{
		config: config,
	}
}

// getClient returns the grpc client
func (srv *SocialgraphGRPC) getClient() (graphpb.SocialgraphServiceClient, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`))

	for {
		cc, err := grpc.NewClient(srv.config.GetGraphServiceAddr(), opts...)

		if err != nil {
			log.Println("Socialgraph Relationship GRPC not ready...")
			socialgraphGRPCConnectCounts++
		} else {
			log.Println("Connected to socialgraph relationship GRPC!")
			return graphpb.NewSocialgraphServiceClient(cc), nil
		}

		if socialgraphGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// Browse gets a list of relationships for actorAIds vs. actorBId
func (srv *SocialgraphGRPC) Browse(actorAIds []int64, actorBId int64) (map[int64]*entities.ActorRelation, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	req := &graphpb.SocialgraphBrowseRequest{
		ActorAIds: actorAIds,
		ActorBId:  actorBId,
	}

	resp, err := client.Browse(context.Background(), req)
	if err != nil {
		return nil, err
	}

	relations := make(map[int64]*entities.ActorRelation)
	for id, r := range resp.Relations {
		relation := GrpcToRelationEntity(r)
		relations[id] = relation
	}

	return relations, nil
}

// Read gets a relationship for actorAid vs. actorBid
func (srv *SocialgraphGRPC) Read(actorAId int64, actorBId int64) (*entities.ActorRelation, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	req := &graphpb.SocialgraphReadRequest{
		ActorAId: actorAId,
		ActorBId: actorBId,
	}

	resp, err := client.Read(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return GrpcToRelationEntity(resp.GetRelation()), nil
}
