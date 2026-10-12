package services

import (
	"context"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// AuthoredInterface lists what one person wrote, wherever it is, for their
// own copy of their data. The graph applies no access rule to it, so it is
// asked only on behalf of that person.
type AuthoredInterface interface {
	// ListAuthored returns a page, lowest id first, after an id.
	ListAuthored(personID, afterID int64, limit uint64) ([]*entities.AnahitaNode, error)
}

// NewAuthoredGRPC is the media service's client, for that list.
func NewAuthoredGRPC(config MediaGRPCConfigInterface) AuthoredInterface {
	return &MediaGRPC{config: config}
}

func (srv *MediaGRPC) ListAuthored(personID, afterID int64, limit uint64) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.ListAuthored(context.Background(), &graphpb.AuthoredListRequest{
		PersonId: personID,
		AfterId:  afterID,
		Limit:    limit,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return grpcToGraphNodes(resp.Nodes), nil
}
