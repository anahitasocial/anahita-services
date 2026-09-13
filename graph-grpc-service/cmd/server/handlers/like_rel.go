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

type likeGrpcHandler struct {
	graphpb.LikeServiceServer
	env *Env
}

func NewLikeGrpcHandler(env *Env) *likeGrpcHandler {
	return &likeGrpcHandler{
		env: env,
	}
}

// Add Adds a like relationship between the source and the target
func (h *likeGrpcHandler) Add(ctx context.Context, req *graphpb.LikeAddRequest) (*graphpb.LikeAddResponse, error) {
	log.Printf("Received Add request: %v", req)
	var err error

	// verify req parameters
	if req.Source == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Source node is missing in the like request")
	}
	if req.Target == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Target node is missing in the like request")
	}
	if req.Viewer == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Viewer information is missing in the like request")
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
	viewer.PersonType = req.Viewer.PersonType

	edge := entities.NewAnahitaEdge()
	edge.Type = constants.EdgeBaseLikeEntityType
	edge.NodeAID = source.ID
	edge.NodeAType = source.Type
	edge.NodeBID = target.ID
	edge.NodeBType = target.Type
	edge.CreatedBy = viewer.ID

	edge, err = h.env.Repos.Edges.Add(edge)
	if err != nil {
		log.Printf("Error creating edge for like: %v", err)
		return nil, err
	}

	if edge.ID > 0 {
		if err := h.env.Repos.LikesRel.UpdateLikeCount(target.ID); err != nil {
			log.Printf("Error updating like count for target node: %v", err)
		}
	}

	return &graphpb.LikeAddResponse{
		Success: true,
	}, nil
}

// Delete Deletes a like relationship between the source and the target
func (h *likeGrpcHandler) Delete(ctx context.Context, req *graphpb.LikeDeleteRequest) (*graphpb.LikeDeleteResponse, error) {
	log.Printf("Received Delete request: %v", req)
	var err error

	if req.Source == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Source node is missing in the like request")
	}
	if req.Target == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Target node is missing in the like request")
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
		Types:   []string{constants.EdgeBaseLikeEntityType},
		NodeAID: source.ID,
		NodeBID: target.ID,
	}
	deleteCount, err := h.env.Repos.Edges.BatchDeleteByFilters(edgeReq)
	if err != nil {
		log.Printf("Error deleting edge for like: %v", err)
		return nil, err
	}

	if deleteCount > 0 {
		if err := h.env.Repos.LikesRel.UpdateLikeCount(target.ID); err != nil {
			log.Printf("Error updating like count for target node: %v", err)
		}
	}

	return &graphpb.LikeDeleteResponse{
		Success: true,
	}, nil
}

// IsLiked Checks if a like relationship exists between the source and the target
func (h *likeGrpcHandler) IsLiked(ctx context.Context, req *graphpb.LikeIsLikedRequest) (*graphpb.LikeIsLikedResponse, error) {
	log.Printf("Received IsSubscribedTo request: %v", req)
	var err error

	if req.SourceId <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "Source ID is missing in the like request")
	}
	if req.TargetId <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "Target ID is missing in the like request")
	}

	isLiked, err := h.env.Repos.LikesRel.IsLikedOne(req.SourceId, req.TargetId)
	if err != nil {
		log.Printf("Error checking IsLiked: %v", err)
		return nil, err
	}

	return &graphpb.LikeIsLikedResponse{
		IsLiked: isLiked,
	}, nil
}

// IsLikedMany Checks if like relationships exist between the source and multiple targets
func (h *likeGrpcHandler) IsLikedMany(ctx context.Context, req *graphpb.LikeIsLikedManyRequest) (*graphpb.LikeIsLikedManyResponse, error) {
	log.Printf("Received IsSubscribedToMany request: %v", req)
	var err error

	if req.SourceId <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "Source ID is missing in the like request")
	}
	if len(req.TargetIds) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "Target IDs are missing in the like request")
	}

	results, err := h.env.Repos.LikesRel.IsLikedMany(req.SourceId, req.TargetIds)
	if err != nil {
		log.Printf("Error checking IsLikedMany: %v", err)
		return nil, err
	}

	var likeStatuses []*graphpb.LikeStatus
	for _, status := range results {
		likeStatuses = append(likeStatuses, &graphpb.LikeStatus{
			Id:      status.TargetID,
			IsLiked: status.IsLiked,
		})
	}

	return &graphpb.LikeIsLikedManyResponse{
		Results: likeStatuses,
	}, nil
}
