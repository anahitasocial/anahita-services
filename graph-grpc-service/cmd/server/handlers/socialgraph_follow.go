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

type socialgraphFollowGrpcHandler struct {
	graphpb.SocialgraphFollowServiceServer
	env *Env
}

func NewSocialgraphFollowGrpcHandler(env *Env) *socialgraphFollowGrpcHandler {
	return &socialgraphFollowGrpcHandler{env: env}
}

// Edit
func (h *socialgraphFollowGrpcHandler) Edit(ctx context.Context, req *graphpb.SocialgraphFollowEditRequest) (*graphpb.SocialgraphFollowEditResponse, error) {
	log.Printf("Received Edit request: %v", req)

	if req.Source.Id == 0 || req.Target.Id == 0 {
		return nil, errors.New("source and target IDs are required")
	}

	if req.Source.Id == req.Target.Id {
		return nil, errors.New("source and target cannot be the same")
	}

	if req.Viewer.Id == 0 {
		return nil, errors.New("viewer ID is required")
	}

	if req.AcceptFollowRequest {
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

		rReq := requests.NewEdgeRequest().ReadByFilters
		rReq.Types = []string{constants.EdgeFollowRequestEntityType}
		rReq.NodeAID = source.ID
		rReq.NodeAType = source.Type
		rReq.NodeBID = target.ID
		rReq.NodeBType = target.Type
		edge, err := h.env.Repos.Edges.ReadByFilters(rReq)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		edge.Type = constants.EdgeFollowEntityType
		edge.ModifiedBy = req.Viewer.Id
		_, err = h.env.Repos.Edges.Edit(edge)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		// publish a follower event
		go func() {
			srvName := "socialgraph"
			routingKey := fmt.Sprintf(event.EventFormatEdgeFollowAdded, srvName)
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
	}

	return &graphpb.SocialgraphFollowEditResponse{}, nil
}

// Add adds a follower
func (h *socialgraphFollowGrpcHandler) Add(ctx context.Context, req *graphpb.SocialgraphFollowAddRequest) (*graphpb.SocialgraphFollowAddResponse, error) {
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

	// create the follow edge
	edgeType := constants.EdgeFollowEntityType
	if req.IsFollowRequest {
		edgeType = constants.EdgeFollowRequestEntityType
	}

	_, err = h.env.Repos.Edges.Add(&entities.AnahitaEdge{
		Type:       edgeType,
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

	if err = h.env.Repos.SocialGraphCounts.UpdateLeaderCount(source.ID); err != nil {
		log.Println(err)
		return nil, err
	}

	if err = h.env.Repos.SocialGraphCounts.UpdateFollowerCount(target.ID); err != nil {
		log.Println(err)
		return nil, err
	}

	// publish a follower event
	go func() {
		if !req.IsFollowRequest {
			srvName := "socialgraph"
			routingKey := fmt.Sprintf(event.EventFormatEdgeFollowAdded, srvName)
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
		}
	}()

	return &graphpb.SocialgraphFollowAddResponse{}, nil
}

// Delete removes a follower
func (h *socialgraphFollowGrpcHandler) Delete(ctx context.Context, req *graphpb.SocialgraphFollowDeleteRequest) (*graphpb.SocialgraphFollowDeleteResponse, error) {
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

	if err = h.env.Repos.SocialGraphCounts.UpdateLeaderCount(source.ID); err != nil {
		log.Println(err)
		return nil, err
	}

	if err = h.env.Repos.SocialGraphCounts.UpdateFollowerCount(target.ID); err != nil {
		log.Println(err)
		return nil, err
	}

	// publish a follower event
	go func() {
		srvName := "socialgraph"
		routingKey := fmt.Sprintf(event.EventFormatEdgeFollowDeleted, srvName)
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

	return &graphpb.SocialgraphFollowDeleteResponse{}, nil
}
