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

	// publish a block event
	go func() {
		srvName := "socialgraph"
		routingKey := fmt.Sprintf(event.EventFormatEdgeBlockAdded, srvName)
		payload := &event.EventPayload{
			Name:           routingKey,
			SourceService:  "graph-grpc-service",
			SubjectID:      req.Viewer.Id,
			ObjectID:       source.ID,
			TargetID:       target.ID,
			ViewerID:       req.Viewer.Id,
			ViewerUserType: req.Viewer.UserType,
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

	edgeReq := requests.NewEdgeRequest().BatchDeleteByFilters
	edgeReq.Types = []string{constants.EdgeFollowEntityType}
	edgeReq.NodeAID = source.ID
	edgeReq.NodeAType = source.Type
	edgeReq.NodeBID = target.ID
	edgeReq.NodeBType = target.Type

	_, err = h.env.Repos.Edges.BatchDeleteByFilters(edgeReq)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// publish a block event
	go func() {
		srvName := "socialgraph"
		routingKey := fmt.Sprintf(event.EventFormatEdgeBlockDeleted, srvName)
		payload := &event.EventPayload{
			Name:           routingKey,
			SourceService:  "graph-grpc-service",
			SubjectID:      req.Viewer.Id,
			ObjectID:       source.ID,
			TargetID:       target.ID,
			ViewerID:       req.Viewer.Id,
			ViewerUserType: req.Viewer.UserType,
		}
		log.Println("Publishing event", routingKey)
		h.env.Publisher.Publish(payload, routingKey)
	}()

	return &graphpb.SocialgraphBlockDeleteResponse{}, nil
}
