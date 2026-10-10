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

// Edit accepts a request to follow: the request goes and a follow takes its
// place.
//
// It used to set the type on the edge it had read and save it, and saving
// an edge does not write its type. So nothing changed: the request stayed a
// request, and a notification went out saying the follow had been made.
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

	if !req.AcceptFollowRequest {
		return &graphpb.SocialgraphFollowEditResponse{}, nil
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

	request, err := h.readEdge(constants.EdgeFollowRequestEntityType, source, target)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	if request == nil {
		return nil, errors.New("there is no request to follow to accept")
	}

	if err = h.addFollow(source, target, req.Viewer.Id); err != nil {
		log.Println(err)
		return nil, err
	}

	if err = h.deleteEdges(constants.EdgeFollowRequestEntityType, source, target); err != nil {
		log.Println(err)
		return nil, err
	}

	if err = h.updateCounts(source, target); err != nil {
		log.Println(err)
		return nil, err
	}

	h.publish(event.EventFormatEdgeFollowRequestDeleted, source, target, req.Viewer)
	h.publish(event.EventFormatEdgeFollowRequestAccepted, source, target, req.Viewer)
	h.publish(event.EventFormatEdgeFollowAdded, source, target, req.Viewer)

	return &graphpb.SocialgraphFollowEditResponse{}, nil
}

// Add makes the source a follower of the target, or records that it asked
// to be.
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

	if req.IsFollowRequest {
		// Asking twice is asking once: the edge is unique, and a second
		// insert would be refused by the database as an error.
		existing, err := h.readEdge(constants.EdgeFollowRequestEntityType, source, target)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		if existing != nil {
			return &graphpb.SocialgraphFollowAddResponse{}, nil
		}

		if _, err = h.env.Repos.Edges.Add(&entities.AnahitaEdge{
			Type:       constants.EdgeFollowRequestEntityType,
			NodeAID:    source.ID,
			NodeAType:  source.Type,
			NodeBID:    target.ID,
			NodeBType:  target.Type,
			CreatedBy:  req.Viewer.Id,
			ModifiedBy: req.Viewer.Id,
		}); err != nil {
			log.Println(err)
			return nil, err
		}

		h.publish(event.EventFormatEdgeFollowRequestAdded, source, target, req.Viewer)

		return &graphpb.SocialgraphFollowAddResponse{}, nil
	}

	if err = h.addFollow(source, target, req.Viewer.Id); err != nil {
		log.Println(err)
		return nil, err
	}

	// A request that was waiting is answered by the follow itself.
	waiting, err := h.readEdge(constants.EdgeFollowRequestEntityType, source, target)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	if waiting != nil {
		if err = h.deleteEdges(constants.EdgeFollowRequestEntityType, source, target); err != nil {
			log.Println(err)
			return nil, err
		}
		h.publish(event.EventFormatEdgeFollowRequestDeleted, source, target, req.Viewer)
	}

	if err = h.updateCounts(source, target); err != nil {
		log.Println(err)
		return nil, err
	}

	h.publish(event.EventFormatEdgeFollowAdded, source, target, req.Viewer)

	return &graphpb.SocialgraphFollowAddResponse{}, nil
}

// Delete ends a follow, or removes a request to follow.
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

	// A request ignored or taken back. Nobody followed anybody, so the
	// counts stand.
	if req.IsFollowRequest {
		if err = h.deleteEdges(constants.EdgeFollowRequestEntityType, source, target); err != nil {
			log.Println(err)
			return nil, err
		}

		h.publish(event.EventFormatEdgeFollowRequestDeleted, source, target, req.Viewer)

		return &graphpb.SocialgraphFollowDeleteResponse{}, nil
	}

	if err = h.deleteEdges(constants.EdgeFollowEntityType, source, target); err != nil {
		log.Println(err)
		return nil, err
	}

	if err = h.updateCounts(source, target); err != nil {
		log.Println(err)
		return nil, err
	}

	h.publish(event.EventFormatEdgeFollowDeleted, source, target, req.Viewer)

	return &graphpb.SocialgraphFollowDeleteResponse{}, nil
}

// readEdge returns the edge of this type from the source to the target, or
// nil when there is none.
func (h *socialgraphFollowGrpcHandler) readEdge(edgeType string, source *entities.AnahitaNode, target *entities.AnahitaNode) (*entities.AnahitaEdge, error) {
	rReq := requests.NewEdgeRequest().ReadByFilters
	rReq.Types = []string{edgeType}
	rReq.NodeAID = source.ID
	rReq.NodeAType = source.Type
	rReq.NodeBID = target.ID
	rReq.NodeBType = target.Type

	return h.env.Repos.Edges.ReadByFilters(rReq)
}

func (h *socialgraphFollowGrpcHandler) deleteEdges(edgeType string, source *entities.AnahitaNode, target *entities.AnahitaNode) error {
	edgeReq := requests.NewEdgeRequest().BatchDeleteByFilters
	edgeReq.Types = []string{edgeType}
	edgeReq.NodeAID = source.ID
	edgeReq.NodeAType = source.Type
	edgeReq.NodeBID = target.ID
	edgeReq.NodeBType = target.Type

	_, err := h.env.Repos.Edges.BatchDeleteByFilters(edgeReq)

	return err
}

func (h *socialgraphFollowGrpcHandler) addFollow(source *entities.AnahitaNode, target *entities.AnahitaNode, viewerID int64) error {
	_, err := h.env.Repos.Edges.Add(&entities.AnahitaEdge{
		Type:       constants.EdgeFollowEntityType,
		NodeAID:    source.ID,
		NodeAType:  source.Type,
		NodeBID:    target.ID,
		NodeBType:  target.Type,
		CreatedBy:  viewerID,
		ModifiedBy: viewerID,
	})

	return err
}

// updateCounts recounts who the source follows and who follows the target.
// Both count follows only, so a request waiting is in neither.
func (h *socialgraphFollowGrpcHandler) updateCounts(source *entities.AnahitaNode, target *entities.AnahitaNode) error {
	if err := h.env.Repos.SocialGraphCounts.UpdateLeaderCount(source.ID); err != nil {
		return err
	}

	return h.env.Repos.SocialGraphCounts.UpdateFollowerCount(target.ID)
}

// publish tells the other services what happened between the source and
// the target. The subject is whoever did it, which is not always the source:
// an administrator accepts a request, or removes a follower.
func (h *socialgraphFollowGrpcHandler) publish(format string, source *entities.AnahitaNode, target *entities.AnahitaNode, viewer *graphpb.Viewer) {
	routingKey := fmt.Sprintf(format, "socialgraph")
	payload := &event.EventPayload{
		Name:             routingKey,
		SourceService:    "graph-grpc-service",
		SubjectID:        viewer.GetId(),
		ObjectID:         source.ID,
		TargetID:         target.ID,
		ViewerID:         viewer.GetId(),
		ViewerPersonType: viewer.GetPersonType(),
	}

	go func() {
		log.Println("Publishing event", routingKey)
		h.env.Publisher.Publish(payload, routingKey)
	}()
}
