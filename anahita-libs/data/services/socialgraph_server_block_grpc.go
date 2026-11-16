package services

import (
	"anahita-libs/proto/graphpb"
	"context"
	"time"
)

// ServerBlock is a server a person blocked for themselves (roadmap row
// 152).
type ServerBlock struct {
	Domain    string
	CreatedAt time.Time
}

// ServerFollow is a follow between a person and somebody on a server: who
// follows whom.
type ServerFollow struct {
	SourceID int64
	TargetID int64
}

// SocialgraphServerBlockGRPCInterface blocks and unblocks servers for one
// person, and lists them.
type SocialgraphServerBlockGRPCInterface interface {
	Add(personID int64, domain string) error
	Delete(personID int64, domain string) error
	// Browse is the person's blocked servers, the latest first, and how
	// many there are.
	Browse(personID, start, limit int64) ([]*ServerBlock, int64, error)
	// Follows is the follows, either way, between the person and anybody
	// on the server or one under it.
	Follows(personID int64, domain string) ([]*ServerFollow, error)
}

type SocialgraphServerBlockGRPC struct {
	config GraphGRPCConfigInterface
}

func NewSocialgraphServerBlockGRPC(config GraphGRPCConfigInterface) SocialgraphServerBlockGRPCInterface {
	return &SocialgraphServerBlockGRPC{config: config}
}

func (srv *SocialgraphServerBlockGRPC) getClient() (graphpb.SocialgraphServerBlockServiceClient, error) {
	conn, err := sharedConnection(srv.config.GetGraphServiceAddr())
	if err != nil {
		return nil, err
	}
	return graphpb.NewSocialgraphServerBlockServiceClient(conn), nil
}

func (srv *SocialgraphServerBlockGRPC) Add(personID int64, domain string) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}
	_, err = client.Add(context.Background(), &graphpb.ServerBlockAddRequest{PersonId: personID, Domain: domain})
	return err
}

func (srv *SocialgraphServerBlockGRPC) Delete(personID int64, domain string) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}
	_, err = client.Delete(context.Background(), &graphpb.ServerBlockDeleteRequest{PersonId: personID, Domain: domain})
	return err
}

func (srv *SocialgraphServerBlockGRPC) Browse(personID, start, limit int64) ([]*ServerBlock, int64, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.Browse(context.Background(), &graphpb.ServerBlockBrowseRequest{PersonId: personID, Start: start, Limit: limit})
	if err != nil {
		return nil, 0, err
	}
	blocks := make([]*ServerBlock, 0, len(resp.Blocks))
	for _, b := range resp.Blocks {
		blocks = append(blocks, &ServerBlock{Domain: b.Domain, CreatedAt: unixToTime(b.CreatedAt)})
	}
	return blocks, resp.Total, nil
}

func (srv *SocialgraphServerBlockGRPC) Follows(personID int64, domain string) ([]*ServerFollow, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.Follows(context.Background(), &graphpb.ServerFollowsRequest{PersonId: personID, Domain: domain})
	if err != nil {
		return nil, err
	}
	follows := make([]*ServerFollow, 0, len(resp.Follows))
	for _, f := range resp.Follows {
		follows = append(follows, &ServerFollow{SourceID: f.SourceId, TargetID: f.TargetId})
	}
	return follows, nil
}
