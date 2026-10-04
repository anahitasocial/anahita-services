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

var subscriptionsGRPCConnectCounts int

type SubscriptionsGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

type SubscriptionsGRPCInterface interface {
	BrowseSources(target *entities.AnahitaNode) ([]*entities.AnahitaNode, error)
	Add(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) error
	Delete(source *entities.AnahitaNode, target *entities.AnahitaNode) error
	IsSubscribedTo(sourceID int64, targetID int64) (bool, error)
	IsSubscribedToMany(sourceID int64, targetIDs []int64) (map[int64]bool, error)
}

type SubscriptionsGRPC struct {
	config SubscriptionsGRPCConfigInterface
}

func NewSubscriptionsGRPC(config SubscriptionsGRPCConfigInterface) *SubscriptionsGRPC {
	return &SubscriptionsGRPC{
		config: config,
	}
}

// getClient
func (srv *SubscriptionsGRPC) getClient() (graphpb.SubscriptionServiceClient, error) {
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
			log.Println("Notification Subscriptions GRPC not ready...")
			subscriptionsGRPCConnectCounts++
		} else {
			log.Println("Connected to Notification Subscriptions GRPC!")
			return graphpb.NewSubscriptionServiceClient(cc), nil
		}

		if subscriptionsGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// BrowseSources
func (srv *SubscriptionsGRPC) BrowseSources(target *entities.AnahitaNode) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.BrowseSources(context.Background(), &graphpb.SubscriptionBrowseSourcesRequest{
		Target: &graphpb.Target{
			Id:   target.ID,
			Type: target.Type,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	var sources []*entities.AnahitaNode
	for _, src := range resp.Sources {
		sources = append(sources, &entities.AnahitaNode{
			ID:   src.Id,
			Type: src.Type,
		})
	}

	return sources, nil
}

// Add
func (srv *SubscriptionsGRPC) Add(source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *entities.Viewer) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Add(context.Background(), &graphpb.SubscriptionAddRequest{
		Source: &graphpb.Source{
			Id:   source.ID,
			Type: source.Type,
		},
		Target: &graphpb.Target{
			Id:   target.ID,
			Type: target.Type,
		},
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// Delete
func (srv *SubscriptionsGRPC) Delete(source *entities.AnahitaNode, target *entities.AnahitaNode) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Delete(context.Background(), &graphpb.SubscriptionDeleteRequest{
		Source: &graphpb.Source{
			Id:   source.ID,
			Type: source.Type,
		},
		Target: &graphpb.Target{
			Id:   target.ID,
			Type: target.Type,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// IsSubscribedTo
func (srv *SubscriptionsGRPC) IsSubscribedTo(sourceID int64, targetID int64) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}

	resp, err := client.IsSubscribedTo(context.Background(), &graphpb.SubscriptionIsSubscribedToRequest{
		SourceId: sourceID,
		TargetId: targetID,
	})
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}

	return resp.IsSubscribed, nil
}

// IsSubscribedToMany
func (srv *SubscriptionsGRPC) IsSubscribedToMany(sourceID int64, targetIDs []int64) (map[int64]bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.IsSubscribedToMany(context.Background(), &graphpb.SubscriptionIsSubscribedToManyRequest{
		SourceId:  sourceID,
		TargetIds: targetIDs,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	// loop through resp
	results := make(map[int64]bool)
	for _, item := range resp.Results {
		results[item.Id] = item.IsSubscribed
	}

	return results, nil
}
