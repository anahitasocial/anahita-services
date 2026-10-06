package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"
	"anahita-libs/proto/graphpb"
	"context"
	"fmt"
	"graph/data/validators"
	"graph/requests"
	"log"
	"strings"
)

type mediumHandler struct {
	graphpb.MediumServiceServer
	env *Env
}

func NewMediumHandler(env *Env) *mediumHandler {
	return &mediumHandler{env: env}
}

// Count return the total number of media
func (h *mediumHandler) Count(ctx context.Context, req *graphpb.MediumCountRequest) (*graphpb.MediumCountResponse, error) {
	log.Printf("Received Count request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().Count
	rReq.Viewer = viewer
	if req.OwnerId > 0 {
		rReq.OwnerIDs = []int64{req.OwnerId}
	}
	if len(req.Types) > 0 {
		rReq.Types = req.Types
	}
	rReq.Search = req.Search
	rReq.AccessCheck = req.AccessCheck
	rReq.TopLevelOnly = !req.IncludeReplies

	total, err := h.env.Repos.Nodes.Count(rReq)
	if err != nil {
		return nil, err
	}

	return &graphpb.MediumCountResponse{
		Total: total,
	}, nil
}

// Browse return a list of media
func (h *mediumHandler) Browse(ctx context.Context, req *graphpb.MediumBrowseRequest) (*graphpb.MediumBrowseResponse, error) {
	log.Printf("Received Browse request: %v", req)
	var err error

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().Browse
	if req.OwnerId > 0 {
		rReq.OwnerIDs = []int64{req.OwnerId}
	}
	if len(req.Types) > 0 {
		rReq.Types = req.Types
	}
	rReq.Search = req.Search
	rReq.Offset = req.Offset
	rReq.Limit = req.Limit
	rReq.Ordering = req.Ordering
	rReq.OrderingDir = req.Direction
	rReq.AccessCheck = req.AccessCheck
	rReq.TopLevelOnly = !req.IncludeReplies
	rReq.Viewer = viewer

	nodes, err := h.env.Repos.Nodes.Browse(rReq)
	if err != nil {
		return nil, err
	}

	targetIds := extractNodeIDs(nodes)
	subscriptionMap, err := h.env.Repos.SubscriptionsRel.IsSubscribedToMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	isLikedMap, err := h.env.Repos.LikesRel.IsLikedMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	isRepostedMap, err := h.env.Repos.Reposts.IsRepostedMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		node.IsSubscribedByViewer = subscriptionMap[node.ID].IsSubscribed
		node.IsLikedByViewer = isLikedMap[node.ID].IsLiked
		node.IsRepostedByViewer = isRepostedMap[node.ID]
	}

	return &graphpb.MediumBrowseResponse{
		Media: nodesToGrpc(nodes),
	}, nil
}

// Read return a single medium
func (h *mediumHandler) Read(ctx context.Context, req *graphpb.MediumReadRequest) (*graphpb.MediumReadResponse, error) {
	log.Printf("Received Read request: %v", req.Id)
	// Media only, for the same reason the actor read is scoped: /photos/ and
	// /topics/ both answered 200 with a hashtag for a hashtag's id.
	node, err := h.env.Repos.Nodes.ReadOfType(req.Id, constants.NodeMediumPrefixEntityType)
	if err != nil {
		return nil, err
	}

	node.IsSubscribedByViewer, err = h.env.Repos.SubscriptionsRel.IsSubscribedToOne(req.Viewer.Id, node.ID)
	if err != nil {
		return nil, err
	}

	node.IsLikedByViewer, err = h.env.Repos.LikesRel.IsLikedOne(req.Viewer.Id, node.ID)
	if err != nil {
		return nil, err
	}

	isRepostedMap, err := h.env.Repos.Reposts.IsRepostedMany(req.Viewer.Id, []int64{node.ID})
	if err != nil {
		return nil, err
	}
	node.IsRepostedByViewer = isRepostedMap[node.ID]

	// log.Println("Medium subscription status:", medium.IsSubscribedByViewer)
	// log.Println("Medium like status:", node.IsLikedByViewer)
	// log.Println("Medium repost status:", node.IsRepostedByViewer)

	return &graphpb.MediumReadResponse{
		Medium: nodeToGrpc(node),
	}, nil
}

