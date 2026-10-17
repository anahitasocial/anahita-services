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

var hashtagGRPCConnectCounts int

type HashtagGRPCConfigInterface interface {
	GetGraphServiceAddr() string
}

// HashtagSuggestRequest is what somebody writing has typed: the start of
// a hashtag, or the words of a text, and the hashtags already in it.
type HashtagSuggestRequest struct {
	Prefix  string
	Words   []string
	Exclude []string
	Limit   uint32
	// A listing for the Hashtags page rather than an offer: the ones the
	// viewer follows, or all, perhaps beginning with Prefix, ordered by
	// "name" or "uses", a page at a time.
	List         bool
	FollowedOnly bool
	Order        string
	Offset       uint64
}

// HashtagSuggestion is a hashtag in use, offered to somebody writing.
type HashtagSuggestion struct {
	ID       int64
	Alias    string
	Name     string
	Uses     int64
	Followed bool
}

// HashtagGRPCInterface is the graph's hashtags, as offered to somebody
// writing.
type HashtagGRPCInterface interface {
	Suggest(req *HashtagSuggestRequest, viewer *entities.Viewer) ([]*HashtagSuggestion, error)
	// List is the hashtags of a listing, and how many there are in all.
	List(req *HashtagSuggestRequest, viewer *entities.Viewer) ([]*HashtagSuggestion, uint64, error)
}

func NewHashtagGRPC(config HashtagGRPCConfigInterface) HashtagGRPCInterface {
	return &HashtagGRPC{config: config}
}

type HashtagGRPC struct {
	config HashtagGRPCConfigInterface
}

func (srv *HashtagGRPC) getClient() (graphpb.HashtagServiceClient, error) {
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
			log.Println("Hashtag GRPC not ready...")
			hashtagGRPCConnectCounts++
		} else {
			return graphpb.NewHashtagServiceClient(cc), nil
		}

		if hashtagGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
	}
}

// Suggest implements HashtagGRPCInterface.
func (srv *HashtagGRPC) Suggest(req *HashtagSuggestRequest, viewer *entities.Viewer) ([]*HashtagSuggestion, error) {
	found, _, err := srv.List(req, viewer)
	return found, err
}

// List implements HashtagGRPCInterface.
func (srv *HashtagGRPC) List(req *HashtagSuggestRequest, viewer *entities.Viewer) ([]*HashtagSuggestion, uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.Suggest(context.Background(), &graphpb.HashtagSuggestRequest{
		Viewer:       &graphpb.Viewer{Id: viewer.ID, PersonType: viewer.PersonType},
		Prefix:       req.Prefix,
		Words:        req.Words,
		Exclude:      req.Exclude,
		Limit:        req.Limit,
		List:         req.List,
		FollowedOnly: req.FollowedOnly,
		Order:        req.Order,
		Offset:       req.Offset,
	})
	if err != nil {
		return nil, 0, err
	}

	suggestions := []*HashtagSuggestion{}
	for _, each := range resp.GetSuggestions() {
		suggestions = append(suggestions, &HashtagSuggestion{
			ID:       each.GetId(),
			Alias:    each.GetAlias(),
			Name:     each.GetName(),
			Uses:     each.GetUses(),
			Followed: each.GetFollowed(),
		})
	}
	return suggestions, resp.GetTotal(), nil
}
