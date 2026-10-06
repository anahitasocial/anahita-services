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

var feedLeadersGRPCConnectCounts int

type FeedLeadersGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

// FeedLeadersGRPCInterface is the interface for feed leaders service
type FeedLeadersGRPCInterface interface {
	// Count returns the total number of feed nodes from leaders
	Count(req *FeedLeadersCountRequest, viewer *entities.Viewer) (uint64, error)

	// Browse returns a list of feed nodes from leaders
	Browse(req *FeedLeadersBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error)

	// Active returns the actors the viewer follows that have posted in the
	// last so many hours, most recent first.
	Active(hours uint32, limit uint32, mediumTypes []string, viewer *entities.Viewer) ([]ActiveLeader, error)

	// getClient returns the grpc client
	getClient() (graphpb.FeedLeadersServiceClient, error)
}

// NewFeedLeadersGRPC creates a new feed leaders service
func NewFeedLeadersGRPC(config FeedLeadersGRPCConfigInterface) FeedLeadersGRPCInterface {
	return &FeedLeadersGRPC{
		config: config,
	}
}

// FeedLeadersGRPC is the feed leaders service
type FeedLeadersGRPC struct {
	config FeedLeadersGRPCConfigInterface
}

// getClient returns the grpc client
func (srv *FeedLeadersGRPC) getClient() (graphpb.FeedLeadersServiceClient, error) {
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
			log.Println("Feed Leaders GRPC not ready...")
			feedLeadersGRPCConnectCounts++
		} else {
			log.Println("Connected to FeedLeaders GRPC!")
			return graphpb.NewFeedLeadersServiceClient(cc), nil
			// return srv.client, nil
		}

		if feedLeadersGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// Count returns the total number of feed nodes from leaders
func (srv *FeedLeadersGRPC) Count(req *FeedLeadersCountRequest, viewer *entities.Viewer) (uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return 0, err
	}

	resp, err := client.Count(context.Background(), &graphpb.FeedLeadersCountRequest{
		IncludeReposts:    req.IncludeReposts,
		IncludeLiked:      req.IncludeLiked,
		LeaderEntityTypes: req.LeaderEntityTypes,
		MediumEntityTypes: req.MediumEntityTypes,
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

// Browse returns a list of feed nodes from leaders
func (srv *FeedLeadersGRPC) Browse(req *FeedLeadersBrowseRequest, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Browse(context.Background(), &graphpb.FeedLeadersBrowseRequest{
		IncludeReposts:    req.IncludeReposts,
		IncludeLiked:      req.IncludeLiked,
		LeaderEntityTypes: req.LeaderEntityTypes,
		MediumEntityTypes: req.MediumEntityTypes,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
		Offset: req.Offset,
		Limit:  req.Limit,
	})
	if err != nil {
		return nil, err
	}
	if resp.Nodes == nil {
		return []*entities.AnahitaNode{}, nil
	}

	return grpcToGraphNodes(resp.Nodes), nil
}

// ActiveLeader is one actor the viewer follows, and what they posted lately:
// when the newest was made, and how many there are that the viewer may see.
type ActiveLeader struct {
	ActorID  int64
	LatestAt time.Time
	Count    uint64
}

// Active returns the actors the viewer follows that have posted lately.
func (srv *FeedLeadersGRPC) Active(hours uint32, limit uint32, mediumTypes []string, viewer *entities.Viewer) ([]ActiveLeader, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Active(context.Background(), &graphpb.FeedLeadersActiveRequest{
		Hours:             hours,
		Limit:             limit,
		MediumEntityTypes: mediumTypes,
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

	leaders := make([]ActiveLeader, 0, len(resp.GetLeaders()))
	for _, leader := range resp.GetLeaders() {
		leaders = append(leaders, ActiveLeader{
			ActorID:  leader.GetActorId(),
			LatestAt: time.Unix(leader.GetLatestAt(), 0).UTC(),
			Count:    leader.GetCount(),
		})
	}

	return leaders, nil
}
