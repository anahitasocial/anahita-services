package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
)

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

// getClient returns the client, on the one connection to the service this
// process keeps (sharedConnection).
func (srv *SocialgraphGRPC) getClient() (graphpb.SocialgraphServiceClient, error) {
	conn, err := sharedConnection(srv.config.GetGraphServiceAddr())
	if err != nil {
		return nil, err
	}
	return graphpb.NewSocialgraphServiceClient(conn), nil
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
