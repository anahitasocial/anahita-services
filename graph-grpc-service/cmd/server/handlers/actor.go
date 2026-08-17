package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/event"
	"anahita-libs/proto/graphpb"
	"context"
	"encoding/json"
	"fmt"
	"graph/requests"
	"log"
	"strings"

	features "graph/data/actor_features"
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

	node, err := h.env.Repos.Nodes.Read(req.Id)
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

	node.Enabled = req.Actor.Enabled
	node.Access = req.Actor.Access
	node.Name = req.Actor.Name
	node.Body = req.Actor.Body
	node.Alias = req.Actor.Alias
	node.Verified = req.Actor.Verified
	node.Filename = req.Actor.Filename
	node.CoverFilename = req.Actor.CoverFilename
	node.AllowFollowRequest = req.Actor.AllowFollowRequest
	node.FeaturesRaw = json.RawMessage(req.Actor.FeaturesRaw)
	node.ModifiedBy = req.Actor.ModifiedBy

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

	node := entities.NewAnahitaNode()
	node.Type = req.Actor.Type
	node.Service = req.Actor.Service
	node.Enabled = req.Actor.Enabled
	node.Access = req.Actor.Access
	node.Name = req.Actor.Name
	node.Alias = req.Actor.Alias
	node.Body = req.Actor.Body
	node.Verified = req.Actor.Verified
	node.AllowFollowRequest = req.Actor.AllowFollowRequest
	node.FeaturesRaw = json.RawMessage(req.Actor.FeaturesRaw)
	node.Filename = req.Actor.Filename
	node.CoverFilename = req.Actor.CoverFilename
	node.Meta = json.RawMessage(req.Actor.Meta)
	node.CreatedBy = req.Actor.CreatedBy
	node.ModifiedBy = req.Actor.ModifiedBy

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

// Delete remove a medium
func (h *actorHandler) Delete(ctx context.Context, req *graphpb.ActorDeleteRequest) (*graphpb.ActorDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req.Id)
	actor, err := h.env.Repos.Nodes.Read(req.Id)
	if err != nil {
		return nil, err
	}

	_, err = h.env.Repos.Nodes.PurgeByOwner(actor.ID)
	if err != nil {
		return nil, err
	}

	return &graphpb.ActorDeleteResponse{}, nil
}

// Ping return a pong message
func (h *actorHandler) Ping(ctx context.Context, req *graphpb.ActorPingRequest) (*graphpb.ActorPingResponse, error) {
	log.Printf("Received Ping request: %v", req.Message)
	return &graphpb.ActorPingResponse{
		Message: "Pong",
	}, nil
}
