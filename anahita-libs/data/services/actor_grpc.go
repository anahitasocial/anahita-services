package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"log"
	"strings"
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
	// ReadOfType is Read narrowed to one actor type. See the implementation:
	// people and groups are both actors, so the family filter alone is not enough.
	ReadOfType(id int64, nodeType string, viewer *entities.Viewer) (*entities.AnahitaNode, error)

	// ReadDeletionCounts totals what deleting this actor would take with it,
	// for the confirmation dialog.
	ReadDeletionCounts(id int64, viewer *entities.Viewer) (*graphpb.ActorDeletionCountsResponse, error)

	// Archive marks an actor archived. Permanent; destroys nothing.
	Archive(id int64, viewer *entities.Viewer) (bool, error)

	// SetEnabled turns an actor on or off. Reversible.
	SetEnabled(id int64, enabled bool, viewer *entities.Viewer) (bool, error)

	// SetFeatured features or unfeatures an actor.
	//
	// SUPER ADMINISTRATORS ONLY, and the caller enforces that. Featuring a
	// disabled, archived or deleted actor is refused with an error carrying
	// "not_featurable"; see IsNotFeaturable.
	SetFeatured(id int64, featured bool, viewer *entities.Viewer) (bool, error)

	// TouchLastVisit records that an actor just signed in.
	TouchLastVisit(id int64) (bool, error)

	// Purge destroys an actor immediately, skipping the grace period.
	//
	// SUPER ADMINISTRATORS ONLY, and the caller enforces that — graph-grpc has
	// no notion of a usertype. Nothing about this is reversible.
	Purge(id int64, viewer *entities.Viewer) (bool, error)

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
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
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
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}

	return grpcToGraphNodes(resp.Actors), nil
}

// Read returns a single media
// Read fetches an actor by id, of any actor type.
//
// Prefer ReadOfType where the caller knows what it expects — see the note
// there. This remains for the internal lookups that genuinely take any actor,
// such as resolving the viewer's own node.
func (srv *ActorsGRPC) Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	return srv.ReadOfType(id, "", viewer)
}

// ReadOfType fetches an actor by id and refuses it if it is not of nodeType.
//
// Node ids are unique across every type in one table, so an id-only read
// returns whatever holds that id. The graph service already restricts this RPC
// to the node.actor. family, which stops a hashtag or a photo coming back —
// but people and groups are both actors, so /groups/<a person id> still
// answered with a person until the exact type travelled with the request.
//
// An empty nodeType means "any actor" and keeps the family restriction.
func (srv *ActorsGRPC) ReadOfType(id int64, nodeType string, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}

	resp, err := client.Read(context.Background(), &graphpb.ActorReadRequest{
		Id:       id,
		NodeType: nodeType,
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
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
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
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
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
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
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
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
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

// ReadDeletionCounts totals what deleting this actor would take with it.
//
// Returns the protobuf response rather than mapping it into an entity: every
// field is a number destined for a dialog, and an intermediate type would be a
// copy of this one under a different name.
func (srv *ActorsGRPC) ReadDeletionCounts(id int64, viewer *entities.Viewer) (*graphpb.ActorDeletionCountsResponse, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	if client == nil {
		return nil, nil
	}

	return client.DeletionCounts(context.Background(), &graphpb.ActorDeletionCountsRequest{
		Id: id,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
	})
}

// Purge destroys an actor immediately, skipping the grace period.
//
// Returns false when there was nothing to purge, so a caller can tell that
// from success. The authority check belongs to the caller; see the interface.
func (srv *ActorsGRPC) Purge(id int64, viewer *entities.Viewer) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}
	if client == nil {
		return false, nil
	}

	resp, err := client.Purge(context.Background(), &graphpb.ActorPurgeRequest{
		Id: id,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
	})
	if err != nil {
		return false, err
	}

	return resp.Purged, nil
}

// Archive marks an actor archived. Permanent; destroys nothing.
func (srv *ActorsGRPC) Archive(id int64, viewer *entities.Viewer) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}
	if client == nil {
		return false, nil
	}

	resp, err := client.Archive(context.Background(), &graphpb.ActorArchiveRequest{
		Id:     id,
		Viewer: viewerToGrpc(viewer),
	})
	if err != nil {
		return false, err
	}

	return resp.Archived, nil
}

// SetEnabled turns an actor on or off. Reversible.
func (srv *ActorsGRPC) SetEnabled(id int64, enabled bool, viewer *entities.Viewer) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}
	if client == nil {
		return false, nil
	}

	resp, err := client.SetEnabled(context.Background(), &graphpb.ActorSetEnabledRequest{
		Id:      id,
		Enabled: enabled,
		Viewer:  viewerToGrpc(viewer),
	})
	if err != nil {
		return false, err
	}

	return resp.Changed, nil
}

// SetFeatured features or unfeatures an actor.
func (srv *ActorsGRPC) SetFeatured(id int64, featured bool, viewer *entities.Viewer) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}
	if client == nil {
		return false, nil
	}

	resp, err := client.SetFeatured(context.Background(), &graphpb.ActorSetFeaturedRequest{
		Id:       id,
		Featured: featured,
		Viewer:   viewerToGrpc(viewer),
	})
	if err != nil {
		return false, err
	}

	return resp.Changed, nil
}

// IsNotFeaturable reports whether the graph refused to feature an actor because
// it is disabled, archived or deleted.
//
// Here rather than in each service, because people and groups both answer it
// with the same 409. Matched on the message, which is all gRPC carries across —
// the same way isLastSuperAdmin reads its sentinel.
func IsNotFeaturable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not_featurable")
}

// TouchLastVisit records that an actor just signed in.
//
// No viewer: the person is acting as themselves and the caller has already
// authenticated them.
func (srv *ActorsGRPC) TouchLastVisit(id int64) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}
	if client == nil {
		return false, nil
	}

	resp, err := client.TouchLastVisit(context.Background(), &graphpb.ActorTouchLastVisitRequest{
		Id: id,
	})
	if err != nil {
		return false, err
	}

	return resp.Changed, nil
}

// viewerToGrpc maps a viewer for the actor RPCs.
//
// The four fields every one of them sends, in one place — they were repeated
// inline at each call site, which is how one of them came to omit a field.
func viewerToGrpc(viewer *entities.Viewer) *graphpb.Viewer {
	if viewer == nil {
		return &graphpb.Viewer{}
	}
	return &graphpb.Viewer{
		Id:         viewer.ID,
		PersonType: viewer.PersonType,
		Email:      viewer.Email,
		Username:   viewer.Username,
	}
}
