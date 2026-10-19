package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
)

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

// getClient returns the client, on the one connection to the service this
// process keeps (sharedConnection).
func (srv *SocialgraphBlockGRPC) getClient() (graphpb.SocialgraphBlockServiceClient, error) {
	conn, err := sharedConnection(srv.config.GetGraphServiceAddr())
	if err != nil {
		return nil, err
	}
	return graphpb.NewSocialgraphBlockServiceClient(conn), nil
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
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
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
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
	})
	if err != nil {
		return err
	}

	return nil
}