// BatchRead return a list of media by their IDs
func (h *mediumHandler) BatchRead(ctx context.Context, req *graphpb.MediumBatchReadRequest) (*graphpb.MediumBatchReadResponse, error) {
	log.Printf("Received BatchRead request: %v", req.Ids)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	rReq := requests.NewNodeRequest().BatchRead
	rReq.IDs = req.Ids
	rReq.Ordering = req.Ordering
	rReq.OrderingDir = req.Direction

	nodes, err := h.env.Repos.Nodes.BatchRead(rReq)
	if err != nil {
		return nil, err
	}

	targetIds := extractNodeIDs(nodes)
	subscriptionMap, err := h.env.Repos.SubscriptionsRel.IsSubscribedToMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	isRepostedMap, err := h.env.Repos.Reposts.IsRepostedMany(req.Viewer.Id, targetIds)
	if err != nil {
		return nil, err
	}

	isLikedMap, err := h.env.Repos.LikesRel.IsLikedMany(viewer.ID, targetIds)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		node.IsSubscribedByViewer = subscriptionMap[node.ID].IsSubscribed
		node.IsRepostedByViewer = isRepostedMap[node.ID]
		node.IsLikedByViewer = isLikedMap[node.ID].IsLiked
	}

	return &graphpb.MediumBatchReadResponse{
		Media: nodesToGrpc(nodes),
	}, nil
}

