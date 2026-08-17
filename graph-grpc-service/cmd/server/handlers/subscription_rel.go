package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"graph/requests"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type subscriptionGrpcHandler struct {
	graphpb.SubscriptionServiceServer
	env *Env
}

func NewSubscriptionGrpcHandler(env *Env) *subscriptionGrpcHandler {
	return &subscriptionGrpcHandler{
		env: env,
	}
}

// BrowseSources
func (h *subscriptionGrpcHandler) BrowseSources(ctx context.Context, req *graphpb.SubscriptionBrowseSourcesRequest) (*graphpb.SubscriptionBrowseSourcesResponse, error) {
	log.Printf("Received BrowseSources request: %v", req)
	var err error

	if req.Target == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Target node is missing in the subscription request")
	}

	target, err := h.env.Repos.Nodes.Read(req.Target.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to read target node: %v", err)
	}

	edgeReq := &requests.EdgeBrowseRequest{
		NodeBID:   target.ID,
		NodeBType: target.Type,
		Types:     []string{constants.EdgeBaseSubscriptionEntityType},
	}

	edges, err := h.env.Repos.Edges.Browse(edgeReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to browse edges: %v", err)
	}

	var respSources []*graphpb.Source
	for _, edge := range edges {
		respSources = append(respSources, &graphpb.Source{
			Id:   edge.NodeAID,
			Type: edge.NodeAType,
		})
	}

	return &graphpb.SubscriptionBrowseSourcesResponse{
		Sources: respSources,
	}, nil
}

// Add Adds a subscription relationship between the source and the target
func (h *subscriptionGrpcHandler) Add(ctx context.Context, req *graphpb.SubscriptionAddRequest) (*graphpb.SubscriptionAddResponse, error) {
	log.Printf("Received Add request: %v", req)
	var err error

	// verify req parameters
	if req.Source == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Source node is missing in the subscription request")
	}
	if req.Target == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Target node is missing in the subscription request")
	}
	if req.Viewer == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Viewer information is missing in the subscription request")
	}

	source, err := h.env.Repos.Nodes.Read(req.Source.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to read source node: %v", err)
	}

	target, err := h.env.Repos.Nodes.Read(req.Target.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to read target node: %v", err)
	}

	viewer := entities.NewViewer()
	viewer.ID = req.Viewer.Id
	viewer.UserType = req.Viewer.UserType

	edge := entities.NewAnahitaEdge()
	edge.Type = constants.EdgeBaseSubscriptionEntityType
	edge.NodeAID = source.ID
	edge.NodeAType = source.Type
	edge.NodeBID = target.ID
	edge.NodeBType = target.Type
	edge.CreatedBy = viewer.ID

	edge, err = h.env.Repos.Edges.Add(edge)
	if err != nil {
		log.Printf("Error creating edge for subscription: %v", err)
		return nil, err
	}

	if edge.ID > 0 {
		if err := h.env.Repos.SubscriptionsRel.UpdateSubscriberCount(target.ID); err != nil {
			log.Printf("Error updating subscriber count for target node: %v", err)
		}
	}

	// Implement the logic to add a subscription
	return &graphpb.SubscriptionAddResponse{
		Success: true,
	}, nil
}

// Delete
func (h *subscriptionGrpcHandler) Delete(ctx context.Context, req *graphpb.SubscriptionDeleteRequest) (*graphpb.SubscriptionDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req)
	var err error

	if req.Source == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Source node is missing in the subscription request")
	}
	if req.Target == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Target node is missing in the subscription request")
	}

	source, err := h.env.Repos.Nodes.Read(req.Source.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to read source node: %v", err)
	}

	target, err := h.env.Repos.Nodes.Read(req.Target.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to read target node: %v", err)
	}

	edgeReq := &requests.EdgeBatchDeleteByFiltersRequest{
		Types:   []string{constants.EdgeBaseSubscriptionEntityType},
		NodeAID: source.ID,
		NodeBID: target.ID,
	}
	deleteCount, err := h.env.Repos.Edges.BatchDeleteByFilters(edgeReq)
	if err != nil {
		log.Printf("Error deleting edge for subscription: %v", err)
		return nil, err
	}

	if deleteCount > 0 {
		if err := h.env.Repos.SubscriptionsRel.UpdateSubscriberCount(target.ID); err != nil {
			log.Printf("Error updating subscriber count for target node: %v", err)
		}
	}

	// Implement the logic to delete a subscription
	return &graphpb.SubscriptionDeleteResponse{}, nil
}

// IsSubscribedTo
func (h *subscriptionGrpcHandler) IsSubscribedTo(ctx context.Context, req *graphpb.SubscriptionIsSubscribedToRequest) (*graphpb.SubscriptionIsSubscribedToResponse, error) {
	log.Printf("Received IsSubscribedTo request: %v", req)
	var err error

	if req.SourceId <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "Source ID is missing in the subscription request")
	}
	if req.TargetId <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "Target ID is missing in the subscription request")
	}

	isSubscribed, err := h.env.Repos.SubscriptionsRel.IsSubscribedToOne(req.SourceId, req.TargetId)
	if err != nil {
		log.Printf("Error checking IsSubscribedTo: %v", err)
		return nil, err
	}

	return &graphpb.SubscriptionIsSubscribedToResponse{
		IsSubscribed: isSubscribed,
	}, nil
}

// IsSubscribedToMany
func (h *subscriptionGrpcHandler) IsSubscribedToMany(ctx context.Context, req *graphpb.SubscriptionIsSubscribedToManyRequest) (*graphpb.SubscriptionIsSubscribedToManyResponse, error) {
	log.Printf("Received IsSubscribedToMany request: %v", req)
	var err error

	if req.SourceId <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "Source ID is missing in the subscription request")
	}
	if len(req.TargetIds) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "Target IDs are missing in the subscription request")
	}

	results, err := h.env.Repos.SubscriptionsRel.IsSubscribedToMany(req.SourceId, req.TargetIds)
	if err != nil {
		log.Printf("Error checking IsSubscribedToMany: %v", err)
		return nil, err
	}

	// loop through and convert the values to SubscriptionStatus
	var subscriptionStatuses []*graphpb.SubscriptionStatus
	for _, result := range results {
		subscriptionStatuses = append(subscriptionStatuses, &graphpb.SubscriptionStatus{
			Id:           result.TargetID,
			IsSubscribed: result.IsSubscribed,
		})

		// log.Println("Result:", result.TargetID, result.IsSubscribed)
	}

	return &graphpb.SubscriptionIsSubscribedToManyResponse{
		Results: subscriptionStatuses,
	}, nil
}
