package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"
	"anahita-libs/proto/graphpb"
	"context"
	"errors"
	"fmt"
	"graph/requests"
	"log"
)

type socialgraphBlockGrpcHandler struct {
	graphpb.SocialgraphBlockServiceServer
	env *Env
}

func NewSocialgraphBlockGrpcHandler(env *Env) *socialgraphBlockGrpcHandler {
	return &socialgraphBlockGrpcHandler{env: env}
}

// Add adds a block
func (h *socialgraphBlockGrpcHandler) Add(ctx context.Context, req *graphpb.SocialgraphBlockAddRequest) (*graphpb.SocialgraphBlockAddResponse, error) {
	log.Printf("Received Add request: %v", req)

	if req.Source.Id == 0 || req.Target.Id == 0 {
		return nil, errors.New("source and target IDs are required")
	}

	if req.Source.Id == req.Target.Id {
		return nil, errors.New("source and target cannot be the same")
	}

	if req.Viewer.Id == 0 {
		return nil, errors.New("viewer ID is required")
	}

	source, err := h.env.Repos.Nodes.Read(req.Source.Id)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	target, err := h.env.Repos.Nodes.Read(req.Target.Id)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	sourceVsTargetRel, err := h.env.Repos.SocialgraphRel.GetOne(source.ID, target.ID)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if sourceVsTargetRel.IsFollowing {
		edgeReq := requests.NewEdgeRequest().BatchDeleteByFilters
		edgeReq.Types = []string{constants.EdgeFollowEntityType}
		edgeReq.NodeAID = source.ID
		edgeReq.NodeAType = source.Type
		edgeReq.NodeBID = target.ID
		edgeReq.NodeBType = target.Type
		_, err := h.env.Repos.Edges.BatchDeleteByFilters(edgeReq)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		// Update counts
		if err := h.env.Repos.SocialGraphCounts.UpdateLeaderCount(source.ID); err != nil {
			log.Println(err)
			return nil, err
		}
		if err := h.env.Repos.SocialGraphCounts.UpdateFollowerCount(target.ID); err != nil {
			log.Println(err)
			return nil, err
		}
	}

	targetVsSourceRel, err := h.env.Repos.SocialgraphRel.GetOne(target.ID, source.ID)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if targetVsSourceRel.IsFollowing {
		edgeReq := requests.NewEdgeRequest().BatchDeleteByFilters
		edgeReq.Types = []string{constants.EdgeFollowEntityType}
		edgeReq.NodeAID = target.ID
		edgeReq.NodeAType = target.Type
		edgeReq.NodeBID = source.ID
		edgeReq.NodeBType = source.Type
		_, err := h.env.Repos.Edges.BatchDeleteByFilters(edgeReq)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		// Update counts
		if err := h.env.Repos.SocialGraphCounts.UpdateLeaderCount(target.ID); err != nil {
			log.Println(err)
			return nil, err
		}
		if err := h.env.Repos.SocialGraphCounts.UpdateFollowerCount(source.ID); err != nil {
			log.Println(err)
			return nil, err
		}
	}

	// Neither is told of the other any more (roadmap row 96, part 1): the
	// subscriptions go with the follows, both ways, the one that came with
	// administering included.
	for _, pair := range [][2]*entities.AnahitaNode{{source, target}, {target, source}} {
		edgeReq := requests.NewEdgeRequest().BatchDeleteByFilters
		edgeReq.Types = []string{constants.EdgeBaseSubscriptionEntityType}
		edgeReq.NodeAID = pair[0].ID
		edgeReq.NodeAType = pair[0].Type
		edgeReq.NodeBID = pair[1].ID
		edgeReq.NodeBType = pair[1].Type
		if _, err := h.env.Repos.Edges.BatchDeleteByFilters(edgeReq); err != nil {
			log.Println(err)
			return nil, err
		}
		if err := h.env.Repos.SubscriptionsRel.UpdateSubscriberCount(pair[1].ID); err != nil {
			log.Println(err)
		}
	}

	_, err = h.env.Repos.Edges.Add(&entities.AnahitaEdge{
		Type:       constants.EdgeBlockEntityType,
		NodeAID:    source.ID,
		NodeAType:  source.Type,
		NodeBID:    target.ID,
		NodeBType:  target.Type,
		CreatedBy:  req.Viewer.Id,
		ModifiedBy: req.Viewer.Id,
	})
	if err != nil {
		log.Println(err)
		return nil, err
	}

	h.recountForBlock(source.ID, target.ID)

	// publish a block event
	go func() {
		srvName := "socialgraph"
		routingKey := fmt.Sprintf(event.EventFormatEdgeBlockAdded, srvName)
		payload := &event.EventPayload{
			Name:             routingKey,
			SourceService:    "graph-grpc-service",
			SubjectID:        req.Viewer.Id,
			ObjectID:         source.ID,
			TargetID:         target.ID,
			ViewerID:         req.Viewer.Id,
			ViewerPersonType: req.Viewer.PersonType,
		}
		log.Println("Publishing event", routingKey)
		h.env.Publisher.Publish(payload, routingKey)
	}()

	return &graphpb.SocialgraphBlockAddResponse{}, nil
}