// Edit update a medium
func (h *mediumHandler) Edit(ctx context.Context, req *graphpb.MediumEditRequest) (*graphpb.MediumEditResponse, error) {
	log.Printf("Received Edit request: %v", req)

	node, err := h.env.Repos.Nodes.Read(req.Medium.Id)
	if err != nil {
		return nil, err
	}

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	applyEditableMediumFields(node, req.Medium)

	v := validators.NewMediumValidator()
	if err := v.ValidateEdit(node); err != nil {
		return nil, err
	}

	node, err = h.env.Repos.Nodes.Edit(node)
	if err != nil {
		return nil, err
	}

	node.IsSubscribedByViewer, err = h.env.Repos.SubscriptionsRel.IsSubscribedToOne(viewer.ID, node.ID)
	if err != nil {
		return nil, err
	}

	node.IsLikedByViewer, err = h.env.Repos.LikesRel.IsLikedOne(viewer.ID, node.ID)
	if err != nil {
		return nil, err
	}

	isRepostedMap, err := h.env.Repos.Reposts.IsRepostedMany(viewer.ID, []int64{node.ID})
	if err != nil {
		return nil, err
	}
	node.IsRepostedByViewer = isRepostedMap[node.ID]

	// publish event
	go func() {
		entityName := strings.Split(node.Type, ".")[3]
		srvName := strings.Split(node.Service, "-")[0]
		routing_key := fmt.Sprintf(event.EventFormatNodeMediumEdited, srvName, entityName)
		payload := &event.EventPayload{
			Name:             routing_key,
			SourceService:    node.Service,
			SubjectID:        viewer.ID,
			SubjectType:      constants.NodeActorPersonEntityType,
			ObjectID:         node.ID,
			ObjectType:       node.Type,
			TargetID:         node.Owner.ID,
			TargetType:       node.Owner.Type,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		log.Println("Publishing event", routing_key)
		if err = h.env.Publisher.Publish(payload, routing_key); err != nil {
			log.Println(err)
		}
	}()

	return &graphpb.MediumEditResponse{
		Medium: nodeToGrpc(node),
	}, nil
}

// Add create a new medium
func (h *mediumHandler) Add(ctx context.Context, req *graphpb.MediumAddRequest) (*graphpb.MediumAddResponse, error) {
	log.Printf("Received Add request: %v", req)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	node := newMediumFromRequest(req.Medium)

	// A reply. Where it sits in its thread is worked out here from what it
	// answers, and never taken from the request: the root is what decides
	// who may see the reply, so it is not for a caller to name.
	if req.Medium.ParentId > 0 {
		parent, err := h.env.Repos.Nodes.Read(req.Medium.ParentId)
		if err != nil {
			return nil, err
		}
		if err := placeInThread(node, parent); err != nil {
			return nil, err
		}
	}

	v := validators.NewMediumValidator()
	if err := v.ValidateAdd(node); err != nil {
		return nil, err
	}

	node, err := h.env.Repos.Nodes.Add(node)
	if err != nil {
		return nil, err
	}

	if node.RootID > 0 {
		if err := h.env.Repos.Replies.Recount(node.RootID, node.ParentID); err != nil {
			log.Printf("counting the replies of %d failed: %v", node.RootID, err)
		}
	}

	// A note that quotes a post is one more quote of it.
	if node.QuoteID > 0 {
		if err := h.env.Repos.Quotes.Recount(node.QuoteID); err != nil {
			log.Printf("counting the quotes of %d failed: %v", node.QuoteID, err)
		}
	}

	// publish event
	go func() {
		entityName := strings.Split(node.Type, ".")[3]
		srvName := strings.Split(node.Service, "-")[0]
		routing_key := fmt.Sprintf(event.EventFormatNodeMediumAdded, srvName, entityName)
		payload := &event.EventPayload{
			Name:             routing_key,
			SourceService:    node.Service,
			SubjectID:        viewer.ID,
			SubjectType:      constants.NodeActorPersonEntityType,
			ObjectID:         node.ID,
			ObjectType:       node.Type,
			TargetID:         node.Owner.ID,
			TargetType:       node.Owner.Type,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		h.env.Publisher.Publish(payload, routing_key)
	}()

	return &graphpb.MediumAddResponse{
		Medium: nodeToGrpc(node),
	}, nil
}

// Delete remove a medium
func (h *mediumHandler) Delete(ctx context.Context, req *graphpb.MediumDeleteRequest) (*graphpb.MediumDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req.Id)

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.PersonType = req.Viewer.PersonType
	viewer.Username = req.Viewer.Username
	viewer.Email = req.Viewer.Email

	node, err := h.env.Repos.Nodes.Read(req.Id)
	if err != nil {
		return nil, err
	}

	// A reply that has been answered is emptied and kept, so what was said
	// under it still hangs from something. Anything else goes, with
	// everything under it: a post takes its whole thread.
	answered := false
	if node.RootID > 0 {
		if answered, err = h.env.Repos.Replies.HasReplies(node.ID); err != nil {
			return nil, err
		}
	}

	if answered {
		err = h.env.Repos.Replies.Tombstone(node.ID, viewer.ID)
	} else {
		_, err = h.env.Repos.Nodes.DeleteSubtree(node.ID)
	}
	if err != nil {
		return nil, err
	}

	if node.RootID > 0 {
		if err := h.env.Repos.Replies.Recount(node.RootID, node.ParentID); err != nil {
			log.Printf("counting the replies of %d failed: %v", node.RootID, err)
		}
	}

	// One quote fewer of what it quoted.
	if node.QuoteID > 0 {
		if err := h.env.Repos.Quotes.Recount(node.QuoteID); err != nil {
			log.Printf("counting the quotes of %d failed: %v", node.QuoteID, err)
		}
	}

	// The list of a photo post's images goes with the post. The stored images
	// themselves are removed by the service that asked for this.
	if err := h.env.Repos.PhotoFiles.DeleteForNodes([]int64{node.ID}); err != nil {
		log.Printf("deleting the image list of %d failed: %v", node.ID, err)
	}

	// publish event
	go func() {
		entityName := strings.Split(node.Type, ".")[3]
		srvName := strings.Split(node.Service, "-")[0]
		routing_key := fmt.Sprintf(event.EventFormatNodeMediumDeleted, srvName, entityName)
		payload := &event.EventPayload{
			Name:             routing_key,
			SourceService:    node.Service,
			SubjectID:        viewer.ID,
			SubjectType:      constants.NodeActorPersonEntityType,
			ObjectID:         node.ID,
			ObjectType:       node.Type,
			TargetID:         node.Owner.ID,
			TargetType:       node.Owner.Type,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		h.env.Publisher.Publish(payload, routing_key)
	}()

	return &graphpb.MediumDeleteResponse{}, nil
}
