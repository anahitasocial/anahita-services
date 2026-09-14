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

var mediaGRPCConnectCounts int

type MediaGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

// MediaGRPCInterface is the interface for media service
type MediaGRPCInterface interface {
	// Count returns the total number of media
	Count(req *MediaGRPCBrowseRequest, viewer *entities.Viewer) (uint64, error)

	// Browse returns a list of media
	Browse(req *MediaGRPCBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)

	// Read returns a single media
	Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error)

	// Edit updates a media
	Edit(medium *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error)

	// Add creates a new media
	Add(medium *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error)

	// Delete removes a media
	Delete(id int64, viewer *entities.Viewer) error

	// getClient returns the grpc client
	getClient() (graphpb.MediumServiceClient, error)
}

// NewMediaGRPC creates a new media service
func NewMediaGRPC(config MediaGRPCConfigInterface) MediaGRPCInterface {
	return &MediaGRPC{
		config: config,
	}
}

// MediaGRPC is the media service
type MediaGRPC struct {
	config MediaGRPCConfigInterface
}

// GetClient returns the grpc client
func (srv *MediaGRPC) getClient() (graphpb.MediumServiceClient, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`))

	for {
		cc, err := grpc.NewClient(srv.config.GetGraphServiceAddr(), opts...)

		if err != nil {
			log.Println("Medium GRPC not ready...")
			mediaGRPCConnectCounts++
		} else {
			log.Println("Connected to Medium GRPC!")
			return graphpb.NewMediumServiceClient(cc), nil
		}

		if mediaGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// Count returns the total number of media
func (srv *MediaGRPC) Count(req *MediaGRPCBrowseRequest, viewer *entities.Viewer) (uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return 0, err
	}
	if client == nil {
		return 0, nil
	}

	resp, err := client.Count(context.Background(), &graphpb.MediumCountRequest{
		Types:   req.Types,
		OwnerId: req.OwnerID,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
	})
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, nil
	}

	return resp.Total, nil
}

// Browse returns a list of media
func (srv *MediaGRPC) Browse(req *MediaGRPCBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.Browse(context.Background(), &graphpb.MediumBrowseRequest{
		Types:     req.Types,
		OwnerId:   req.OwnerID,
		Limit:     req.Limit,
		Offset:    req.Offset,
		Ordering:  req.Ordering,
		Direction: req.Direction,
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
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNodes(resp.Media), nil
}

// Read returns a single media
func (srv *MediaGRPC) Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.Read(context.Background(), &graphpb.MediumReadRequest{
		Id: id,
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
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Medium), nil
}

// Edit updates a media
func (srv *MediaGRPC) Edit(item *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.Edit(context.Background(), &graphpb.MediumEditRequest{
		Medium: graphNodeToGrpc(item),
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
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Medium), nil
}

// Add creates a new media
func (srv *MediaGRPC) Add(item *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.Add(context.Background(), &graphpb.MediumAddRequest{
		Medium: graphNodeToGrpc(item),
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
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Medium), nil
}

// Delete removes a media
func (srv *MediaGRPC) Delete(id int64, viewer *entities.Viewer) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}
	if client == nil {
		return nil
	}

	_, err = client.Delete(context.Background(), &graphpb.MediumDeleteRequest{
		Id: id,
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
