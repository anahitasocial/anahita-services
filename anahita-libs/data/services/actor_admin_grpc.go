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

var actorAdminsGRPCConnectCounts int

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

// getClient returns the grpc client
func (srv *ActorAdminsGRPC) getClient() (graphpb.ActorAdminServiceClient, error) {
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
			log.Println("Actor Admin GRPC not ready...")
			actorAdminsGRPCConnectCounts++
		} else {
			log.Println("Connected to Actor Admin GRPC!")
			return graphpb.NewActorAdminServiceClient(cc), nil
		}

		if actorAdminsGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
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
