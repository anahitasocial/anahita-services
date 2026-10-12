package services

import (
	"context"
	"errors"
	"log"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/feeds"
	"anahita-libs/grpctls"
	"anahita-libs/proto/graphpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var feedCustomGRPCConnectCounts int

// ErrFeedTooBroad is a feed that could not be read in the time a query is
// given. The feed has to be narrowed.
var ErrFeedTooBroad = errors.New("feed too broad")

type FeedCustomGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

// FeedCustomGRPCInterface reads a custom feed from the graph.
type FeedCustomGRPCInterface interface {
	// Browse returns a page of the feed def describes, owned by ownerID,
	// older than the cursor, as viewer may see it.
	Browse(def *feeds.Definition, ownerID int64, before feeds.Cursor, limit uint64, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)
}

func NewFeedCustomGRPC(config FeedCustomGRPCConfigInterface) FeedCustomGRPCInterface {
	return &FeedCustomGRPC{
		config: config,
	}
}

type FeedCustomGRPC struct {
	config FeedCustomGRPCConfigInterface
}

func (srv *FeedCustomGRPC) getClient() (graphpb.FeedCustomServiceClient, error) {
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
			log.Println("Feed Custom GRPC not ready...")
			feedCustomGRPCConnectCounts++
		} else {
			return graphpb.NewFeedCustomServiceClient(cc), nil
		}

		if feedCustomGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

func (srv *FeedCustomGRPC) Browse(def *feeds.Definition, ownerID int64, before feeds.Cursor, limit uint64, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	req := &graphpb.FeedCustomBrowseRequest{
		Definition: string(def.JSON()),
		OwnerId:    ownerID,
		Limit:      limit,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
	}
	if !before.IsZero() {
		req.BeforeTime = before.CreatedAt.Unix()
		req.BeforeId = before.ID
	}

	resp, err := client.Browse(context.Background(), req)
	if err != nil {
		if status.Code(err) == codes.DeadlineExceeded {
			return nil, ErrFeedTooBroad
		}
		return nil, err
	}
	if resp.Nodes == nil {
		return []*entities.AnahitaNode{}, nil
	}

	return grpcToGraphNodes(resp.Nodes), nil
}