// Delete removes a block
func (h *socialgraphBlockGrpcHandler) Delete(ctx context.Context, req *graphpb.SocialgraphBlockDeleteRequest) (*graphpb.SocialgraphBlockDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req)

	if req.Source.Id == 0 || req.Target.Id == 0 {
		return nil, errors.New("source and target IDs are required")
	}

	source, err := h.env.Repos.Nodes.Read(req.Source.Id)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	target, err := h.env.Repos.Nodes.Read(req.Target.Id)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	_, err = h.env.Repos.Edges.BatchDeleteByFilters(unblockFilter(source, target))
	if err != nil {
		log.Println(err)
		return nil, err
	}

	h.recountForBlock(source.ID, target.ID)

	// publish a block event
	go func() {
		srvName := "socialgraph"
		routingKey := fmt.Sprintf(event.EventFormatEdgeBlockDeleted, srvName)
		payload := &event.EventPayload{
			Name:             routingKey,
			SourceService:    "graph-grpc-service",
			SubjectID:        req.Viewer.Id,
			ObjectID:         source.ID,
			TargetID:         target.ID,
			ViewerID:         req.Viewer.Id,
			ViewerPersonType: req.Viewer.PersonType,
		}
		log.Println("Publishing event", routingKey)
		h.env.Publisher.Publish(payload, routingKey)
	}()

	return &graphpb.SocialgraphBlockDeleteResponse{}, nil
}

// unblockFilter names the one edge that unblocking removes: the block, from
// whoever blocked to whoever was blocked.
//
// It named the follow edge between them until October 2026. Blocking had
// already removed any follow, so there was nothing of that kind to delete:
// unblocking answered that it was done and the block stayed.
func unblockFilter(source, target *entities.AnahitaNode) *requests.EdgeBatchDeleteByFiltersRequest {
	filter := requests.NewEdgeRequest().BatchDeleteByFilters
	filter.Types = []string{constants.EdgeBlockEntityType}
	filter.NodeAID = source.ID
	filter.NodeAType = source.Type
	filter.NodeBID = target.ID
	filter.NodeBType = target.Type
	return filter
}

// recountForBlock counts again what a block, made or lifted, changes for
// other readers: replies and likes the blocked person left on what is the
// blocker's are out of sight while the block stands. A count that fails is
// logged; it is put right the next time anything changes it.
func (h *socialgraphBlockGrpcHandler) recountForBlock(blockerID, blockedID int64) {
	roots, liked, err := h.env.Repos.Replies.TouchedByBlock(blockerID, blockedID)
	if err != nil {
		log.Println("recounting after a block:", err)
		return
	}
	for _, root := range roots {
		if err := h.env.Repos.Replies.Recount(root, 0); err != nil {
			log.Println("recounting after a block:", err)
		}
	}
	for _, post := range liked {
		if err := h.env.Repos.LikesRel.UpdateLikeCount(post); err != nil {
			log.Println("recounting after a block:", err)
		}
	}
}
