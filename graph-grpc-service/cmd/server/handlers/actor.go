package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"
	"anahita-libs/proto/graphpb"
	"context"
	"fmt"
	"graph/requests"
	"log"
	"strings"

	features "graph/data/actor_features"
	"graph/data/repositories"
	"graph/data/validators"
)

type actorHandler struct {
	graphpb.ActorServiceServer
	env *Env
}

func NewActorHandler(env *Env) *actorHandler {
	return &actorHandler{env: env}
}

// Count return the total number of actors
func (h *actorHandler) Count(ctx context.Context, req *graphpb.ActorCountRequest) (*graphpb.ActorCountResponse, error) {
	log.Printf("Received Count request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().Count
	rReq.Viewer = viewer
	rReq.Types = req.Types
	rReq.Search = req.Search
	rReq.AccessCheck = req.AccessCheck

	total, err := h.env.Repos.Nodes.Count(rReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.ActorCountResponse{
		Total: total,
	}, nil
}

// Browse return a list of actors
func (h *actorHandler) Browse(ctx context.Context, req *graphpb.ActorBrowseRequest) (*graphpb.ActorBrowseResponse, error) {
	log.Printf("Received Browse request: %v", req)
	var err error

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().Browse
	rReq.Viewer = viewer
	rReq.Types = req.Types
	rReq.Search = req.Search
	rReq.Offset = req.Offset
	rReq.Limit = req.Limit
	rReq.Ordering = req.Ordering
	rReq.OrderingDir = req.Direction
	rReq.AccessCheck = req.AccessCheck

	actors, err := h.env.Repos.Nodes.Browse(rReq)
	if err != nil {
		return nil, err
	}

	targetIds := extractNodeIDs(actors)
	subscriptionStatuses, err := h.env.Repos.SubscriptionsRel.IsSubscribedToMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	for _, actor := range actors {
		actor.IsSubscribedByViewer = subscriptionStatuses[actor.ID].IsSubscribed
	}

	return &graphpb.ActorBrowseResponse{
		Actors: nodesToGrpc(actors),
	}, nil
}

// Read return a single actor
func (h *actorHandler) Read(ctx context.Context, req *graphpb.ActorReadRequest) (*graphpb.ActorReadResponse, error) {
	log.Printf("Received Get request: %v", req)

	loader := features.NewFeatureLoader(h.env.Config.GetActorFeaturesPath())

	// Actors only, and the caller's exact type when it supplied one.
	//
	// A bare id lookup returned whatever node held that id, so
	// GET /groups/<hashtag id> answered 200 with a hashtag body. The family
	// filter stops that. The exact type stops the subtler version: a person
	// and a group are both actors, so /groups/<a person id> still returned a
	// person until the route's own type travelled with the request.
	//
	// Falling back to the family when node_type is empty keeps the internal
	// callers that genuinely take any actor working unchanged.
	nodeType := req.NodeType
	if nodeType == "" {
		nodeType = constants.NodeActorPrefixEntityType
	}

	node, err := h.env.Repos.Nodes.ReadOfType(req.Id, nodeType)
	if err != nil {
		return nil, err
	}

	actor := nodeToGrpc(node)
	defaults, err := loader.Load(actor.Type)
	if err != nil {
		log.Printf("Failed to load default features for %s: %v", actor.Type, err)
	} else {
		_ = features.ApplyFeatures(actor, loader, defaults)
	}

	if req.Viewer != nil {
		actor.IsSubscribedByViewer, err = h.env.Repos.SubscriptionsRel.IsSubscribedToOne(req.Viewer.Id, actor.Id)
		if err != nil {
			return nil, err
		}
	}

	return &graphpb.ActorReadResponse{
		Actor: actor,
	}, nil
}

// BatchRead return a list of actors
func (h *actorHandler) BatchRead(ctx context.Context, req *graphpb.ActorBatchReadRequest) (*graphpb.ActorBatchReadResponse, error) {
	log.Printf("Received BatchRead request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().BatchRead
	rReq.IDs = req.Ids
	rReq.Ordering = req.Ordering
	rReq.OrderingDir = req.Direction

	result, err := h.env.Repos.Nodes.BatchRead(rReq)
	if err != nil {
		return nil, err
	}

	targetIds := extractNodeIDs(result)
	subscriptionStatuses, err := h.env.Repos.SubscriptionsRel.IsSubscribedToMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	for _, actor := range result {
		actor.IsSubscribedByViewer = subscriptionStatuses[actor.ID].IsSubscribed
	}

	return &graphpb.ActorBatchReadResponse{
		Actors: nodesToGrpc(result),
	}, nil
}

// Edit update an actor
func (h *actorHandler) Edit(ctx context.Context, req *graphpb.ActorEditRequest) (*graphpb.ActorEditResponse, error) {
	log.Printf("Received Edit request: %v", req)

	node, err := h.env.Repos.Nodes.Read(req.Actor.Id)
	if err != nil {
		return nil, err
	}

	applyEditableActorFields(node, req.Actor)

	v := validators.NewActorValidator()
	if err := v.ValidateEdit(node); err != nil {
		return nil, err
	}

	result, err := h.env.Repos.Nodes.Edit(node)
	if err != nil {
		return nil, err
	}

	actor := nodeToGrpc(result)

	if actor.Type != "" {
		loader := features.NewFeatureLoader(h.env.Config.GetActorFeaturesPath())
		defaults, err := loader.Load(actor.Type)
		if err != nil {
			log.Printf("Failed to load default features for %s: %v", actor.Type, err)
		} else {
			_ = features.ApplyFeatures(actor, loader, defaults)
		}
	}

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType
	actor.IsSubscribedByViewer, err = h.env.Repos.SubscriptionsRel.IsSubscribedToOne(viewer.ID, actor.Id)
	if err != nil {
		return nil, err
	}

	// publish event
	go func() {
		entityName := strings.Split(actor.Type, ".")[3]
		srvName := strings.Split(actor.Service, "-")[0]
		routing_key := fmt.Sprintf(event.EventFormatNodeActorEdited, srvName, entityName)
		payload := &event.EventPayload{
			Name:           routing_key,
			SourceService:  actor.Service,
			SubjectID:      req.Viewer.Id,
			ObjectID:       actor.Id,
			TargetID:       actor.Id,
			ViewerID:       viewer.ID,
			ViewerUserType: viewer.UserType,
		}
		log.Println("Publishing event", routing_key)
		if err = h.env.Publisher.Publish(payload, routing_key); err != nil {
			log.Println(err)
		}
	}()

	return &graphpb.ActorEditResponse{
		Actor: actor,
	}, nil
}

// Add create a new actor
func (h *actorHandler) Add(ctx context.Context, req *graphpb.ActorAddRequest) (*graphpb.ActorAddResponse, error) {
	log.Printf("Received Add request: %v", req)

	node := newActorFromRequest(req.Actor)

	v := validators.NewActorValidator()
	if err := v.ValidateAdd(node); err != nil {
		return nil, err
	}

	result, err := h.env.Repos.Nodes.Add(node)
	if err != nil {
		return nil, err
	}

	actor := nodeToGrpc(result)

	if actor.Type != "" {
		loader := features.NewFeatureLoader(h.env.Config.GetActorFeaturesPath())
		defaults, err := loader.Load(actor.Type)
		if err != nil {
			log.Printf("Failed to load default features for %s: %v", actor.Type, err)
		} else {
			_ = features.ApplyFeatures(actor, loader, defaults)
		}
	}

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType

	// publish event
	go func() {
		entityName := strings.Split(actor.Type, ".")[3]
		srvName := strings.Split(actor.Service, "-")[0]
		routing_key := fmt.Sprintf(event.EventFormatNodeActorAdded, srvName, entityName)
		payload := &event.EventPayload{
			Name:           routing_key,
			SourceService:  actor.Service,
			SubjectID:      req.Viewer.Id,
			ObjectID:       actor.Id,
			TargetID:       actor.Id,
			ViewerID:       viewer.ID,
			ViewerUserType: viewer.UserType,
		}
		h.env.Publisher.Publish(payload, routing_key)
	}()

	return &graphpb.ActorAddResponse{
		Actor: actor,
	}, nil
}

// actorLifecycleRoutingKey builds the routing key for an actor transition.
//
// Separated out because it is the part that can be quietly wrong. A key with
// the wrong name does not fail — it publishes to a topic nothing is listening
// on, which looks exactly like a working publish from the emitting side and
// exactly like no event at all from the consuming side.
//
// The shape is node.<type>.<service>.<entity>.<action>.<version>, and both
// middle parts are short names: "node.actor.person-service.person.v1" gives
// the entity "person", and the service "person-service" gives "person". That
// is the convention Add already follows and the one person-service hardcoded
// before this took the job over.
//
// Reports false rather than guessing when the type is not shaped like an actor
// type. A malformed key is worse than none: no event is at least visible as an
// absence, while a wrong one is a message nobody receives and nobody misses.
func actorLifecycleRoutingKey(format, nodeType, service string) (string, bool) {
	parts := strings.Split(nodeType, ".")
	if len(parts) < 4 || parts[3] == "" || service == "" {
		return "", false
	}

	entityName := parts[3]
	srvName := strings.Split(service, "-")[0]
	if srvName == "" {
		return "", false
	}

	return fmt.Sprintf(format, srvName, entityName), true
}

// publishActorLifecycle emits a lifecycle transition for an actor.
//
// From HERE rather than from each calling service, because this is where the
// state actually changes. person-service published its own "deleted" event and
// group-service published nothing, so deleting a person notified consumers and
// deleting a group silently did not — an asymmetry nobody chose, and one that
// would have grown a third case with the next actor type.
//
// The routing key is derived from the node rather than passed in, for the same
// reason: a caller that hardcodes its own names is a caller that can disagree
// with the others.
//
// Best-effort and asynchronous. The transition has already committed, and a
// broker that is down is not a reason to tell the caller their group is still
// there.
func (h *actorHandler) publishActorLifecycle(format string, node *entities.AnahitaNode, viewerID int64, viewerType string) {
	routingKey, ok := actorLifecycleRoutingKey(format, node.Type, node.Service)
	if !ok {
		log.Printf("actor lifecycle: cannot derive a routing key from type %q "+
			"service %q, not publishing", node.Type, node.Service)
		return
	}

	payload := &event.EventPayload{
		Name:           routingKey,
		SourceService:  node.Service,
		SubjectID:      viewerID,
		ObjectID:       node.ID,
		TargetID:       node.ID,
		ViewerID:       viewerID,
		ViewerUserType: viewerType,
	}

	go func() {
		if err := h.env.Publisher.Publish(payload, routingKey); err != nil {
			log.Println("actor lifecycle: publish failed:", err)
		}
	}()
}

// Delete remove a medium
// Delete SOFT-deletes an actor: it is marked and disabled, and nothing is
// destroyed.
//
// This used to call PurgeByOwner, which hard-deleted the actor, everything it
// owned, every descendant of those by parent_id — other people's comments on
// the actor's posts included — and every edge touching any of them. There was
// no way back from a misclick and no record that it had happened.
//
// PurgeByOwner still exists and still does exactly that, but it now runs from
// the sweeper once the grace period has expired, not from a request.
func (h *actorHandler) Delete(ctx context.Context, req *graphpb.ActorDeleteRequest) (*graphpb.ActorDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req.Id)
	actor, err := h.env.Repos.Nodes.Read(req.Id)
	if err != nil {
		return nil, err
	}

	deleted, err := h.env.Repos.Nodes.SoftDeleteActor(actor.ID)
	if err != nil {
		return nil, err
	}

	// Already deleted. Not an error — a repeated request should not restart the
	// grace period, and the caller wanted the account gone, which it is. No
	// audit row either: nothing happened this time.
	if !deleted {
		log.Printf("actor %d was already marked deleted", actor.ID)
		return &graphpb.ActorDeleteResponse{}, nil
	}

	// Who deleted what, and when. Best-effort: the deletion has already
	// committed, and failing the request over the audit row would leave the
	// caller believing nothing happened when it did.
	//
	// req.ViewerId equals the actor for a self-service deletion and differs for
	// an administrator removing somebody else — which is the case this row
	// exists to answer for.
	var deletedBy int64
	if req.Viewer != nil {
		deletedBy = req.Viewer.Id
	}

	if err := h.env.Repos.NodeDeletionLogs.Add(
		actor.ID, actor.Alias, actor.Type, deletedBy,
		repositories.DeletionActionScheduled,
	); err != nil {
		log.Println("actor delete: audit write failed:", err)
	}

	// After the "already deleted" return above, so a repeated request does not
	// re-announce a deletion that happened days ago.
	var viewerType string
	if req.Viewer != nil {
		viewerType = req.Viewer.UserType
	}
	h.publishActorLifecycle(
		event.EventFormatNodeActorDeleted, actor, deletedBy, viewerType)

	return &graphpb.ActorDeleteResponse{}, nil
}

// Restore reverses a Delete, within the grace period.
//
// People restore themselves by signing in — the password is the proof, and the
// login page offers it. A group cannot sign in, so its administrators restore
// it from the group's own settings page, and this is what that calls.
//
// The permission check lives in group-service, which knows who administers the
// group; this service knows only nodes. Restoring is gated on the same
// permission as deleting, on the reasoning that undoing an action should never
// need more authority than doing it.
func (h *actorHandler) Restore(ctx context.Context, req *graphpb.ActorRestoreRequest) (*graphpb.ActorRestoreResponse, error) {
	log.Printf("Received Restore request: %v", req.Id)

	restored, err := h.env.Repos.Nodes.RestoreActor(req.Id)
	if err != nil {
		return nil, err
	}

	if !restored {
		// Already purged, or never deleted. Not an error — the caller needs to
		// tell it from success, which the response field carries.
		log.Printf("actor %d had nothing to restore", req.Id)
		return &graphpb.ActorRestoreResponse{Restored: false}, nil
	}

	var restoredBy int64
	if req.Viewer != nil {
		restoredBy = req.Viewer.Id
	}

	node, err := h.env.Repos.Nodes.Read(req.Id)
	if err != nil {
		// Restored, but we cannot describe what. Log rather than fail: the
		// group is back, which is what was asked for.
		log.Println("actor restore: audit read failed:", err)
		return &graphpb.ActorRestoreResponse{Restored: true}, nil
	}

	if err := h.env.Repos.NodeDeletionLogs.Add(
		node.ID, node.Alias, node.Type, restoredBy,
		repositories.DeletionActionRestored,
	); err != nil {
		log.Println("actor restore: audit write failed:", err)
	}

	// Only on a real restore — the "nothing to restore" branch returned above.
	// A consumer that revoked something on the way down needs this to put it
	// back; without it the only signal is a poll.
	var restoreViewerType string
	if req.Viewer != nil {
		restoreViewerType = req.Viewer.UserType
	}
	h.publishActorLifecycle(
		event.EventFormatNodeActorRestored, node, restoredBy, restoreViewerType)

	return &graphpb.ActorRestoreResponse{Restored: true}, nil
}

// Ping return a pong message
func (h *actorHandler) Ping(ctx context.Context, req *graphpb.ActorPingRequest) (*graphpb.ActorPingResponse, error) {
	log.Printf("Received Ping request: %v", req.Message)
	return &graphpb.ActorPingResponse{
		Message: "Pong",
	}, nil
}
