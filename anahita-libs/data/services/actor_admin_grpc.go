package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
)

type ActorAdminsGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

type ActorAdminsGRPCInterface interface {
	// Browse returns a list of admins
	Browse(actorId int64) ([]*entities.AnahitaNode, error)

	// Add adds a new admin
	Add(actor *entities.AnahitaNode, admin *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error)

	// Delete removes an admin
	Delete(actor *entities.AnahitaNode, admin *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error)
}

type ActorAdminsGRPC struct {
	config ActorAdminsGRPCConfigInterface
}

func NewActorAdminsGRPC(config ActorAdminsGRPCConfigInterface) ActorAdminsGRPCInterface {
	return &ActorAdminsGRPC{
		config: config,
	}
}

// getClient returns the client, on the one connection to the service this
// process keeps (sharedConnection).
func (srv *ActorAdminsGRPC) getClient() (graphpb.ActorAdminServiceClient, error) {
	conn, err := sharedConnection(srv.config.GetGraphServiceAddr())
	if err != nil {
		return nil, err
	}
	return graphpb.NewActorAdminServiceClient(conn), nil
}

// Browse returns a list of admins
func (srv *ActorAdminsGRPC) Browse(actorId int64) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Browse(context.Background(), &graphpb.ActorAdminBrowseRequest{
		ActorId: actorId,
	})
	if err != nil {
		return nil, err
	}
	if resp.Admins == nil {
		return []*entities.AnahitaNode{}, nil
	}

	return grpcToGraphNodes(resp.Admins), nil
}

// Add adds a new admin
func (srv *ActorAdminsGRPC) Add(actor *entities.AnahitaNode, admin *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Add(context.Background(), &graphpb.ActorAdminAddRequest{
		Actor: graphNodeToGrpc(actor),
		Admin: graphNodeToGrpc(admin),
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.Admin == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Admin), nil
}

// Delete removes an admin
func (srv *ActorAdminsGRPC) Delete(actor *entities.AnahitaNode, admin *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Delete(context.Background(), &graphpb.ActorAdminDeleteRequest{
		Actor: graphNodeToGrpc(actor),
		Admin: graphNodeToGrpc(admin),
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.Admin == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Admin), nil
}
