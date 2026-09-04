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

var actorsGRPCConnectCounts int

type ActorGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

// ActorsGRPCInterface is the interface for media service
type ActorsGRPCInterface interface {
	// Count returns the total number of media
	Count(req *ActorGRPCBrowseRequest, viewer *entities.Viewer) (uint64, error)

	// Browse returns a list of media
	Browse(req *ActorGRPCBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)

	// Read returns a single media
	Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error)

	// BatchRead returns a list of media
	BatchRead(req *ActorGRPCBatchReadRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)

	// Edit updates a media
	Edit(node *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error)

	// Add creates a new node
	Add(node *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error)

	// Delete removes a node
	Delete(id int64, viewer *entities.Viewer) error

	// Restore reverses a Delete within the grace period, reporting whether
	// there was anything to restore.
	Restore(id int64, viewer *entities.Viewer) (bool, error)

	// getClient returns the grpc client
	getClient() (graphpb.ActorServiceClient, error)
}

// NewActorsGRPC creates a new media service
func NewActorsGRPC(config ActorGRPCConfigInterface) ActorsGRPCInterface {
	return &ActorsGRPC{
		config: config,
	}
}

// ActorsGRPC is the media service
type ActorsGRPC struct {
	config     ActorGRPCConfigInterface
	entityType string
}

// getClient returns the grpc client
func (srv *ActorsGRPC) getClient() (graphpb.ActorServiceClient, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`))

	for {
		cc, err := grpc.NewClient(srv.config.GetGraphServiceAddr(), opts...)

		if err != nil {
			log.Println("Actor GRPC not ready...")
			actorsGRPCConnectCounts++
		} else {
			log.Println("Connected to Actor GRPC!")
			return graphpb.NewActorServiceClient(cc), nil
			// return srv.client, nil
		}

		if actorsGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// Count returns the total number of media
func (srv *ActorsGRPC) Count(req *ActorGRPCBrowseRequest, viewer *entities.Viewer) (uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return 0, err
	}

	resp, err := client.Count(context.Background(), &graphpb.ActorCountRequest{
		Types:  req.Types,
		Search: req.Search,
		Viewer: &graphpb.Viewer{
			Id:       viewer.ID,
			UserType: viewer.UserType,
			Email:    viewer.Email,
			Username: viewer.Username,
		},
	})
	if err != nil {
		return 0, err
	}

	return resp.Total, nil
}

// Browse returns a list of media
func (srv *ActorsGRPC) Browse(req *ActorGRPCBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.Browse(context.Background(), &graphpb.ActorBrowseRequest{
		Types:     req.Types,
		Limit:     req.Limit,
		Offset:    req.Offset,
		Ordering:  req.Ordering,
		Direction: req.Direction,
		Search:    req.Search,
		Viewer: &graphpb.Viewer{
			Id:       viewer.ID,
			UserType: viewer.UserType,
			Email:    viewer.Email,
			Username: viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}

	return grpcToGraphNodes(resp.Actors), nil
}

// Read returns a single media
func (srv *ActorsGRPC) Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.Read(context.Background(), &graphpb.ActorReadRequest{
		Id: id,
		Viewer: &graphpb.Viewer{
			Id:       viewer.ID,
			UserType: viewer.UserType,
			Email:    viewer.Email,
			Username: viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.Actor == nil {
		return nil, nil
	}

	// log.Println("GRPC SERVICE: Actor GRPC Features Read:", resp.Actor.FeaturesRaw, resp.Actor.Features)

	return grpcToGraphNode(resp.Actor), nil
}

// BatchRead returns a list of media
func (srv *ActorsGRPC) BatchRead(req *ActorGRPCBatchReadRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.BatchRead(context.Background(), &graphpb.ActorBatchReadRequest{
		Ids:       req.IDs,
		Ordering:  req.Ordering,
		Direction: req.Direction,
		Viewer: &graphpb.Viewer{
			Id:       viewer.ID,
			UserType: viewer.UserType,
			Email:    viewer.Email,
			Username: viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.Actors == nil {
		return nil, nil
	}

	return grpcToGraphNodes(resp.Actors), nil
}

// Edit updates a media
func (srv *ActorsGRPC) Edit(item *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.Edit(context.Background(), &graphpb.ActorEditRequest{
		Actor: graphNodeToGrpc(item),
		Viewer: &graphpb.Viewer{
			Id:       viewer.ID,
			UserType: viewer.UserType,
			Email:    viewer.Email,
			Username: viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.Actor == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Actor), nil
}

// Add creates a new media
func (srv *ActorsGRPC) Add(item *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.Add(context.Background(), &graphpb.ActorAddRequest{
		Actor: graphNodeToGrpc(item),
		Viewer: &graphpb.Viewer{
			Id:       viewer.ID,
			UserType: viewer.UserType,
			Email:    viewer.Email,
			Username: viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.Actor == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Actor), nil
}

// Restore reverses a Delete within the grace period.
//
// Returns whether anything was restored: false means the account was already
// purged or was never deleted, which the caller has to tell apart from success
// so it can say so rather than claiming a group is back when it is not.
func (srv *ActorsGRPC) Restore(id int64, viewer *entities.Viewer) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}
	if client == nil {
		return false, nil
	}

	resp, err := client.Restore(context.Background(), &graphpb.ActorRestoreRequest{
		Id: id,
		Viewer: &graphpb.Viewer{
			Id:       viewer.ID,
			UserType: viewer.UserType,
			Email:    viewer.Email,
			Username: viewer.Username,
		},
	})
	if err != nil {
		return false, err
	}

	return resp.Restored, nil
}

// Delete removes a media
func (srv *ActorsGRPC) Delete(id int64, viewer *entities.Viewer) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}
	if client == nil {
		return nil
	}

	_, err = client.Delete(context.Background(), &graphpb.ActorDeleteRequest{
		Id: id,
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
