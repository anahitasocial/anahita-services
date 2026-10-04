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

var repostGRPCConnectCounts int

type RepostGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

type RepostGRPCInterface interface {
	Browse(req *RepostBrowseRequest) ([]*entities.AnahitaNode, error)
	BrowseAuthors(req *RepostBrowseAuthorsRequest) ([]*entities.AnahitaNode, error)
	Read(req *RepostReadRequest) (*entities.AnahitaNode, error)
	Add(req *RepostAddRequest) (*entities.AnahitaNode, error)
	Delete(req *RepostDeleteRequest) error
	Count(req *RepostCountRequest) (uint64, error)
	IsReposted(actorID int64, nodeID int64) (bool, error)
	IsRepostedMany(actorID int64, nodeIDs []int64) (map[int64]bool, error)
}

type RepostGRPC struct {
	config RepostGRPCConfigInterface
}

func NewRepostGRPC(config RepostGRPCConfigInterface) RepostGRPCInterface {
	return &RepostGRPC{
		config: config,
	}
}

// getClient
func (srv *RepostGRPC) getClient() (graphpb.RepostServiceClient, error) {
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
			log.Println("Repost GRPC not ready...")
			repostGRPCConnectCounts++
		} else {
			log.Println("Connected to Like GRPC!")
			return graphpb.NewRepostServiceClient(cc), nil
		}

		if repostGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// Browse
func (srv *RepostGRPC) Browse(req *RepostBrowseRequest) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Browse(context.Background(), &graphpb.RepostBrowseRequest{
		OwnerId:   req.OwnerID,
		Offset:    req.Offset,
		Limit:     req.Limit,
		Direction: req.OrderingDir,
		Ordering:  req.Ordering,
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.Reposts == nil {
		return []*entities.AnahitaNode{}, nil
	}

	return grpcToGraphNodes(resp.Reposts), nil
}

// BrowseAuthors
func (srv *RepostGRPC) BrowseAuthors(req *RepostBrowseAuthorsRequest) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.BrowseAuthors(context.Background(), &graphpb.RepostBrowseAuthorsRequest{
		ParentId:  req.ParentID,
		Offset:    req.Offset,
		Limit:     req.Limit,
		Direction: req.OrderingDir,
		Ordering:  req.Ordering,
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.Authors == nil {
		return []*entities.AnahitaNode{}, nil
	}

	return grpcToGraphNodes(resp.Authors), nil
}

// Read
func (srv *RepostGRPC) Read(req *RepostReadRequest) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	repost, err := client.Read(context.Background(), &graphpb.RepostReadRequest{
		ParentId: req.ParentID,
		AuthorId: req.AuthorID,
	})
	if err != nil {
		return nil, err
	}
	if repost.Repost == nil {
		return nil, nil
	}

	return grpcToGraphNode(repost.Repost), nil
}

// Add
func (srv *RepostGRPC) Add(req *RepostAddRequest) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Add(context.Background(), &graphpb.RepostAddRequest{
		OwnerId:  req.OwnerID,
		ParentId: req.ParentID,
		Service:  req.Service,
		Access:   req.Access,
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.Repost == nil {
		return nil, nil
	}

	return grpcToGraphNode(resp.Repost), nil
}

// Delete
func (srv *RepostGRPC) Delete(req *RepostDeleteRequest) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Delete(context.Background(), &graphpb.RepostDeleteRequest{
		ParentId: req.ParentID,
		AuthorId: req.AuthorID,
	})
	if err != nil {
		return err
	}

	return nil
}

// Count
func (srv *RepostGRPC) Count(req *RepostCountRequest) (uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return 0, err
	}

	resp, err := client.Count(context.Background(), &graphpb.RepostCountRequest{
		OwnerId: req.OwnerID,
		Viewer: &graphpb.Viewer{
			Id:         req.Viewer.ID,
			PersonType: req.Viewer.PersonType,
			Email:      req.Viewer.Email,
			Username:   req.Viewer.Username,
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

// IsReposted
func (srv *RepostGRPC) IsReposted(authorID int64, nodeID int64) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}

	resp, err := client.IsReposted(context.Background(), &graphpb.IsRepostedRequest{
		AuthorId: authorID,
		NodeId:   nodeID,
	})
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}

	return resp.IsReposted, nil
}

// IsRepostedMany
func (srv *RepostGRPC) IsRepostedMany(authorID int64, nodeIDs []int64) (map[int64]bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.IsRepostedMany(context.Background(), &graphpb.IsRepostedManyRequest{
		AuthorId: authorID,
		NodeIds:  nodeIDs,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return map[int64]bool{}, nil
	}

	return resp.Results, nil
}
